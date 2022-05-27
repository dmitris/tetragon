// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Tetragon

package eventchecker

import (
	"github.com/cilium/tetragon/cmd/protoc-gen-go-tetragon/common"
	"google.golang.org/protobuf/compiler/protogen"
)

// generateMultiEventCheckers generates boilerplate for MultiEventChecker types
func generateMultiEventCheckers(g *protogen.GeneratedFile, f *protogen.File) error {
	if err := generateMultiEventCheckerInterface(g, f); err != nil {
		return err
	}

	if err := generateOrderedEventChecker(g, f); err != nil {
		return err
	}

	if err := generateUnorderedEventChecker(g, f); err != nil {
		return err
	}

	if err := generateFnEventChecker(g, f); err != nil {
		return err
	}

	return nil
}

// generateOrderedEventChecker generates boilerplate for the ordered MultiEventChecker
func generateOrderedEventChecker(g *protogen.GeneratedFile, f *protogen.File) error {
	tetragonGER := common.TetragonApiIdent(g, "GetEventsResponse")
	logger := common.GoIdent(g, "github.com/sirupsen/logrus", "Logger")

	g.P(`// OrderedEventChecker checks a series of events in order
    type OrderedEventChecker struct {
        checks []EventChecker
        idx    int
    }`)

	g.P(`// NewOrderedEventChecker creates a new OrderedEventChecker
    func NewOrderedEventChecker(checks ...EventChecker) *OrderedEventChecker {
        return &OrderedEventChecker{
            checks: checks,
            idx:    0,
        }
    }`)

	g.P(`// NextEventCheck implements the MultiEventChecker interface
    func (checker *OrderedEventChecker) NextEventCheck(event Event, logger *` + logger + `) (bool, error) {
        if checker.idx >= len(checker.checks) {
            return true, nil
        }

        err := checker.checks[checker.idx].CheckEvent(event)
        if err != nil {
            return false, err
        }

        checker.idx++
        if checker.idx == len(checker.checks) {
            if logger != nil {
                logger.Infof("OrderedEventChecker: all %d checks matched", len(checker.checks))
            }
            return true, nil
        }

        if logger != nil {
            logger.Infof("OrderedEventChecker: %d/%d matched", checker.idx, len(checker.checks))
        }
        return false, nil
    }`)

	g.P(`// NextResponseCheck implements the MultiEventChecker interface
    func (checker *OrderedEventChecker) NextResponseCheck(response *` + tetragonGER + `, logger  *` + logger + `) (bool, error) {
        event, err := EventFromResponse(response)
        if err != nil {
            return false, err
        }
        return checker.NextEventCheck(event, logger)
    }`)

	g.P(`// FinalCheck implements the MultiEventChecker interface
    func (checker *OrderedEventChecker) FinalCheck(logger *` + logger + `) error {
        idx := checker.idx
        checker.idx = 0

        if idx >= len(checker.checks) {
            return nil
        }

        return ` + common.FmtErrorf(g, "OrderedEventChecker: only %d/%d matched", "idx", "len(checker.checks)") + `
    }`)

	g.P(`// AddChecks adds one or more checks to the end of this event checker
    func (checker *OrderedEventChecker) AddChecks(checks ...EventChecker) {
        for _, check := range checks {
            checker.checks = append(checker.checks, check)
        }
    }`)

	g.P(`// GetChecks returns this checker's list of checks
    func (checker *OrderedEventChecker) GetChecks() []EventChecker {
        return checker.checks
    }`)

	return nil
}

// generateUnorderedEventChecker generates boilerplate for the unordered MultiEventChecker
func generateUnorderedEventChecker(g *protogen.GeneratedFile, f *protogen.File) error {
	tetragonGER := common.TetragonApiIdent(g, "GetEventsResponse")
	logger := common.GoIdent(g, "github.com/sirupsen/logrus", "Logger")

	listList := common.GoIdent(g, "container/list", "List")

	g.P(`// UnorderedEventChecker checks a series of events in arbitrary order
    type UnorderedEventChecker struct {
        pendingChecks *` + listList + `
        totalChecks   int
        allChecks     *` + listList + `
    }`)

	g.P(`// NewUnorderedEventChecker creates a new UnorderedEventChecker
    func NewUnorderedEventChecker(checks ...EventChecker) *UnorderedEventChecker {
        allList := list.New()
        for _, c := range checks {
            allList.PushBack(c)
        }

        pendingList := list.New()
        pendingList.PushBackList(allList)

        return &UnorderedEventChecker{
            allChecks:     allList,
            pendingChecks: pendingList,
            totalChecks:   len(checks),
        }
    }`)

	g.P(`// NextEventCheck implements the MultiEventChecker interface
    func (checker *UnorderedEventChecker) NextEventCheck(event Event, logger *` + logger + `) (bool, error) {
        pending := checker.pendingChecks.Len()
        if pending == 0 {
            return true, nil
        }

        if logger != nil {
            logger.Infof("UnorderedEventChecker: %d/%d matched", checker.totalChecks-pending, checker.totalChecks)
        }
        idx := 1

        for e := checker.pendingChecks.Front(); e != nil; e = e.Next() {
            check := e.Value.(EventChecker)
            err := check.CheckEvent(event)
            if err == nil {
                checker.pendingChecks.Remove(e)
                pending--
                if pending > 0 {
                    return false, nil
                }

                if logger != nil {
                    logger.Infof("UnorderedEventChecker: all %d checks matched", checker.totalChecks)
                }
                return true, nil
            }
            if logger != nil {
                logger.Infof("UnorderedEventChecker: checking %d/%d: failure: %s", idx, pending, err)
            }
            idx++
        }

        return false, ` + common.FmtErrorf(g, "UnorderedEventChecker: all %d checks failed", "pending") + `
    }`)

	g.P(`// NextResponseCheck implements the MultiEventChecker interface
    func (checker *UnorderedEventChecker) NextResponseCheck(response *` + tetragonGER + `, logger *` + logger + `) (bool, error) {
        event, err := EventFromResponse(response)
        if err != nil {
            return false, err
        }
        return checker.NextEventCheck(event, logger)
    }`)

	g.P(`// FinalCheck implements the MultiEventChecker interface
    func (checker *UnorderedEventChecker) FinalCheck(logger *` + logger + `) error {
        pending := checker.pendingChecks.Len()
        total := checker.totalChecks

        checker.pendingChecks = list.New()
        checker.pendingChecks.PushBackList(checker.allChecks)
        checker.totalChecks = checker.pendingChecks.Len()

        if pending == 0 {
            return nil
        }

        return ` + common.FmtErrorf(g, "UnorderedEventChecker: %d/%d checks remain", "pending", "total") + `
    }`)

	g.P(`// AddChecks adds one or more checks to the set of checks in this event checker
    func (checker *UnorderedEventChecker) AddChecks(checks ...EventChecker) {
        for _, check := range checks {
            checker.pendingChecks.PushBack(check)
            checker.allChecks.PushBack(check)
            checker.totalChecks++
        }
    }`)

	g.P(`// GetChecks returns this checker's list of checks
    func (checker *UnorderedEventChecker) GetChecks() []EventChecker {
        var checks []EventChecker

        for e := checker.allChecks.Front(); e != nil; e = e.Next() {
            if check, ok := e.Value.(EventChecker); ok {
                checks = append(checks, check)
            }
        }

        return checks
    }`)

	return nil
}

// generateFnEventChecker generates boilerplate for the unordered MultiEventChecker
func generateFnEventChecker(g *protogen.GeneratedFile, f *protogen.File) error {
	tetragonGER := common.TetragonApiIdent(g, "GetEventsResponse")
	logger := common.GoIdent(g, "github.com/sirupsen/logrus", "Logger")

	g.P(`// FnEventChecker checks a series of events using custom-defined functions for
    // the MultiEventChecker implementation
    type FnEventChecker struct {
        // NextCheckFn checks an event and returns a boolean value indicating
        // whether the checker has concluded, and an error indicating whether the
        // check was successful. The boolean value allows short-circuiting checks.
        //
        // Specifically:
        // (false,  nil): this event check was successful, but need to check more events
        // (false, !nil): this event check not was successful, but need to check more events
        // (true,   nil): checker was successful, no need to check more events
        // (true,  !nil): checker failed, no need to check more events
        NextCheckFn  func(Event, *` + logger + `) (bool, error)
        // FinalCheckFn indicates that the sequence of events has ended, and asks the
        // checker to make a final decision. Any cleanup should also be performed here.
        FinalCheckFn func(*` + logger + `) error
    }`)

	g.P(`// NextEventCheck implements the MultiEventChecker interface
    func (checker *FnEventChecker) NextEventCheck(event Event, logger *` + logger + `) (bool, error) {
        return checker.NextCheckFn(event, logger)
    }`)

	g.P(`// NextResponseCheck implements the MultiEventChecker interface
    func (checker *FnEventChecker) NextResponseCheck(response *` + tetragonGER + `, logger *` + logger + `) (bool, error) {
        event, err := EventFromResponse(response)
        if err != nil {
            return false, err
        }
        return checker.NextEventCheck(event, logger)
    }`)

	g.P(`// FinalCheck implements the MultiEventChecker interface
    func (checker *FnEventChecker) FinalCheck(logger *` + logger + `) error {
        return checker.FinalCheckFn(logger)
    }`)

	return nil
}

// generateMultiEventCheckerInterface generates the MultiEventChecker interface
func generateMultiEventCheckerInterface(g *protogen.GeneratedFile, f *protogen.File) error {
	tetragonGER := common.TetragonApiIdent(g, "GetEventsResponse")
	logger := common.GoIdent(g, "github.com/sirupsen/logrus", "Logger")

	g.P(`// MultiEventChecker is an interface for checking multiple Tetragon events
        type MultiEventChecker interface {
            // NextResponseCheck checks an response and returns a boolean value indicating
            // whether the checker has concluded, and an error indicating whether the
            // check was successful. The boolean value allows short-circuiting checks.
            //
            // Specifically:
            // (false,  nil): this response check was successful, but need to check more responses
            // (false, !nil): this response check not was successful, but need to check more responses
            // (true,   nil): checker was successful, no need to check more responses
            // (true,  !nil): checker failed, no need to check more responses
            NextResponseCheck(*` + tetragonGER + `, *` + logger + `) (bool, error)

            // NextEventCheck checks an event and returns a boolean value indicating
            // whether the checker has concluded, and an error indicating whether the
            // check was successful. The boolean value allows short-circuiting checks.
            //
            // Specifically:
            // (false,  nil): this event check was successful, but need to check more events
            // (false, !nil): this event check not was successful, but need to check more events
            // (true,   nil): checker was successful, no need to check more events
            // (true,  !nil): checker failed, no need to check more events
            NextEventCheck(Event, *` + logger + `) (bool, error)

            // FinalCheck indicates that the sequence of events has ended, and asks
            // the checker to make a final decision.
            FinalCheck(*` + logger + `) error
        }`)

	return nil
}
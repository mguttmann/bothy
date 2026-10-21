// SPDX-License-Identifier: Apache-2.0

package title

import "github.com/OWNER/bothy/services/server/internal/kernel/errs"

// Source is threads.title_source (03 threads).
type Source string

// The title sources: auto until a principal other than the operator agent sets a title, then
// manual for good (09 section 12 point 1).
const (
	Auto   Source = "auto"
	Manual Source = "manual"
)

// By is the kind of principal that gives a title, classified by the app layer from the request
// principal and the thread's operator agent.
type By uint8

// The classes of principals: a person (actor kind user), the thread's operator agent, and every
// other agent or app.
const (
	ByPerson By = iota + 1
	ByOperatorAgent
	ByOther
)

// String names the class for test output and logs.
func (b By) String() string {
	switch b {
	case ByPerson:
		return "person"
	case ByOperatorAgent:
		return "operator_agent"
	case ByOther:
		return "other"
	}
	return "unknown"
}

// SourceAtCreation returns the source of a new thread whose title by gives (09 section 12 points
// 1 and 2): manual for a person whose title is no placeholder, auto otherwise; an absent title
// counts as empty. Agents and apps always create auto titles. The title is stored as given; this
// only chooses the source.
func (p Placeholders) SourceAtCreation(by By, title string) Source {
	if by == ByPerson && !p.IsPlaceholder(title) {
		return Manual
	}
	return Auto
}

// SourceOnUpdate returns the source after by changes the title of a thread with the source
// current (09 section 12 point 3, 05 thread.title.update): the operator agent keeps auto, every
// other principal sets manual, also with a placeholder text (placeholder detection runs only at
// creation). Errors: title_manual (409) when the operator agent meets a manual title; the caller
// then writes nothing.
func SourceOnUpdate(by By, current Source) (Source, error) {
	if by != ByOperatorAgent {
		return Manual, nil
	}
	if current == Manual {
		return "", errs.New(errs.CodeTitleManual, "the title was set by a person")
	}
	return Auto, nil
}

// SPDX-License-Identifier: Apache-2.0

package guide

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Kind is the kind of a fenced block of the page (README.md "Block grammar").
type Kind string

// The block kinds.
const (
	// Request is an API request of the owner or a person (language http).
	Request Kind = "request"
	// Response is the expected answer to the request before it (language json).
	Response Kind = "response"
	// Shell is a command run to its end in the repository root (language bash).
	Shell Kind = "shell"
	// AdapterStart starts the example adapter in the background (language bash).
	AdapterStart Kind = "adapter-start"
	// AdapterStop interrupts the running adapter (language bash).
	AdapterStop Kind = "adapter-stop"
	// Output is the expected stdout of a command or the next lines of the adapter (language text).
	Output Kind = "output"
	// File is a file of the example adapter, shown whole (language python).
	File Kind = "file"
	// Pin is the excerpt of a pyproject.toml with the SDK version (language toml).
	Pin Kind = "pin"
)

// ExamplesDir is the directory of the example adapter whose files the page shows.
const ExamplesDir = "sdk/python/examples/guide/"

// StopBody is the only body of an "Adapter: stop" block.
const StopBody = "# Ctrl+C in the adapter's terminal"

// The fixed section titles around the ten steps, in page order.
const (
	sectionPlaceholders  = "Placeholders"
	sectionPrerequisites = "Prerequisites"
	sectionChecks        = "How the repository checks this guide"
	sectionNext          = "Next"
)

// actors are the subjects of request blocks; there is no Agent: the agent acts only through the
// example program.
var actors = map[string]bool{"Owner": true, "Alex": true, "Sam": true}

// Block is one fenced block of the page.
type Block struct {
	// Kind is the kind the language and title name.
	Kind Kind
	// Line is the 1-based line of the opening fence.
	Line int
	// Lang and Title are the info string of the fence.
	Lang, Title string
	// Body is the content between the fences, without the final line break.
	Body string
	// Section is the title of the section the block stands in; Step is its step (1 to 10), 0
	// in Prerequisites.
	Section string
	Step    int
	// Actor and OperationID name a request ("<Actor>: <operationId>").
	Actor, OperationID string
	// Status is the status of a response ("Response <status>").
	Status int
	// Path is the repository path of a file block.
	Path string
}

// place is "Step <n>" inside a step and the section title outside.
func place(section string, step int) string {
	if step > 0 {
		return fmt.Sprintf("Step %d", step)
	}
	return section
}

// Where names the block in an error: its step or section, its title and its line.
func (b *Block) Where() string {
	return fmt.Sprintf("%s, block %q (line %d)", place(b.Section, b.Step), b.Title, b.Line)
}

// Section is one second-level heading of the page.
type Section struct {
	// Title is the heading text without "## ".
	Title string
	// Step is 1 to 10 for a step section, else 0.
	Step int
	// Line is the 1-based line of the heading.
	Line int
}

// Page is the parsed guide.
type Page struct {
	// Sections are the second-level headings in page order.
	Sections []Section
	// Blocks are the fenced blocks in page order.
	Blocks []*Block
	// Placeholders are the names in the first column of the table of section Placeholders,
	// without braces.
	Placeholders []string
}

var (
	stepHeading = regexp.MustCompile(`^Step (\d+): (.+)$`)
	fenceInfo   = regexp.MustCompile(`^([a-z]+) title="([^"]*)"$`)
	requestT    = regexp.MustCompile(`^([A-Za-z]+): ([A-Za-z]+)$`)
	responseT   = regexp.MustCompile(`^Response ([1-5][0-9][0-9])$`)
	tableName   = regexp.MustCompile("^\\| `\\{([a-z][a-z0-9_]*)\\}` \\|")
	chapterStep = regexp.MustCompile(`^(\d+)\. \*\*(.+?)\.\*\*`)
)

// StepTitles reads the bold step titles of chapter 11 section 13 without their final period.
func StepTitles(chapter []byte) ([]string, error) {
	var titles []string
	in := false
	sc := bufio.NewScanner(bytes.NewReader(chapter))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") {
			in = strings.HasPrefix(line, "## 13. ")
			continue
		}
		if m := chapterStep.FindStringSubmatch(line); in && m != nil {
			if n, _ := strconv.Atoi(m[1]); n != len(titles)+1 {
				return nil, fmt.Errorf("chapter 11 section 13: step %s out of order", m[1])
			}
			titles = append(titles, m[2])
		}
	}
	if len(titles) == 0 {
		return nil, fmt.Errorf("chapter 11 section 13: no steps found")
	}
	return titles, sc.Err()
}

// Parse reads the page and checks its structure: the sections in order (Placeholders,
// Prerequisites, Step 1 to Step n with the titles of chapter 11 section 13, How the repository
// checks this guide, Next), every fence at column 0 with a language and a title of a known form,
// fences only in Prerequisites and the steps, a response directly after a request, and adapter
// starts and stops in a possible order. The error names the line, or the step, and the reason.
func Parse(src []byte, titles []string) (*Page, error) {
	p := &Page{}
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	section, step := "", 0
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "## "):
			section, step = strings.TrimPrefix(line, "## "), 0
			if m := stepHeading.FindStringSubmatch(section); m != nil {
				step, _ = strconv.Atoi(m[1])
			}
			p.Sections = append(p.Sections, Section{Title: section, Step: step, Line: i + 1})
		case section == sectionPlaceholders && tableName.MatchString(line):
			p.Placeholders = append(p.Placeholders, tableName.FindStringSubmatch(line)[1])
		case strings.HasPrefix(strings.TrimLeft(line, " \t"), "```"):
			if line != strings.TrimLeft(line, " \t") {
				return nil, fmt.Errorf("line %d (%s): an indented fence; every block starts at column 0", i+1, place(section, step))
			}
			b, end, err := fence(lines, i, section, step)
			if err != nil {
				return nil, err
			}
			p.Blocks = append(p.Blocks, b)
			i = end
		}
	}
	if err := checkSections(p.Sections, titles); err != nil {
		return nil, err
	}
	if err := checkOrder(p.Blocks); err != nil {
		return nil, err
	}
	return p, nil
}

// fence reads the block that opens at lines[i] and returns it with the index of its closing
// fence.
func fence(lines []string, i int, section string, step int) (*Block, int, error) {
	info := strings.TrimPrefix(lines[i], "```")
	m := fenceInfo.FindStringSubmatch(info)
	if m == nil {
		return nil, 0, fmt.Errorf("line %d (%s): a fence without a language and a title=\"...\"", i+1, place(section, step))
	}
	if section != sectionPrerequisites && step == 0 {
		return nil, 0, fmt.Errorf("line %d: a block in section %q; blocks stand only in Prerequisites and the steps", i+1, section)
	}
	end := i + 1
	for end < len(lines) && lines[end] != "```" {
		end++
	}
	if end == len(lines) {
		return nil, 0, fmt.Errorf("line %d (%s): the block is not closed", i+1, place(section, step))
	}
	b := &Block{Line: i + 1, Lang: m[1], Title: m[2], Body: strings.Join(lines[i+1:end], "\n"), Section: section, Step: step}
	if err := classify(b); err != nil {
		return nil, 0, fmt.Errorf("line %d (%s): %w", b.Line, place(section, step), err)
	}
	return b, end, nil
}

// classify sets the kind of a block from its language and title.
func classify(b *Block) error {
	switch {
	case b.Lang == "http" && requestT.MatchString(b.Title):
		m := requestT.FindStringSubmatch(b.Title)
		if !actors[m[1]] {
			return fmt.Errorf("unknown actor %q in %q; requests are sent as Owner, Alex or Sam", m[1], b.Title)
		}
		b.Kind, b.Actor, b.OperationID = Request, m[1], m[2]
	case b.Lang == "json" && responseT.MatchString(b.Title):
		b.Kind = Response
		b.Status, _ = strconv.Atoi(responseT.FindStringSubmatch(b.Title)[1])
	case b.Lang == "bash" && b.Title == "Adapter: start":
		if strings.Contains(b.Body, "\n") || strings.TrimSpace(b.Body) == "" {
			return fmt.Errorf("%q holds exactly one command line", b.Title)
		}
		b.Kind = AdapterStart
	case b.Lang == "bash" && b.Title == "Adapter: stop":
		if b.Body != StopBody {
			return fmt.Errorf("%q holds exactly %q", b.Title, StopBody)
		}
		b.Kind = AdapterStop
	case b.Lang == "bash" && strings.HasPrefix(b.Title, "Shell: ") && len(b.Title) > len("Shell: "):
		b.Kind = Shell
	case b.Lang == "text" && b.Title == "Output":
		b.Kind = Output
	case b.Lang == "python" && strings.HasPrefix(b.Title, ExamplesDir) && !strings.Contains(b.Title, ".."):
		b.Kind, b.Path = File, b.Title
	case b.Lang == "toml" && b.Title == "pyproject.toml (excerpt)":
		b.Kind = Pin
	default:
		return fmt.Errorf("unknown block form %s title=%q", b.Lang, b.Title)
	}
	return nil
}

// checkSections holds the second-level headings to the fixed order with the step titles.
func checkSections(got []Section, titles []string) error {
	want := []string{sectionPlaceholders, sectionPrerequisites}
	for n, t := range titles {
		want = append(want, fmt.Sprintf("Step %d: %s", n+1, t))
	}
	want = append(want, sectionChecks, sectionNext)
	for i, w := range want {
		if i >= len(got) {
			return fmt.Errorf("%s: the section is missing", label(w))
		}
		if got[i].Title != w {
			return fmt.Errorf("%s: line %d has the heading %q, want %q (chapter 11 section 13)", label(w), got[i].Line, got[i].Title, w)
		}
	}
	if len(got) > len(want) {
		return fmt.Errorf("line %d: the heading %q is not part of the guide's outline", got[len(want)].Line, got[len(want)].Title)
	}
	return nil
}

// label is "Step <n>" for a step section and the title otherwise.
func label(title string) string {
	if m := stepHeading.FindStringSubmatch(title); m != nil {
		return "Step " + m[1]
	}
	return title
}

// checkOrder checks the order rules between blocks: a response directly after a request, an
// output not as the first block, and adapter starts and stops in a possible order.
func checkOrder(blocks []*Block) error {
	running, stopsItself := false, false
	for i, b := range blocks {
		switch b.Kind {
		case Response:
			if i == 0 || blocks[i-1].Kind != Request {
				return fmt.Errorf("%s: a response stands directly after a request", b.Where())
			}
		case AdapterStart:
			if running && !stopsItself {
				return fmt.Errorf("%s: a second adapter start while the first adapter may still run; stop it first", b.Where())
			}
			running, stopsItself = true, strings.Contains(b.Body, "--stop-after accept")
		case AdapterStop:
			if !running || stopsItself {
				return fmt.Errorf("%s: no adapter runs that this block could stop", b.Where())
			}
			running = false
		case Request, Shell, Output, File, Pin:
		}
	}
	return nil
}

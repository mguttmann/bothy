// SPDX-License-Identifier: Apache-2.0

package guide

import (
	"fmt"
	"strings"
	"testing"
)

// testTitles are ten step titles of a synthetic chapter.
var testTitles = []string{"One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten"}

// fenceOf returns a fenced block with language, title and body.
func fenceOf(lang, title, body string) string {
	return fmt.Sprintf("```%s title=%q\n%s\n```\n", lang, title, body)
}

// synthetic builds a page with the sections in order; extra maps a step (0 for Prerequisites)
// to the content of its section, steps without content get one shell block.
func synthetic(titles []string, extra map[int]string) string {
	var b strings.Builder
	b.WriteString("# Guide\n\n## Placeholders\n\n| Placeholder | Meaning |\n|---|---|\n| `{server_url}` | address |\n\n## Prerequisites\n\n")
	b.WriteString(extra[0])
	for i, t := range titles {
		fmt.Fprintf(&b, "\n## Step %d: %s\n\n", i+1, t)
		if c, ok := extra[i+1]; ok {
			b.WriteString(c)
		} else {
			b.WriteString(fenceOf("bash", "Shell: step", "true"))
		}
	}
	b.WriteString("\n## How the repository checks this guide\n\nProse.\n\n## Next\n\nLinks.\n")
	return b.String()
}

func TestStepTitles_ReadChapter11(t *testing.T) {
	chapter := "## 12. Other\n\n1. **Not this.** x\n\n## 13. Guide\n\n1. **Create the agent.** a\n2. **Create a token.** b\n\n## 14. Next\n"
	got, err := StepTitles([]byte(chapter))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "|") != "Create the agent|Create a token" {
		t.Fatalf("titles = %q", got)
	}
}

func TestParse_StepHeadingsFollowChapter11(t *testing.T) {
	if _, err := Parse([]byte(synthetic(testTitles, nil)), testTitles); err != nil {
		t.Fatalf("the outline in order failed: %v", err)
	}
	renamed := append([]string(nil), testTitles...)
	renamed[2] = "Drei"
	missing := append(append([]string(nil), testTitles[:3]...), testTitles[4:]...)
	duplicated := append(append([]string(nil), testTitles[:5]...), testTitles[4:]...)
	swapped := append([]string(nil), testTitles...)
	swapped[5], swapped[6] = swapped[6], swapped[5]
	page := func(titles []string) string { return synthetic(titles, nil) }
	cases := []struct {
		name, page, want string
	}{
		{"renamed", page(renamed), "Step 3"},
		{"missing", page(missing), "Step 4"},
		{"duplicated", page(duplicated), "Step 6"},
		{"swapped", page(swapped), "Step 6"},
		{"eleventh", strings.Replace(page(testTitles), "\n## How the repository", "\n## Step 11: Eleven\n\nMore.\n\n## How the repository", 1), "Step 11"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.page), testTitles)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one naming %q", err, c.want)
			}
		})
	}
}

func TestParse_BlockKinds(t *testing.T) {
	steps := map[int]string{
		1: fenceOf("http", "Owner: createAgent", "POST /api/v1/agents HTTP/1.1\nContent-Type: application/json\n\n{}") +
			fenceOf("json", "Response 201", `{"id": "{agent_id}"}`),
		2: fenceOf("bash", "Shell: create the key", "echo hi") + fenceOf("text", "Output", "hi"),
		3: fenceOf("python", "sdk/python/examples/guide/agent.py", "print(1)"),
		4: fenceOf("bash", "Adapter: start", "uv run agent.py") + fenceOf("text", "Output", "Step 4: connected"),
		5: fenceOf("bash", "Adapter: stop", StopBody) + fenceOf("toml", "pyproject.toml (excerpt)", "[project]"),
	}
	p, err := Parse([]byte(synthetic(testTitles, steps)), testTitles)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[Kind]*Block{}
	for _, b := range p.Blocks {
		kinds[b.Kind] = b
	}
	for _, k := range []Kind{Request, Response, Shell, AdapterStart, AdapterStop, Output, File, Pin} {
		if kinds[k] == nil {
			t.Fatalf("kind %s not recognised", k)
		}
	}
	if r := kinds[Request]; r.Actor != "Owner" || r.OperationID != "createAgent" || r.Step != 1 {
		t.Fatalf("request = %+v", r)
	}
	if kinds[Response].Status != 201 || kinds[File].Path != "sdk/python/examples/guide/agent.py" {
		t.Fatalf("response or file wrong: %+v %+v", kinds[Response], kinds[File])
	}
	if strings.Join(p.Placeholders, ",") != "server_url" {
		t.Fatalf("placeholders = %q", p.Placeholders)
	}

	bad := []struct {
		name string
		page string
		want string
	}{
		{"untitled", synthetic(testTitles, map[int]string{2: "```bash\necho\n```\n"}), "without a language and a title"},
		{"unknown title form", synthetic(testTitles, map[int]string{2: fenceOf("bash", "Run: x", "echo")}), "unknown block form"},
		{"indented", synthetic(testTitles, map[int]string{2: "  " + fenceOf("bash", "Shell: x", "echo")}), "indented"},
		{"fence in Next", strings.Replace(synthetic(testTitles, nil), "Links.", fenceOf("bash", "Shell: x", "echo"), 1), "section \"Next\""},
		{"agent actor", synthetic(testTitles, map[int]string{2: fenceOf("http", "Agent: startRun", "POST /api/v1/runs/x/start HTTP/1.1")}), "unknown actor \"Agent\""},
		{"stop body", synthetic(testTitles, map[int]string{2: fenceOf("bash", "Adapter: start", "x") + fenceOf("bash", "Adapter: stop", "kill 1")}), "holds exactly"},
		{"second start", synthetic(testTitles, map[int]string{2: fenceOf("bash", "Adapter: start", "x"), 3: fenceOf("bash", "Adapter: start", "y")}), "second adapter start"},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.page), testTitles)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one with %q", err, c.want)
			}
			if !strings.Contains(err.Error(), "line ") {
				t.Fatalf("err = %v names no line", err)
			}
		})
	}

	// A start after a start that stops by itself is allowed.
	self := synthetic(testTitles, map[int]string{2: fenceOf("bash", "Adapter: start", "x --stop-after accept"), 3: fenceOf("bash", "Adapter: start", "y")})
	if _, err := Parse([]byte(self), testTitles); err != nil {
		t.Fatalf("a start after --stop-after accept failed: %v", err)
	}
}

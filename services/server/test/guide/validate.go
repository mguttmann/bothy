// SPDX-License-Identifier: Apache-2.0

package guide

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// RunnerBound are the placeholders the runner binds before the first block; the table of
// section Placeholders lists exactly these.
var RunnerBound = []string{"server_url", "adapter_dir", "alex_id", "sam_id", "new_uuid", "expires_at"}

// Operation is the method and path template of an operation of docs/api/openapi.yaml.
type Operation struct {
	Method, Path string
}

// Inputs are the repository files the page is checked against.
type Inputs struct {
	// Operations are the operations of docs/api/openapi.yaml by operationId.
	Operations map[string]Operation
	// Examples are the files under ExamplesDir by repository path.
	Examples map[string][]byte
	// SDKVersion is the content of sdk/python/VERSION.
	SDKVersion string
}

// Files of the repository the check reads.
const (
	PagePath    = "docs/guides/connect-agent.md"
	ChapterPath = "docs/spec/11-extensions.md"
	OpenAPIPath = "docs/api/openapi.yaml"
	VersionPath = "sdk/python/VERSION"
)

// RepoRoot returns the directory above dir that holds mkdocs.yml.
func RepoRoot(dir string) (string, error) {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "mkdocs.yml")); err == nil {
			return d, nil
		}
		if filepath.Dir(d) == d {
			return "", fmt.Errorf("no mkdocs.yml above %s", dir)
		}
	}
}

// Load reads the page, the step titles and the inputs from the repository at root, and parses
// the page.
func Load(root string) (*Page, Inputs, error) {
	read := func(rel string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- fixed files of the repository
	}
	chapter, err := read(ChapterPath)
	if err != nil {
		return nil, Inputs{}, err
	}
	titles, err := StepTitles(chapter)
	if err != nil {
		return nil, Inputs{}, err
	}
	src, err := read(PagePath)
	if err != nil {
		return nil, Inputs{}, err
	}
	page, err := Parse(src, titles)
	if err != nil {
		return nil, Inputs{}, err
	}
	in, err := LoadInputs(root)
	return page, in, err
}

// LoadInputs reads the operations, the example files and the SDK version.
func LoadInputs(root string) (Inputs, error) {
	in := Inputs{Operations: map[string]Operation{}, Examples: map[string][]byte{}}
	spec, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(OpenAPIPath))) // #nosec G304 -- a fixed file of the repository
	if err != nil {
		return in, err
	}
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		return in, err
	}
	for path, methods := range doc.Paths {
		for method, node := range methods {
			var op struct {
				OperationID string `yaml:"operationId"`
			}
			if node.Kind == yaml.MappingNode && node.Decode(&op) == nil && op.OperationID != "" {
				in.Operations[op.OperationID] = Operation{Method: strings.ToUpper(method), Path: path}
			}
		}
	}
	dir := filepath.Join(root, filepath.FromSlash(ExamplesDir))
	var files []string
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && p != dir && (strings.HasPrefix(d.Name(), ".") || d.Name() == "__pycache__"):
			return filepath.SkipDir
		case !d.IsDir():
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return in, err
	}
	for _, p := range files {
		raw, err := os.ReadFile(p) // #nosec G304 -- files of the example directory
		if err != nil {
			return in, err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return in, err
		}
		in.Examples[filepath.ToSlash(rel)] = raw
	}
	version, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(VersionPath))) // #nosec G304 -- a fixed file of the repository
	in.SDKVersion = strings.TrimSpace(string(version))
	return in, err
}

// Validate holds the parsed page against the inputs before anything runs: every request names an
// operation with its method and path and HTTP/1.1, every shown file equals the example and every
// example is shown once, placeholders are captured before they are used and the table lists the
// runner-bound names, and the pin names the SDK version.
func Validate(p *Page, in Inputs) error {
	return errors.Join(
		checkRequests(p, in.Operations),
		checkFiles(p, in.Examples),
		checkPlaceholders(p),
		checkPin(p, in.SDKVersion),
		checkSteps(p),
	)
}

// checkSteps requires at least one block in every step section.
func checkSteps(p *Page) error {
	seen := map[int]bool{}
	for _, b := range p.Blocks {
		seen[b.Step] = true
	}
	for _, s := range p.Sections {
		if s.Step > 0 && !seen[s.Step] {
			return fmt.Errorf("Step %d: the section has no block, so nothing of it is checked", s.Step) //nolint:staticcheck // ST1005: the place as the page names it
		}
	}
	return nil
}

// RequestLine is the first line of a request block: method, path, version.
type RequestLine struct {
	Method, Path string
	// Header are the header lines (name, value) in order.
	Header [][2]string
	// Body is the JSON body, empty when there is none.
	Body string
}

// allowedHeaders are the only request headers a block may show; the client adds the rest.
var allowedHeaders = map[string]bool{"Content-Type": true, "If-Match": true}

// ParseRequest splits a request block into its line, headers and body.
func ParseRequest(b *Block) (RequestLine, error) {
	lines := strings.Split(b.Body, "\n")
	parts := strings.Fields(lines[0])
	if len(parts) != 3 || parts[2] != "HTTP/1.1" {
		return RequestLine{}, fmt.Errorf("%s: the first line is %q, want <METHOD> /api/v1/<path> HTTP/1.1", b.Where(), lines[0])
	}
	if !strings.HasPrefix(parts[1], "/api/v1/") {
		return RequestLine{}, fmt.Errorf("%s: the path %q does not start with /api/v1/", b.Where(), parts[1])
	}
	r := RequestLine{Method: parts[0], Path: strings.TrimPrefix(parts[1], "/api/v1")}
	i := 1
	for ; i < len(lines) && lines[i] != ""; i++ {
		name, value, ok := strings.Cut(lines[i], ": ")
		if !ok || !allowedHeaders[name] {
			return RequestLine{}, fmt.Errorf("%s: header line %q; only Content-Type and If-Match are shown", b.Where(), lines[i])
		}
		if name == "Content-Type" && value != "application/json" {
			return RequestLine{}, fmt.Errorf("%s: Content-Type %q, want application/json", b.Where(), value)
		}
		r.Header = append(r.Header, [2]string{name, value})
	}
	r.Body = strings.TrimSpace(strings.Join(lines[min(i+1, len(lines)):], "\n"))
	return r, nil
}

// templateParam matches a path parameter of a template or a placeholder of the page.
var templateParam = regexp.MustCompile(`\{[^}]+\}`)

// pathMatches reports whether a path of the page fits a template of the contract: each
// parameter of the template matches one segment, which may be a placeholder.
func pathMatches(template, path string) bool {
	quoted := regexp.QuoteMeta(templateParam.ReplaceAllString(template, "\x00"))
	return regexp.MustCompile("^" + strings.ReplaceAll(quoted, "\x00", `[^/?]+`) + `(\?.*)?$`).MatchString(path)
}

func checkRequests(p *Page, ops map[string]Operation) error {
	for _, b := range p.Blocks {
		if b.Kind != Request {
			continue
		}
		op, ok := ops[b.OperationID]
		if !ok {
			return fmt.Errorf("%s: unknown operationId %s in %s", b.Where(), b.OperationID, OpenAPIPath)
		}
		r, err := ParseRequest(b)
		if err != nil {
			return err
		}
		if r.Method != op.Method {
			return fmt.Errorf("%s: method %s, but %s is %s", b.Where(), r.Method, b.OperationID, op.Method)
		}
		if !pathMatches(op.Path, r.Path) {
			return fmt.Errorf("%s: path %s does not fit %s of %s", b.Where(), r.Path, op.Path, b.OperationID)
		}
		if r.Body != "" {
			if _, err := decode([]byte(r.Body)); err != nil {
				return fmt.Errorf("%s: the body is not JSON: %w", b.Where(), err)
			}
		}
	}
	return nil
}

func checkFiles(p *Page, examples map[string][]byte) error {
	shown := map[string]*Block{}
	for _, b := range p.Blocks {
		if b.Kind != File {
			continue
		}
		if prev := shown[b.Path]; prev != nil {
			return fmt.Errorf("%s: %s is shown twice (first at line %d)", b.Where(), b.Path, prev.Line)
		}
		shown[b.Path] = b
		want, ok := examples[b.Path]
		if !ok {
			return fmt.Errorf("%s: %s: no such file in the repository", b.Where(), b.Path)
		}
		got := []byte(b.Body + "\n")
		if !bytes.Equal(got, want) {
			return fmt.Errorf("%s: %s differs from the repository at line %d", b.Where(), b.Path, firstDiff(got, want))
		}
	}
	var missing []string
	for path := range examples {
		if shown[path] == nil {
			missing = append(missing, path)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("%s: not shown on the page; every file of the example appears in one block", strings.Join(missing, ", "))
	}
	return nil
}

// firstDiff returns the 1-based line where a and b first differ.
func firstDiff(a, b []byte) int {
	al, bl := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
	for i := 0; i < len(al) && i < len(bl); i++ {
		if al[i] != bl[i] {
			return i + 1
		}
	}
	return min(len(al), len(bl)) + 1
}

// scanned reports whether placeholders of a block are substituted or captured; file and pin
// blocks are compared byte for byte.
func scanned(k Kind) bool { return k != File && k != Pin }

// captures reports whether an unbound placeholder in a block of kind k captures a value.
func captures(k Kind) bool { return k == Response || k == Output }

func checkPlaceholders(p *Page) error {
	bound := map[string]bool{}
	for _, n := range RunnerBound {
		bound[n] = true
	}
	if err := checkTable(p.Placeholders); err != nil {
		return err
	}
	// Names captured anywhere, to tell a use before the capture from an unknown name.
	capturedSomewhere := map[string]bool{}
	for _, b := range p.Blocks {
		if captures(b.Kind) {
			for _, m := range placeholderPattern.FindAllStringSubmatch(b.Body, -1) {
				capturedSomewhere[m[1]] = true
			}
		}
	}
	for _, b := range p.Blocks {
		if !scanned(b.Kind) {
			continue
		}
		inBlock := map[string]bool{}
		for _, m := range placeholderPattern.FindAllStringSubmatch(b.Body, -1) {
			name := m[1]
			if strings.HasPrefix(name, secretPrefix) {
				if b.Kind != Response {
					return fmt.Errorf("%s: {%s} may stand only in a response block", b.Where(), name)
				}
				if bound[name] {
					return fmt.Errorf("%s: {%s} captured twice", b.Where(), name)
				}
				bound[name] = true
				continue
			}
			switch {
			case bound[name] && !inBlock[name]:
			case captures(b.Kind) && !inBlock[name]:
				inBlock[name] = true
			case inBlock[name] && b.Kind == Output:
				// Output lines match in order: a later line compares what an earlier one captured.
			case inBlock[name]:
				return fmt.Errorf("%s: {%s} captured twice in one block", b.Where(), name)
			case capturedSomewhere[name]:
				return fmt.Errorf("%s: {%s} is used before it is captured", b.Where(), name)
			default:
				return fmt.Errorf("%s: unknown placeholder {%s}", b.Where(), name)
			}
		}
		for name := range inBlock {
			bound[name] = true
		}
	}
	return nil
}

// checkTable requires the table of section Placeholders to list exactly the runner-bound names.
func checkTable(names []string) error {
	want := map[string]bool{}
	for _, n := range RunnerBound {
		want[n] = true
	}
	got := map[string]bool{}
	for _, n := range names {
		if !want[n] {
			return fmt.Errorf("Placeholders: the table lists {%s}, which the runner does not bind", n) //nolint:staticcheck // ST1005: the section as the page names it
		}
		got[n] = true
	}
	for _, n := range RunnerBound {
		if !got[n] {
			return fmt.Errorf("Placeholders: the table lacks {%s}", n) //nolint:staticcheck // ST1005: the section as the page names it
		}
	}
	return nil
}

// sdkDistribution is the name of the SDK's distribution in a pin.
const sdkDistribution = "bothy-sdk"

func checkPin(p *Page, version string) error {
	for _, b := range p.Blocks {
		if b.Kind != Pin {
			continue
		}
		doc, err := parseTOML(b.Body)
		if err != nil {
			return fmt.Errorf("%s: not TOML the check reads: %w", b.Where(), err)
		}
		deps := doc["project.dependencies"]
		want := sdkDistribution + "==" + version
		var sdk []string
		for _, d := range deps {
			if strings.HasPrefix(d, sdkDistribution) {
				sdk = append(sdk, d)
			}
		}
		if len(sdk) != 1 {
			return fmt.Errorf("%s: project.dependencies holds %d entries for %s, want exactly one: %s", b.Where(), len(sdk), sdkDistribution, want)
		}
		if sdk[0] != want {
			return fmt.Errorf("%s: the pin is %s, want %s (%s)", b.Where(), sdk[0], want, VersionPath)
		}
	}
	return nil
}

// tomlKey is a bare key of TOML.
var tomlKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// parseTOML reads the subset of TOML a pyproject excerpt needs: table headers, comments, and
// keys with a basic string or an array of basic strings (possibly over several lines). The
// result maps "table.key" to its strings. Anything else is an error, so the block stays an
// excerpt a reader can paste.
func parseTOML(src string) (map[string][]string, error) {
	out := map[string][]string{}
	table := ""
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "["):
			if !strings.HasSuffix(line, "]") || strings.HasPrefix(line, "[[") {
				return nil, fmt.Errorf("line %d: invalid table header %q", i+1, line)
			}
			name := strings.TrimSpace(line[1 : len(line)-1])
			for _, part := range strings.Split(name, ".") {
				if !tomlKey.MatchString(part) {
					return nil, fmt.Errorf("line %d: invalid table name %q", i+1, name)
				}
			}
			table = name
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || !tomlKey.MatchString(key) {
			return nil, fmt.Errorf("line %d: invalid key/value %q", i+1, line)
		}
		full := key
		if table != "" {
			full = table + "." + key
		}
		if _, dup := out[full]; dup {
			return nil, fmt.Errorf("line %d: key %s defined twice", i+1, full)
		}
		if strings.HasPrefix(value, "[") {
			for !strings.HasSuffix(strings.TrimSpace(stripComment(value)), "]") {
				i++
				if i >= len(lines) {
					return nil, fmt.Errorf("the array of %s is not closed", full)
				}
				value += " " + strings.TrimSpace(lines[i])
			}
			items, err := stringArray(stripComment(value))
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			out[full] = items
			continue
		}
		s, err := basicString(stripComment(value))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		out[full] = []string{s}
	}
	return out, nil
}

// stripComment removes a trailing comment outside strings.
func stripComment(v string) string {
	in := false
	for i, r := range v {
		switch {
		case r == '"' && (i == 0 || v[i-1] != '\\'):
			in = !in
		case r == '#' && !in:
			return strings.TrimSpace(v[:i])
		}
	}
	return strings.TrimSpace(v)
}

// stringArray reads ["a", "b",] .
func stringArray(v string) ([]string, error) {
	inner := strings.TrimSpace(v[1 : len(v)-1])
	var out []string
	for inner != "" {
		if !strings.HasPrefix(inner, `"`) {
			return nil, fmt.Errorf("an array of strings is expected")
		}
		end := strings.Index(inner[1:], `"`)
		if end < 0 {
			return nil, fmt.Errorf("an unterminated string")
		}
		s, err := strconv.Unquote(inner[:end+2])
		if err != nil {
			return nil, err
		}
		out = append(out, s)
		inner = strings.TrimSpace(inner[end+2:])
		if inner == "" {
			break
		}
		if !strings.HasPrefix(inner, ",") {
			return nil, fmt.Errorf("a comma is missing between the strings")
		}
		inner = strings.TrimSpace(inner[1:])
	}
	return out, nil
}

// basicString reads "text".
func basicString(v string) (string, error) {
	if len(v) < 2 || !strings.HasPrefix(v, `"`) || !strings.HasSuffix(v, `"`) {
		return "", fmt.Errorf("a basic string is expected, got %q", v)
	}
	return strconv.Unquote(v)
}

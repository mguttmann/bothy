// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets/stack"
)

// docRow is one row of the table in configuration.md section 5 in the normalized form both
// sides of TestCatalogMatchesConfigurationDoc are compared in.
type docRow struct {
	name     string
	origin   string // "generated", "supplied" or "generated or supplied"
	services string // comma separated as in the table
}

// TestCatalogMatchesConfigurationDoc keeps the Go catalog equal to the table the operator reads
// (D6): a row added, removed or changed on one side only fails here.
func TestCatalogMatchesConfigurationDoc(t *testing.T) {
	t.Parallel()
	want := parseSecretsTable(t, string(repoFile(t, "docs/operations/configuration.md")))
	if len(want) == 0 {
		t.Fatal("no rows parsed from configuration.md section 5")
	}
	var got []docRow
	for _, e := range stack.Catalog() {
		got = append(got, docRow{name: e.Name, origin: docOrigin(t, e), services: strings.Join(e.Services, ", ")})
	}
	gotByName := map[string]docRow{}
	for _, r := range got {
		gotByName[r.name] = r
	}
	wantByName := map[string]docRow{}
	for _, r := range want {
		wantByName[r.name] = r
	}
	for _, w := range want {
		g, ok := gotByName[w.name]
		switch {
		case !ok:
			t.Errorf("configuration.md names %s, the Go catalog does not", w.name)
		case g != w:
			t.Errorf("entry %s: Go catalog %+v, configuration.md %+v", w.name, g, w)
		}
	}
	for _, g := range got {
		if _, ok := wantByName[g.name]; !ok {
			t.Errorf("the Go catalog names %s, configuration.md does not", g.name)
		}
	}
	var gotNames, wantNames []string
	for _, r := range got {
		gotNames = append(gotNames, r.name)
	}
	for _, r := range want {
		wantNames = append(wantNames, r.name)
	}
	if !slices.Equal(gotNames, wantNames) {
		t.Errorf("catalog order differs from configuration.md:\n got %v\nwant %v", gotNames, wantNames)
	}
}

// docOrigin maps an entry to the wording of the column "Origin"; a supplied entry whose
// generator is pending is the row "generated or supplied".
func docOrigin(t *testing.T, e stack.Entry) string {
	t.Helper()
	switch {
	case e.Origin == stack.OriginGenerated:
		return "generated"
	case e.Origin == stack.OriginSupplied && e.Generator == stack.GeneratorPending:
		return "generated or supplied"
	case e.Origin == stack.OriginSupplied:
		return "supplied"
	}
	t.Errorf("entry %s has unknown origin %q", e.Name, e.Origin)
	return ""
}

var (
	backticked  = regexp.MustCompile("`([^`]+)`")
	parenthesis = regexp.MustCompile(`\s*\([^)]*\)`)
)

// parseSecretsTable reads the first table after "## 5. Secrets".
func parseSecretsTable(t *testing.T, text string) []docRow {
	t.Helper()
	start := strings.Index(text, "\n## 5. Secrets\n")
	if start < 0 {
		t.Fatal("configuration.md has no section 5. Secrets")
	}
	var rows []docRow
	inTable := false
	for _, line := range strings.Split(text[start+1:], "\n") {
		if !strings.HasPrefix(line, "|") {
			if inTable {
				break
			}
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if !inTable {
			inTable = true // header row
			continue
		}
		if strings.Trim(cells[0], "-") == "" || len(cells) < 3 {
			continue // separator row
		}
		origin := parenthesis.ReplaceAllString(cells[1], "")
		if i := strings.Index(origin, ","); i >= 0 {
			origin = origin[:i] // "generated, 32 bytes"
		}
		origin = strings.TrimSpace(origin)
		var services []string
		for _, s := range strings.Split(cells[2], ",") {
			services = append(services, strings.TrimSpace(s))
		}
		for _, m := range backticked.FindAllStringSubmatch(cells[0], -1) {
			rows = append(rows, docRow{
				name:     m[1],
				origin:   origin,
				services: strings.Join(services, ", "),
			})
		}
	}
	return rows
}

// repoFile reads a file relative to the repository root, found upwards from the package.
func repoFile(t *testing.T, rel string) []byte {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, "docs", "operations")); err == nil && info.IsDir() {
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			data, err := root.ReadFile(rel)
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no directory with docs/operations above the working directory")
		}
		dir = parent
	}
}

func TestCatalogIsValid(t *testing.T) {
	t.Parallel()
	serviceName := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	seen := map[string]bool{}
	for _, e := range stack.Catalog() {
		if seen[e.Name] {
			t.Errorf("duplicate entry %s", e.Name)
		}
		seen[e.Name] = true
		if len(e.Services) == 0 {
			t.Errorf("entry %s lists no service", e.Name)
		}
		for _, s := range e.Services {
			if s == "_store" || !serviceName.MatchString(s) {
				t.Errorf("entry %s: invalid service %q", e.Name, s)
			}
		}
		if e.Pattern != "" {
			if !strings.HasPrefix(e.Pattern, "^") || !strings.HasSuffix(e.Pattern, "$") {
				t.Errorf("entry %s: pattern %q is not anchored", e.Name, e.Pattern)
			}
			if _, err := regexp.Compile(e.Pattern); err != nil {
				t.Errorf("entry %s: pattern %q: %v", e.Name, e.Pattern, err)
			}
			if strings.HasSuffix(e.Name, "/") {
				t.Errorf("entry %s is a pattern and a directory", e.Name)
			}
		}
		switch e.Origin {
		case stack.OriginGenerated:
			if e.Generator != stack.GeneratorToken && e.Generator != stack.GeneratorKey && e.Generator != stack.GeneratorPending {
				t.Errorf("generated entry %s has generator %q", e.Name, e.Generator)
			}
		case stack.OriginSupplied:
			if e.Generator != stack.GeneratorNone && e.Generator != stack.GeneratorPending {
				t.Errorf("supplied entry %s has generator %q", e.Name, e.Generator)
			}
		default:
			t.Errorf("entry %s has origin %q", e.Name, e.Origin)
		}
		if (e.Generator == stack.GeneratorPending) != (e.PendingOn != "") {
			t.Errorf("entry %s: generator %q with PendingOn %q", e.Name, e.Generator, e.PendingOn)
		}
	}
}

// TestCatalogPendingAndGenerators pins the generator of every entry that brief D5 and D6 name,
// so a key never silently becomes a token or a pending entry starts generating.
func TestCatalogPendingAndGenerators(t *testing.T) {
	t.Parallel()
	want := map[string]struct {
		gen       stack.Generator
		pendingOn string
	}{
		"db_superuser_password":   {stack.GeneratorToken, ""},
		"db_owner_password":       {stack.GeneratorToken, ""},
		"db_app_password":         {stack.GeneratorToken, ""},
		"db_maint_password":       {stack.GeneratorToken, ""},
		"db_monitor_password":     {stack.GeneratorToken, ""},
		"pgbackrest_cipher_pass":  {stack.GeneratorToken, ""},
		"restic_password":         {stack.GeneratorToken, ""},
		"grafana_admin_password":  {stack.GeneratorToken, ""},
		"token_pepper":            {stack.GeneratorKey, ""},
		"telemetry_pseudonym_key": {stack.GeneratorKey, ""},
		"audit_ip_kek":            {stack.GeneratorKey, ""},
		"file_ticket_key":         {stack.GeneratorKey, ""},
		"totp_key":                {stack.GeneratorPending, "M1-39"},
		"pgbackrest_tls/":         {stack.GeneratorPending, "M7-01"},
		"s3_access_key":           {stack.GeneratorPending, "M3"},
		"s3_secret_key":           {stack.GeneratorPending, "M3"},
	}
	byName := map[string]stack.Entry{}
	for _, e := range stack.Catalog() {
		byName[e.Name] = e
	}
	for name, w := range want {
		e, ok := byName[name]
		if !ok {
			t.Errorf("catalog has no entry %s", name)
			continue
		}
		if e.Generator != w.gen || e.PendingOn != w.pendingOn {
			t.Errorf("%s: generator %q pending on %q, want %q and %q", name, e.Generator, e.PendingOn, w.gen, w.pendingOn)
		}
	}
	for _, e := range stack.Catalog() {
		want := ""
		switch e.Name {
		case "oidc/<provider>_client_secret":
			want = `^oidc/[a-z0-9_-]+_client_secret$`
		case "oidc/<provider>_client_key.pem":
			want = `^oidc/[a-z0-9_-]+_client_key\.pem$`
		}
		if e.Pattern != want {
			t.Errorf("%s: pattern %q, want %q", e.Name, e.Pattern, want)
		}
	}
}

func TestCatalogReturnsCopies(t *testing.T) {
	t.Parallel()
	first := stack.Catalog()
	first[0].Name = "EXAMPLE-changed"
	first[0].Services[0] = "EXAMPLE-changed"
	services := stack.Services()
	services[0] = "EXAMPLE-changed"
	second := stack.Catalog()
	if second[0].Name == "EXAMPLE-changed" || second[0].Services[0] == "EXAMPLE-changed" {
		t.Error("Catalog returns shared state")
	}
	if stack.Services()[0] == "EXAMPLE-changed" {
		t.Error("Services returns shared state")
	}
}

func TestServicesSortedUnique(t *testing.T) {
	t.Parallel()
	want := []string{"alertmanager", "backup", "collector", "grafana", "migrate", "postgres", "seaweedfs", "server"}
	if got := stack.Services(); !slices.Equal(got, want) {
		t.Errorf("Services() = %v, want %v", got, want)
	}
}

// TestCatalogServicesMatchComposeSubpaths keeps the service directories init creates equal to the
// subpath mounts of the stack: a service without its directory fails to start in Compose.
func TestCatalogServicesMatchComposeSubpaths(t *testing.T) {
	t.Parallel()
	text := string(repoFile(t, "infra/compose/compose.yaml"))
	var subpaths []string
	for _, m := range regexp.MustCompile(`(?m)^\s*subpath:\s*([^\s#]+)\s*$`).FindAllStringSubmatch(text, -1) {
		if !slices.Contains(subpaths, m[1]) {
			subpaths = append(subpaths, m[1])
		}
	}
	slices.Sort(subpaths)
	if got := stack.Services(); !slices.Equal(got, subpaths) {
		t.Errorf("Services() = %v, compose.yaml subpaths %v", got, subpaths)
	}

	block := regexp.MustCompile(`(?ms)^  secrets-init:\n(.*?)^  [a-z]`).FindStringSubmatch(text)
	if block == nil {
		t.Fatal("compose.yaml has no service secrets-init")
	}
	command := regexp.MustCompile(`(?s)command:\s*\[(.*?)\]`).FindStringSubmatch(block[1])
	if command == nil {
		t.Fatal("service secrets-init has no command list")
	}
	var args []string
	for _, m := range regexp.MustCompile(`"([^"]*)"`).FindAllStringSubmatch(command[1], -1) {
		args = append(args, m[1])
	}
	for _, want := range []string{"server=65532:65532", "migrate=65532:65532"} {
		found := false
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--owner" && args[i+1] == want {
				found = true
			}
		}
		if !found {
			t.Errorf("secrets-init command %v lacks --owner %s", args, want)
			continue
		}
		if _, _, err := stack.ParseOwner(want); err != nil {
			t.Errorf("ParseOwner(%q): %v", want, err)
		}
	}
}

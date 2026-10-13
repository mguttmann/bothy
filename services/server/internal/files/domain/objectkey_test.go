// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// exampleID is the file ID of the example in docs/spec/12-files.md "4.4 Key schema" as
// the brief of M3-01 spells it out.
const exampleID = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b"

// keyGrammar is the grammar of 12 "4.4": the original, the four variants and a part key.
var keyGrammar = regexp.MustCompile(`^(f/[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}/(orig-[1-9][0-9]*|small|medium|large|poster)|uploads/[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}/[1-9][0-9]*)$`)

// uuidPattern finds every UUID in a key, with or without dashes.
var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-?[0-9a-f]{4}-?[0-9a-f]{4}-?[0-9a-f]{4}-?[0-9a-f]{12}`)

// randomID draws a UUIDv7 from r, so property tests are reproducible from their seed.
func randomID(r *rand.Rand) ids.ID {
	var id ids.ID
	_, _ = r.Read(id[:])
	id[6] = 0x70 | id[6]&0x0f
	id[8] = 0x80 | id[8]&0x3f
	return id
}

func mustOriginal(t *testing.T, id ids.ID, rev int) string {
	t.Helper()
	k, err := domain.OriginalKey(id, rev)
	if err != nil {
		t.Fatalf("OriginalKey(%s, %d): %v", id, rev, err)
	}
	return k
}

func TestOriginalKeyLayout(t *testing.T) {
	t.Parallel()
	id := ids.MustParse(exampleID)
	if got, want := mustOriginal(t, id, 1), "f/5a/6b/"+exampleID+"/orig-1"; got != want {
		t.Errorf("OriginalKey(rev 1) = %q, want %q", got, want)
	}
	if got, want := mustOriginal(t, id, 2), "f/5a/6b/"+exampleID+"/orig-2"; got != want {
		t.Errorf("OriginalKey(rev 2) = %q, want %q", got, want)
	}
	for _, rev := range []int{0, -1} {
		if k, err := domain.OriginalKey(id, rev); !errors.Is(err, domain.ErrInvalidKey) || k != "" {
			t.Errorf("OriginalKey(rev %d) = %q, %v; want ErrInvalidKey", rev, k, err)
		}
	}
	if k, err := domain.OriginalKey(ids.ID{}, 1); !errors.Is(err, domain.ErrInvalidKey) || k != "" {
		t.Errorf("OriginalKey(nil ID) = %q, %v; want ErrInvalidKey", k, err)
	}
}

func TestVariantKeys(t *testing.T) {
	t.Parallel()
	id := ids.MustParse(exampleID)
	prefix := "f/5a/6b/" + exampleID + "/"
	for _, v := range []domain.Variant{domain.VariantSmall, domain.VariantMedium, domain.VariantLarge, domain.VariantPoster} {
		k, err := domain.VariantKey(id, v)
		if err != nil || k != prefix+string(v) {
			t.Errorf("VariantKey(%s) = %q, %v; want %q", v, k, err, prefix+string(v))
		}
	}
	for _, v := range []domain.Variant{"", "playback", "orig-1", "Small", "../x"} {
		if k, err := domain.VariantKey(id, v); !errors.Is(err, domain.ErrInvalidKey) || k != "" {
			t.Errorf("VariantKey(%q) = %q, %v; want ErrInvalidKey", v, k, err)
		}
	}
	if _, err := domain.VariantKey(ids.ID{}, domain.VariantSmall); !errors.Is(err, domain.ErrInvalidKey) {
		t.Errorf("VariantKey(nil ID) err = %v, want ErrInvalidKey", err)
	}
}

func TestUploadPartKey(t *testing.T) {
	t.Parallel()
	id := ids.MustParse(exampleID)
	if k, err := domain.UploadPartKey(id, 1); err != nil || k != "uploads/"+exampleID+"/1" {
		t.Errorf("UploadPartKey(1) = %q, %v", k, err)
	}
	if k, err := domain.UploadPartKey(id, 12); err != nil || k != "uploads/"+exampleID+"/12" {
		t.Errorf("UploadPartKey(12) = %q, %v", k, err)
	}
	for _, n := range []int{0, -3} {
		if k, err := domain.UploadPartKey(id, n); !errors.Is(err, domain.ErrInvalidKey) || k != "" {
			t.Errorf("UploadPartKey(%d) = %q, %v; want ErrInvalidKey", n, k, err)
		}
	}
	if _, err := domain.UploadPartKey(ids.ID{}, 1); !errors.Is(err, domain.ErrInvalidKey) {
		t.Errorf("UploadPartKey(nil ID) err = %v, want ErrInvalidKey", err)
	}
}

func TestParseOriginalKey(t *testing.T) {
	t.Parallel()
	seed := time.Now().UnixNano()
	t.Logf("seed %d", seed)
	r := rand.New(rand.NewSource(seed)) //nolint:gosec // G404: reproducible test data from a logged seed, not security
	for range 500 {
		id := randomID(r)
		rev := 1 + r.Intn(1_000_000)
		gotID, gotRev, err := domain.ParseOriginalKey(mustOriginal(t, id, rev))
		if err != nil || gotID != id || gotRev != rev {
			t.Fatalf("round trip %s rev %d: got %s rev %d, %v", id, rev, gotID, gotRev, err)
		}
	}
	id := ids.MustParse(exampleID)
	small, _ := domain.VariantKey(id, domain.VariantSmall)
	part, _ := domain.UploadPartKey(id, 1)
	refused := []string{
		small,
		part,
		"f/5a/6b/" + exampleID + "/orig-01",
		"f/5a/6b/" + exampleID + "/orig-0",
		"f/5a/6b/" + exampleID + "/orig-",
		"f/5a/6b/" + exampleID + "/orig-1/x",
		"f/6b/5a/" + exampleID + "/orig-1",
		"f/00/00/" + exampleID + "/orig-1",
		"f/5A/6B/" + strings.ToUpper(exampleID) + "/orig-1",
		"g/5a/6b/" + exampleID + "/orig-1",
		"f/5a/6b/00000000-0000-0000-0000-000000000000/orig-1",
		"f/5a/6b/" + exampleID + "/orig-99999999999999999999999",
		"",
	}
	for _, k := range refused {
		if gotID, rev, err := domain.ParseOriginalKey(k); !errors.Is(err, domain.ErrInvalidKey) || !gotID.IsZero() || rev != 0 {
			t.Errorf("ParseOriginalKey(%q) = %s, %d, %v; want ErrInvalidKey", k, gotID, rev, err)
		}
	}
}

func TestValidateKey(t *testing.T) {
	t.Parallel()
	id := ids.MustParse(exampleID)
	accepted := []string{"a", "a-b/c", "uploads/x/1", mustOriginal(t, id, 1), mustOriginal(t, id, 77)}
	for _, v := range []domain.Variant{domain.VariantSmall, domain.VariantMedium, domain.VariantLarge, domain.VariantPoster} {
		k, _ := domain.VariantKey(id, v)
		accepted = append(accepted, k)
	}
	p, _ := domain.UploadPartKey(id, 3)
	accepted = append(accepted, p)
	for _, k := range accepted {
		if err := domain.ValidateKey(k); err != nil {
			t.Errorf("ValidateKey(%q) = %v, want nil", k, err)
		}
	}
	refused := []string{
		"", ".", "..", "../x", "a/../b", "a/..", "/abs", "a//b", "a/", "/", ".tmp/x", "A", "a.b",
		"a b", `a\b`, "%2e%2e", "a\u202eb", "a\x00b", "a\u00e9", "a\nb", "a\tb", "-/", "a/./b",
	}
	for _, k := range refused {
		err := domain.ValidateKey(k)
		if !errors.Is(err, domain.ErrInvalidKey) {
			t.Errorf("ValidateKey(%q) = %v, want ErrInvalidKey", k, err)
			continue
		}
		if k != "" && strings.Contains(err.Error(), k) {
			t.Errorf("ValidateKey(%q): error %q repeats the key", k, err)
		}
	}
}

// keyContext is one random context of TestObjectKeysCarryNoContext: the IDs and names a file has
// around it, which must never reach a key.
type keyContext struct {
	FileID, ChannelID, SpaceID       ids.ID
	Revision, Part                   int
	Variant                          domain.Variant
	FileName, ChannelName, SpaceName string
}

var hostileNames = []string{
	"../../etc/passwd", "Report from Alex.pdf", "Sam\u202efdp.exe", "robin", "orig-1", "small",
	"f", "uploads", "a", "1", "-",
}

// Generate makes keyContext a quick.Generator.
func (keyContext) Generate(r *rand.Rand, _ int) reflect.Value {
	variants := []domain.Variant{domain.VariantSmall, domain.VariantMedium, domain.VariantLarge, domain.VariantPoster}
	c := keyContext{
		FileID: randomID(r), ChannelID: randomID(r), SpaceID: randomID(r),
		Revision: 1 + r.Intn(1000), Part: 1 + r.Intn(10000),
		Variant: variants[r.Intn(len(variants))],
	}
	name := func() string {
		switch r.Intn(4) {
		case 0:
			return hostileNames[r.Intn(len(hostileNames))]
		case 1:
			return c.ChannelID.String()
		case 2:
			return strings.ReplaceAll(c.SpaceID.String(), "-", "")
		default:
			b := make([]rune, 3+r.Intn(20))
			for i := range b {
				b[i] = 0x20 + r.Int31n(0x2fff)
			}
			return string(b)
		}
	}
	c.FileName, c.ChannelName, c.SpaceName = name(), name(), name()
	return reflect.ValueOf(c)
}

// TestObjectKeysCarryNoContext is the property test of 12 "15. Tests" row "Key schema":
// no key carries a channel ID, a space ID or a name. The builders cannot take a name at all
// (TestKeyBuildersTakeOnlyIDsAndNumbers); this test holds what they make of the IDs they take.
func TestObjectKeysCarryNoContext(t *testing.T) {
	t.Parallel()
	seed := time.Now().UnixNano()
	t.Logf("seed %d", seed)
	prop := func(c keyContext) bool {
		orig, err1 := domain.OriginalKey(c.FileID, c.Revision)
		variant, err2 := domain.VariantKey(c.FileID, c.Variant)
		part, err3 := domain.UploadPartKey(c.FileID, c.Part)
		if err1 != nil || err2 != nil || err3 != nil {
			t.Logf("builder error: %v %v %v", err1, err2, err3)
			return false
		}
		for _, k := range []string{orig, variant, part} {
			if !keyGrammar.MatchString(k) || domain.ValidateKey(k) != nil {
				t.Logf("key %q is outside the grammar", k)
				return false
			}
			for _, u := range uuidPattern.FindAllString(k, -1) {
				if strings.ReplaceAll(u, "-", "") != strings.ReplaceAll(c.FileID.String(), "-", "") {
					t.Logf("key %q holds the UUID %s that is not the file ID", k, u)
					return false
				}
			}
			for _, other := range []ids.ID{c.ChannelID, c.SpaceID} {
				if strings.Contains(k, other.String()) || strings.Contains(k, strings.ReplaceAll(other.String(), "-", "")) {
					t.Logf("key %q holds a channel or space ID", k)
					return false
				}
			}
			for _, n := range []string{c.FileName, c.ChannelName, c.SpaceName} {
				// A name that is also a building block of the grammar ("f", "small", digits, hex)
				// can be a substring by coincidence; it still never flowed into the key, which the
				// reflection test holds. Only names that cannot occur by construction are checked
				// here; a name equal to the channel or space ID (with or without dashes) is held
				// by the ID check above.
				if len(n) > 7 && !keyGrammarPiece(n) && strings.Contains(k, n) {
					t.Logf("key %q holds the name %q", k, n)
					return false
				}
			}
		}
		return true
	}
	cfg := &quick.Config{MaxCount: 2000, Rand: rand.New(rand.NewSource(seed))} //nolint:gosec // G404: reproducible test data from a logged seed
	if err := quick.Check(prop, cfg); err != nil {
		t.Fatal(err)
	}
}

// keyGrammarPiece reports whether n could be part of a key built only from the file ID: then a
// match says nothing about a leak.
func keyGrammarPiece(n string) bool {
	return regexp.MustCompile(`^[0-9a-f/-]*$|^orig-[0-9]+$|^(small|medium|large|poster|uploads)$`).MatchString(n)
}

// TestKeyBuildersTakeOnlyIDsAndNumbers: every exported key builder takes only ids.ID, int and
// Variant, so no text (file name, channel name, person) can flow into a key.
func TestKeyBuildersTakeOnlyIDsAndNumbers(t *testing.T) {
	t.Parallel()
	allowed := map[reflect.Type]bool{
		reflect.TypeFor[ids.ID](): true, reflect.TypeFor[int](): true, reflect.TypeFor[domain.Variant](): true,
	}
	builders := map[string]any{
		"OriginalKey": domain.OriginalKey, "VariantKey": domain.VariantKey, "UploadPartKey": domain.UploadPartKey,
	}
	// The list above must hold every exported builder: every exported function of the package
	// whose name ends in Key, found in the source, except the checks that take a key.
	declared := exportedKeyFuncs(t)
	for name := range builders {
		delete(declared, name)
	}
	for name := range declared {
		t.Errorf("exported function %s builds a key but is not checked here", name)
	}
	for name, fn := range builders {
		ft := reflect.TypeOf(fn)
		for i := range ft.NumIn() {
			if !allowed[ft.In(i)] {
				t.Errorf("%s parameter %d has type %s; only ids.ID, int and Variant may build a key", name, i, ft.In(i))
			}
		}
		if ft.NumOut() != 2 || ft.Out(0).Kind() != reflect.String {
			t.Errorf("%s returns %s, want (string, error)", name, ft)
		}
	}
	// Variant must stay a closed string type: a caller cannot pass a name without a conversion,
	// and VariantKey refuses every value outside the four constants (TestVariantKeys).
	if reflect.TypeFor[domain.Variant]().Kind() != reflect.String {
		t.Error("Variant is no longer a string type")
	}
}

// exportedKeyFuncs parses the non-test sources of the package and returns the exported functions
// whose name ends in Key, without ParseOriginalKey and ValidateKey, which take a key.
func exportedKeyFuncs(t *testing.T) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	fset := token.NewFileSet()
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || !fd.Name.IsExported() || !strings.HasSuffix(fd.Name.Name, "Key") {
				continue
			}
			if fd.Name.Name != "ParseOriginalKey" && fd.Name.Name != "ValidateKey" {
				out[fd.Name.Name] = true
			}
		}
	}
	return out
}

func FuzzValidateKey(f *testing.F) {
	id := ids.MustParse(exampleID)
	orig, _ := domain.OriginalKey(id, 1)
	part, _ := domain.UploadPartKey(id, 2)
	for _, s := range []string{orig, part, "", "..", "../x", "a/../b", "/abs", "a//b", "a/", ".tmp/x", "A", "a.b", "a b", `a\b`, "%2e%2e", "a\u202eb", "a\x00b", "\u00e9", "a/b-c/0"} {
		f.Add(s)
	}
	grammar := regexp.MustCompile(`^[a-z0-9-]+(/[a-z0-9-]+)*$`)
	f.Fuzz(func(t *testing.T, key string) {
		err := domain.ValidateKey(key)
		if err != nil {
			if !errors.Is(err, domain.ErrInvalidKey) {
				t.Fatalf("ValidateKey(%q) = %v, not ErrInvalidKey", key, err)
			}
			return
		}
		if !grammar.MatchString(key) || filepath.Clean(key) != key || filepath.IsAbs(key) {
			t.Fatalf("ValidateKey accepted %q outside the grammar", key)
		}
		if strings.Contains(key, "..") {
			t.Fatalf("ValidateKey accepted %q with ..", key)
		}
		for _, seg := range strings.Split(key, "/") {
			if seg == ".." || seg == "." {
				t.Fatalf("ValidateKey accepted %q with a dot segment", key)
			}
		}
	})
}

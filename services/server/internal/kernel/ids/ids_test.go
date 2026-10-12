// SPDX-License-Identifier: Apache-2.0

package ids_test

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// rfcExample is the UUIDv7 example of RFC 9562 appendix A.6, in upper case as clients may send it.
const rfcExample = "017F22E2-79B0-7CC3-98C4-DC0C0C07398F"

// fixedTime is an instant with sub-millisecond digits, so tests see the truncation to milliseconds.
var fixedTime = time.Date(2026, 10, 11, 8, 9, 10, 123456789, time.UTC)

// constReader yields the same byte forever; it drives the counter to known values.
type constReader byte

func (r constReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

// failingReader always fails, like an exhausted entropy source.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("entropy source closed") }

// rejectCases are inputs Parse must refuse; FuzzParse seeds its corpus with them as well.
var rejectCases = []struct{ name, in string }{
	{"version 4", "f47ac10b-58cc-4372-a567-0e02b2c3d479"},
	{"version 1", "c232ab00-9414-11ec-b3c8-9f6bdeced846"},
	{"nil UUID", "00000000-0000-0000-0000-000000000000"},
	{"max UUID", "ffffffff-ffff-ffff-ffff-ffffffffffff"},
	{"wrong variant", "017f22e2-79b0-7cc3-18c4-dc0c0c07398f"},
	{"variant 11 (Microsoft)", "017f22e2-79b0-7cc3-c8c4-dc0c0c07398f"},
	{"variant 111 (reserved)", "017f22e2-79b0-7cc3-f8c4-dc0c0c07398f"},
	{"version 8", "017f22e2-79b0-8cc3-98c4-dc0c0c07398f"},
	{"version 15", "017f22e2-79b0-fcc3-98c4-dc0c0c07398f"},
	{"version 6", "017f22e2-79b0-6cc3-98c4-dc0c0c07398f"},
	{"digit at first hyphen", "017f22e2079b0-7cc3-98c4-dc0c0c07398f"},
	{"underscore at second hyphen", "017f22e2-79b0_7cc3-98c4-dc0c0c07398f"},
	{"colon at third hyphen", "017f22e2-79b0-7cc3:98c4-dc0c0c07398f"},
	{"space at fourth hyphen", "017f22e2-79b0-7cc3-98c4 dc0c0c07398f"},
	{"35 characters", "017f22e2-79b0-7cc3-98c4-dc0c0c07398"},
	{"37 characters", "017f22e2-79b0-7cc3-98c4-dc0c0c07398f0"},
	{"braces", "{017f22e2-79b0-7cc3-98c4-dc0c0c07398f}"},
	{"urn prefix", "urn:uuid:017f22e2-79b0-7cc3-98c4-dc0c0c07398f"},
	{"non-hex", "017f22e2-79b0-7cc3-98c4-dc0c0c07398g"},
	{"empty", ""},
	{"no hyphens", "017f22e279b07cc398c4dc0c0c07398f0000"},
	{"hyphen moved", "017f22e-279b0-7cc3-98c4-dc0c0c07398f"},
	{"hyphen at hex position", "017f22e2-79b0-7cc3-98c4-dc0c0c07398-"},
	{"multi-byte, 36 bytes", "017f22e2-79b0-7cc3-98c4-dc0c0c0739ä"},
	{"plus sign", "+17f22e2-79b0-7cc3-98c4-dc0c0c07398f"},
}

func TestNewHasVersion7AndVariant(t *testing.T) {
	readers := map[string]io.Reader{
		"crypto/rand": rand.Reader,
		"all zero":    constReader(0x00),
		"all ones":    constReader(0xff),
	}
	for name, r := range readers {
		g := ids.NewGenerator(clock.NewFake(fixedTime), r)
		for range 100 {
			id := g.New()
			if v := id[6] >> 4; v != 7 {
				t.Fatalf("%s: version nibble of %s = %d, want 7", name, id, v)
			}
			if v := id[8] >> 6; v != 0b10 {
				t.Fatalf("%s: variant bits of %s = %02b, want 10", name, id, v)
			}
		}
	}
	if id := ids.New(); id[6]>>4 != 7 || id[8]>>6 != 0b10 {
		t.Fatalf("package New() = %s, want version 7 and variant 10", id)
	}
}

func TestNewIsStrictlyMonotonicWithinOneMillisecond(t *testing.T) {
	g := ids.NewGenerator(clock.NewFake(fixedTime), rand.Reader)
	prev := g.New()
	for i := range 10_000 {
		cur := g.New()
		if bytes.Compare(prev[:], cur[:]) >= 0 {
			t.Fatalf("ID %d = %s is not greater than its predecessor %s", i+1, cur, prev)
		}
		prev = cur
	}
}

func TestNewNeverGoesBackwardsWhenClockDoes(t *testing.T) {
	c := clock.NewFake(fixedTime)
	g := ids.NewGenerator(c, rand.Reader)
	first := g.New()

	c.Set(fixedTime.Add(-time.Hour))
	prev := first
	for range 100 {
		cur := g.New()
		if bytes.Compare(prev[:], cur[:]) >= 0 {
			t.Fatalf("after the clock went back, %s is not greater than %s", cur, prev)
		}
		if !cur.Time().Equal(first.Time()) {
			t.Fatalf("after the clock went back, Time() = %v, want the last timestamp %v",
				cur.Time(), first.Time())
		}
		prev = cur
	}

	// Once the clock passes the last timestamp again, IDs follow the clock.
	later := fixedTime.Add(time.Second)
	c.Set(later)
	if got := g.New().Time(); !got.Equal(later.Truncate(time.Millisecond)) {
		t.Fatalf("after the clock caught up, Time() = %v, want %v", got, later.Truncate(time.Millisecond))
	}
}

func TestNewEmbedsClockMilliseconds(t *testing.T) {
	g := ids.NewGenerator(clock.NewFake(fixedTime), rand.Reader)
	id := g.New()

	want := fixedTime.Truncate(time.Millisecond)
	if got := id.Time(); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("Time() = %v, want %v in UTC", got, want)
	}

	// The first 48 bits are the Unix milliseconds, big-endian: 12 hex digits around the first hyphen.
	wantPrefix := fmt.Sprintf("%012x", fixedTime.UnixMilli())
	if s := id.String(); s[:8]+s[9:13] != wantPrefix {
		t.Fatalf("timestamp digits of %s = %s, want %s", s, s[:8]+s[9:13], wantPrefix)
	}

	if got := ids.New().Time(); time.Since(got) < -time.Millisecond || time.Since(got) > time.Minute {
		t.Fatalf("package New().Time() = %v, want close to now", got)
	}
}

// scriptReader hands out exactly the bytes of its script and fails once they are used up, so a
// test fixes every random draw of a generator and sees how many bytes each draw takes.
type scriptReader struct{ script []byte }

func (r *scriptReader) Read(p []byte) (int, error) {
	if len(p) > len(r.script) {
		return 0, fmt.Errorf("script has %d bytes left, read wants %d", len(r.script), len(p))
	}
	n := copy(p, r.script)
	r.script = r.script[n:]
	return n, nil
}

// randA returns the 12 bits of rand_a, the high part of the counter.
func randA(id ids.ID) uint16 { return binary.BigEndian.Uint16(id[6:8]) & (1<<12 - 1) }

// counter returns the 74-bit counter of id: rand_a (12 bits) followed by rand_b (62 bits).
func counter(id ids.ID) *big.Int {
	b := binary.BigEndian.Uint64(id[8:16]) & (1<<62 - 1)
	c := new(big.Int).Lsh(big.NewInt(int64(randA(id))), 62)
	return c.Or(c, new(big.Int).SetUint64(b))
}

func TestNewStepIsTheDrawnValueInRange(t *testing.T) {
	// The seed sets rand_a to 5 and rand_b to 0xff below its maximum, so the third step carries
	// from rand_b into rand_a.
	seed := []byte{0x00, 0x05, 0x3f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00}
	seedCounter := new(big.Int).Lsh(big.NewInt(5), 62)
	seedCounter.Or(seedCounter, new(big.Int).SetUint64(1<<62-1-0xff))
	cases := []struct {
		name  string
		drawn uint32
		want  int64
		carry bool // the step must carry from rand_b into rand_a
	}{
		// A drawn zero becomes 1: a step of 0 would repeat the previous ID.
		{"zero becomes one", 0, 1, false},
		{"one", 1, 1, false},
		{"carry into rand_a", 0x00000200, 0x00000200, true},
		{"middle", 0x12345678, 0x12345678, false},
		{"largest", 0xffffffff, 0xffffffff, false},
	}
	script := append([]byte(nil), seed...)
	for _, tc := range cases {
		script = binary.BigEndian.AppendUint32(script, tc.drawn)
	}
	r := &scriptReader{script: script}
	g := ids.NewGenerator(clock.NewFake(fixedTime), r)

	prev := g.New()
	if got := counter(prev); got.Cmp(seedCounter) != 0 {
		t.Fatalf("counter of the first ID = %#x, want the seed %#x", got, seedCounter)
	}
	for _, tc := range cases {
		cur := g.New()
		if !cur.Time().Equal(prev.Time()) {
			t.Fatalf("%s: Time() = %v, want the same millisecond %v", tc.name, cur.Time(), prev.Time())
		}
		if carried := randA(cur) != randA(prev); carried != tc.carry {
			t.Errorf("%s: rand_a changed = %v, want %v", tc.name, carried, tc.carry)
		}
		step := new(big.Int).Sub(counter(cur), counter(prev))
		if step.Cmp(big.NewInt(tc.want)) != 0 {
			t.Errorf("%s: drawn %#x gave a step of %s, want %d", tc.name, tc.drawn, step, tc.want)
		}
		prev = cur
	}
	if len(r.script) != 0 {
		t.Errorf("%d random bytes left over, want every draw to take its bytes", len(r.script))
	}
}

func TestNewCounterOverflowAdvancesTimestamp(t *testing.T) {
	// All-ones random bits fill the 74-bit counter, so the next increment overflows.
	g := ids.NewGenerator(clock.NewFake(fixedTime), constReader(0xff))
	first := g.New()
	second := g.New()

	if want := first.Time().Add(time.Millisecond); !second.Time().Equal(want) {
		t.Fatalf("after overflow Time() = %v, want %v", second.Time(), want)
	}
	if bytes.Compare(first[:], second[:]) >= 0 {
		t.Fatalf("after overflow %s is not greater than %s", second, first)
	}
}

func TestNewConcurrentUnique(t *testing.T) {
	const goroutines, perGoroutine = 64, 1_000
	results := make([][]ids.ID, goroutines)
	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Go(func() {
			own := make([]ids.ID, perGoroutine)
			for j := range own {
				own[j] = ids.New()
			}
			results[i] = own
		})
	}
	wg.Wait()

	seen := make(map[ids.ID]struct{}, goroutines*perGoroutine)
	for _, own := range results {
		for j, id := range own {
			if _, dup := seen[id]; dup {
				t.Fatalf("duplicate ID %s", id)
			}
			seen[id] = struct{}{}
			if j > 0 && bytes.Compare(own[j-1][:], id[:]) >= 0 {
				t.Fatalf("IDs of one goroutine not increasing: %s then %s", own[j-1], id)
			}
		}
	}
	if len(seen) != goroutines*perGoroutine {
		t.Fatalf("got %d distinct IDs, want %d", len(seen), goroutines*perGoroutine)
	}
}

func TestParseRFC9562Example(t *testing.T) {
	id, err := ids.Parse(rfcExample)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", rfcExample, err)
	}
	if got, want := id.String(), strings.ToLower(rfcExample); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	want := time.Date(2022, 2, 22, 19, 22, 22, 0, time.UTC)
	if got := id.Time(); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("Time() = %v, want %v", got, want)
	}
	if id.IsZero() {
		t.Fatal("IsZero() = true for a parsed ID")
	}
}

func TestParseRejects(t *testing.T) {
	for _, tc := range rejectCases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := ids.Parse(tc.in)
			if err == nil {
				t.Fatalf("Parse(%q) = %s, want an error", tc.in, id)
			}
			if !errors.Is(err, ids.ErrInvalid) {
				t.Fatalf("Parse(%q) error %v does not wrap ErrInvalid", tc.in, err)
			}
			if !id.IsZero() {
				t.Fatalf("Parse(%q) returned %s with the error, want the zero ID", tc.in, id)
			}
			if tc.in != "" && strings.Contains(err.Error(), tc.in) {
				t.Fatalf("error %q echoes the input", err)
			}
		})
	}
}

func TestParseReportsPositionOfDefect(t *testing.T) {
	// Positions are 0-based string indexes, for hyphens and hexadecimal digits alike; every index
	// is checked because Parse reads the digits in pairs.
	valid := strings.ToLower(rfcExample)
	for i := range len(valid) {
		bad := []byte(valid)
		want := fmt.Sprintf("non-hexadecimal character at position %d", i)
		if valid[i] == '-' {
			bad[i] = '0'
			want = fmt.Sprintf("want '-' at position %d", i)
		} else {
			bad[i] = 'g'
		}
		_, err := ids.Parse(string(bad))
		if err == nil || !strings.HasSuffix(err.Error(), want) {
			t.Errorf("Parse with a defect at index %d: error %v, want it to end with %q", i, err, want)
		}
	}
}

func TestTextRoundTrip(t *testing.T) {
	id := ids.New()
	text, err := id.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error = %v", err)
	}
	if string(text) != id.String() {
		t.Fatalf("MarshalText() = %q, want String() %q", text, id.String())
	}

	var back ids.ID
	if err := back.UnmarshalText(text); err != nil {
		t.Fatalf("UnmarshalText(%q) error = %v", text, err)
	}
	if back != id {
		t.Fatalf("round trip gave %s, want %s", back, id)
	}

	var upper ids.ID
	if err := upper.UnmarshalText([]byte(strings.ToUpper(id.String()))); err != nil || upper != id {
		t.Fatalf("UnmarshalText(upper case) = %s, %v; want %s, nil", upper, err, id)
	}

	keep := id
	if err := keep.UnmarshalText([]byte("not-an-id")); !errors.Is(err, ids.ErrInvalid) {
		t.Fatalf("UnmarshalText(invalid) error = %v, want ErrInvalid", err)
	}
	if keep != id {
		t.Fatalf("failed UnmarshalText changed the receiver to %s", keep)
	}

	// A text-based encoder such as fmt uses String.
	if got := fmt.Sprint(id); got != id.String() {
		t.Fatalf("fmt.Sprint(id) = %q, want %q", got, id.String())
	}
}

func TestIsZero(t *testing.T) {
	var zero ids.ID
	if !zero.IsZero() {
		t.Fatal("zero ID: IsZero() = false")
	}
	if ids.New().IsZero() {
		t.Fatal("New(): IsZero() = true")
	}
}

func TestMustParse(t *testing.T) {
	if got, want := ids.MustParse(rfcExample).String(), strings.ToLower(rfcExample); got != want {
		t.Fatalf("MustParse(%q) = %s, want %s", rfcExample, got, want)
	}
	msg := panicMessage(t, func() { ids.MustParse("f47ac10b-58cc-4372-a567-0e02b2c3d479") })
	if !strings.Contains(msg, "ids") {
		t.Fatalf("MustParse panic %q does not name the package", msg)
	}
}

func TestNewGeneratorPanicsOnNilArguments(t *testing.T) {
	cases := map[string]func(){
		"nil clock":  func() { ids.NewGenerator(nil, rand.Reader) },
		"nil reader": func() { ids.NewGenerator(clock.NewFake(fixedTime), nil) },
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			if msg := panicMessage(t, fn); !strings.HasPrefix(msg, "ids: ") {
				t.Fatalf("panic %q does not start with the package name", msg)
			}
		})
	}
}

func TestNewPanicsOnFailingRandomReader(t *testing.T) {
	g := ids.NewGenerator(clock.NewFake(fixedTime), failingReader{})
	msg := panicMessage(t, func() { g.New() })
	if !strings.HasPrefix(msg, "ids: ") || !strings.Contains(msg, "entropy source closed") {
		t.Fatalf("panic %q, want a message starting with \"ids: \" that names the cause", msg)
	}
	// The mutex must be released after the panic, or the next call would deadlock.
	msg = panicMessage(t, func() { g.New() })
	if !strings.HasPrefix(msg, "ids: ") {
		t.Fatalf("second call panic %q, want the same failure", msg)
	}
}

// budgetReader yields 0xff bytes for the next ok reads and fails after that, so a test can make
// exactly one draw of the generator fail and then let the source recover.
type budgetReader struct{ ok int }

func (r *budgetReader) Read(p []byte) (int, error) {
	if r.ok == 0 {
		return 0, errors.New("entropy source closed")
	}
	r.ok--
	for i := range p {
		p[i] = 0xff
	}
	return len(p), nil
}

func TestNewKeepsStateWhenRandomReaderPanics(t *testing.T) {
	// A caller that recovers from the panic must find the generator as it was before the failed
	// call, so the next ID is the one an undisturbed generator would have returned.
	t.Run("new millisecond", func(t *testing.T) {
		r := &budgetReader{}
		g := ids.NewGenerator(clock.NewFake(fixedTime), r)
		panicMessage(t, func() { g.New() })

		r.ok = 1 << 30
		want := ids.NewGenerator(clock.NewFake(fixedTime), constReader(0xff)).New()
		if got := g.New(); got != want {
			t.Fatalf("after recovered panic New() = %s, want %s (fresh counter of this millisecond)", got, want)
		}
	})

	t.Run("counter overflow", func(t *testing.T) {
		r := &budgetReader{ok: 2} // the first seed and the step succeed, the reseed fails
		g := ids.NewGenerator(clock.NewFake(fixedTime), r)
		first := g.New()
		panicMessage(t, func() { g.New() })

		r.ok = 1 << 30
		if got, want := g.New().Time(), first.Time().Add(time.Millisecond); !got.Equal(want) {
			t.Fatalf("after recovered panic Time() = %v, want %v (one overflow, not two)", got, want)
		}
	})
}

func TestNewPanicsOnClockOutsideTimestampRange(t *testing.T) {
	cases := map[string]time.Time{
		"before 1970":       time.Date(1969, 12, 31, 23, 59, 59, 0, time.UTC),
		"beyond 48 bits ms": time.UnixMilli(1 << 48).UTC(),
	}
	for name, at := range cases {
		t.Run(name, func(t *testing.T) {
			g := ids.NewGenerator(clock.NewFake(at), rand.Reader)
			if msg := panicMessage(t, func() { g.New() }); !strings.HasPrefix(msg, "ids: ") {
				t.Fatalf("panic %q does not start with the package name", msg)
			}
		})
	}

	// The last representable millisecond still works.
	last := time.UnixMilli(1<<48 - 1).UTC()
	if got := ids.NewGenerator(clock.NewFake(last), rand.Reader).New().Time(); !got.Equal(last) {
		t.Fatalf("Time() = %v, want %v", got, last)
	}
}

func TestNewPanicsWhenOverflowPassesTimestampRange(t *testing.T) {
	last := time.UnixMilli(1<<48 - 1).UTC()
	g := ids.NewGenerator(clock.NewFake(last), constReader(0xff))
	g.New()
	if msg := panicMessage(t, func() { g.New() }); !strings.HasPrefix(msg, "ids: ") {
		t.Fatalf("panic %q does not start with the package name", msg)
	}
}

// panicMessage runs fn, fails the test if it does not panic, and returns the panic value as text.
func panicMessage(t *testing.T, fn func()) (msg string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("no panic")
		}
		msg = fmt.Sprint(r)
	}()
	fn()
	return ""
}

func FuzzParse(f *testing.F) {
	f.Add(rfcExample)
	f.Add(strings.ToLower(rfcExample))
	for _, tc := range rejectCases {
		f.Add(tc.in)
	}
	f.Fuzz(func(t *testing.T, in string) {
		id, err := ids.Parse(in)
		if err != nil {
			if !errors.Is(err, ids.ErrInvalid) {
				t.Fatalf("Parse(%q) error %v does not wrap ErrInvalid", in, err)
			}
			return
		}
		if got := id.String(); got != strings.ToLower(in) {
			t.Fatalf("Parse(%q).String() = %q, want %q", in, got, strings.ToLower(in))
		}
	})
}

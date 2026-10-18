// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/migrations"
)

// createTable finds the table names of CREATE TABLE in the migrations.
var createTable = regexp.MustCompile(`(?i)\bCREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"?public"?\.)?"?([a-z_][a-z0-9_]*)"?`)

// tableNames returns the tables the embedded migrations create: the tables of the module register
// (register-check holds both equal), the closed set of db.collection.name.
var tableNames = sync.OnceValue(func() []string {
	var out []string
	_ = fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return err
		}
		b, err := fs.ReadFile(migrations.FS, path)
		if err != nil {
			return err
		}
		for _, m := range createTable.FindAllStringSubmatch(string(b), -1) {
			out = append(out, strings.ToLower(m[1]))
		}
		return nil
	})
	slices.Sort(out)
	return slices.Compact(out)
})

// knownOperations is the closed set of db.operation.name.
var knownOperations = sync.OnceValue(func() map[string]bool {
	out := map[string]bool{}
	for _, v := range metrics.DBClientOperationDuration.Attrs[0].Values {
		out[v] = true
	}
	return out
})

// knownTables is the closed set of db.collection.name.
var knownTables = sync.OnceValue(func() map[string]bool {
	out := map[string]bool{}
	for _, v := range tableNames() {
		out[v] = true
	}
	return out
})

// closedValues maps op and table to their closed sets: statements outside them (DDL, the
// system catalogues, River's tables) are expected and count as other, not as a fault.
func closedValues(op, table string) (string, string) {
	if !knownOperations()[op] {
		op = metrics.Other
	}
	if table != "" && !knownTables()[table] {
		table = metrics.Other
	}
	return op, table
}

// queryMetric is db.client.operation.duration of the pool connections (signals.md 4.5). otelpgx
// records a metric of the same name with the raw first word of a statement and the attribute
// pgx.operation.type; the view of the metric registry drops every instrument of otelpgx, so the
// server records its own with closed attributes (docs/DECISIONS.md M1-68).
type queryMetric struct {
	h *metrics.Histogram
}

// newQueryMetric creates the histogram on mp; an error leaves the metric out (nil), because a pool
// must open without telemetry (17 rule 4) and the error handler of platform/otel reports it.
func newQueryMetric(mp metric.MeterProvider) *queryMetric {
	h, err := metrics.NewHistogram(mp, metrics.DBClientOperationDuration, metrics.Values{metrics.SourceTables: tableNames()})
	if err != nil {
		return nil
	}
	return &queryMetric{h: h}
}

// queryStartKey is the context key of a running query.
type queryStartKey struct{}

// queryStart is what TraceQueryStart keeps for TraceQueryEnd.
type queryStart struct {
	at        time.Time
	op, table string
}

// start returns ctx with the start of the query sql.
func (m *queryMetric) start(ctx context.Context, sql string) context.Context {
	if m == nil {
		return ctx
	}
	op, table := closedValues(operationOf(sql))
	return context.WithValue(ctx, queryStartKey{}, queryStart{at: time.Now(), op: op, table: table})
}

// end records the query started in ctx, error or not.
func (m *queryMetric) end(ctx context.Context) {
	if m == nil {
		return
	}
	if s, ok := ctx.Value(queryStartKey{}).(queryStart); ok {
		m.h.RecordDuration(context.WithoutCancel(ctx), time.Since(s.at), s.op, s.table)
	}
}

// operationOf returns the operation (the first keyword in upper case; for WITH the keyword of the
// main statement after the common table expressions) and the table of sql (lower case, without
// schema and quotes): after INTO for INSERT and MERGE, after UPDATE, after FROM for DELETE and
// SELECT (the first FROM outside parentheses), after COPY. Leading comments are skipped and string
// literals ignored. A statement without such a table has table "". The values only feed the closed
// sets of the metric, which map anything else to other.
func operationOf(sql string) (op, table string) {
	toks := tokens(sql)
	if len(toks) == 0 {
		return "", ""
	}
	i := 0
	op = strings.ToUpper(toks[0].text)
	if op == "WITH" {
		i = mainStatement(toks)
		if i < 0 {
			return op, ""
		}
		op = strings.ToUpper(toks[i].text)
	}
	return op, tableAfter(toks, i, op)
}

// token is a word or punctuation of a statement with its parenthesis depth.
type token struct {
	text  string
	depth int
	ident bool
}

// tokens splits sql into words (identifiers, keywords, quoted identifiers as identifiers) and
// single punctuation characters, skipping comments and string literals.
func tokens(sql string) []token {
	var out []token
	depth := 0
	r := []rune(sql)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '-' && i+1 < len(r) && r[i+1] == '-':
			i = skipLine(r, i)
		case c == '/' && i+1 < len(r) && r[i+1] == '*':
			if i = skipBlock(r, i); i >= len(r) {
				return out
			}
		case c == '\'':
			i = skipLiteral(r, i)
		case c == '"':
			j := closingQuote(r, i)
			out = append(out, token{text: string(r[i+1 : j]), depth: depth, ident: true})
			i = j
		case c == '(':
			out = append(out, token{text: "(", depth: depth})
			depth++
		case c == ')':
			depth--
			out = append(out, token{text: ")", depth: depth})
		case unicode.IsLetter(c) || c == '_':
			j := wordEnd(r, i)
			out = append(out, token{text: string(r[i:j]), depth: depth, ident: true})
			i = j - 1
		case !unicode.IsSpace(c):
			out = append(out, token{text: string(c), depth: depth})
		}
	}
	return out
}

// skipLine returns the index of the end of the line comment at i.
func skipLine(r []rune, i int) int {
	for i < len(r) && r[i] != '\n' {
		i++
	}
	return i
}

// skipBlock returns the index of the closing slash of the block comment at i, len(r) when it is
// not closed.
func skipBlock(r []rune, i int) int {
	j := i + 2
	for j+1 < len(r) && (r[j] != '*' || r[j+1] != '/') {
		j++
	}
	if j+1 >= len(r) {
		return len(r)
	}
	return j + 1
}

// skipLiteral returns the index of the closing quote of the string literal at i; two quotes in a
// row stand for one quote inside the literal.
func skipLiteral(r []rune, i int) int {
	j := i + 1
	for j < len(r) {
		if r[j] != '\'' {
			j++
			continue
		}
		if j+1 < len(r) && r[j+1] == '\'' {
			j += 2
			continue
		}
		break
	}
	return j
}

// closingQuote returns the index of the double quote that closes the identifier at i, len(r) when
// there is none.
func closingQuote(r []rune, i int) int {
	j := i + 1
	for j < len(r) && r[j] != '"' {
		j++
	}
	return j
}

// wordEnd returns the index after the identifier that starts at i.
func wordEnd(r []rune, i int) int {
	j := i
	for j < len(r) && (unicode.IsLetter(r[j]) || unicode.IsDigit(r[j]) || r[j] == '_' || r[j] == '$') {
		j++
	}
	return j
}

// mainStatements are the keywords that start the main statement after the CTEs of WITH.
var mainStatements = map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true, "MERGE": true}

// mainStatement returns the index of the first statement keyword outside parentheses after WITH,
// -1 when there is none.
func mainStatement(toks []token) int {
	for i := 1; i < len(toks); i++ {
		if toks[i].depth == 0 && toks[i].ident && mainStatements[strings.ToUpper(toks[i].text)] {
			return i
		}
	}
	return -1
}

// tableAfter returns the table op at index i names, "" when none.
func tableAfter(toks []token, i int, op string) string {
	depth := toks[i].depth
	var marker string
	switch op {
	case "INSERT", "MERGE":
		marker = "INTO"
	case "DELETE", "SELECT":
		marker = "FROM"
	case "UPDATE", "COPY":
		return qualifiedName(toks, i+1, "ONLY")
	default:
		return ""
	}
	for j := i + 1; j < len(toks); j++ {
		if toks[j].depth == depth && toks[j].ident && strings.EqualFold(toks[j].text, marker) {
			return qualifiedName(toks, j+1, "ONLY")
		}
	}
	return ""
}

// qualifiedName returns the last part of the (schema-qualified) name at index i in lower case,
// skipping the keyword skip; "" when no name follows.
func qualifiedName(toks []token, i int, skip string) string {
	if i < len(toks) && strings.EqualFold(toks[i].text, skip) {
		i++
	}
	if i >= len(toks) || !toks[i].ident {
		return ""
	}
	name := toks[i].text
	for i+2 < len(toks) && toks[i+1].text == "." && toks[i+2].ident {
		name = toks[i+2].text
		i += 2
	}
	return strings.ToLower(name)
}

// poolStates are the values of bothy.pool.state.
const (
	poolIdle     = "idle"
	poolAcquired = "acquired"
	poolMax      = "max"
)

// RegisterMetrics registers bothy.db.pool.connections of p on mp (nil means the global provider
// of platform/otel): an observable gauge that reads the pool statistics at every collection, a
// cheap read without a database round trip. internal/wire registers the process pool once after
// Open; the registration ends with Unregister. Errors: those of the meter provider.
func (p *Pool) RegisterMetrics(mp metric.MeterProvider) (metric.Registration, error) {
	g, err := metrics.NewObservableGauge(mp, metrics.DBPoolConnections, nil)
	if err != nil {
		return nil, fmt.Errorf("pg: %w", err)
	}
	return g.Register(func(_ context.Context, o metric.Observer) error {
		st := p.pool.Stat()
		g.Observe(o, float64(st.IdleConns()), poolIdle)
		g.Observe(o, float64(st.AcquiredConns()), poolAcquired)
		g.Observe(o, float64(st.MaxConns()), poolMax)
		return nil
	})
}

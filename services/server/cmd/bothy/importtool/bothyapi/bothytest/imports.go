// SPDX-License-Identifier: Apache-2.0

package bothytest

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The import interface of the fake (M8-04 design note 11): an in-memory model of the routes the
// import tool calls, with the answers, problem codes and reasons of the server's importer
// (internal/importer/app, M8-03 and M8-11). It is only for unit tests of the tool; the guarantees
// that need the real server run in test/importtool.

// The item kinds, states and reasons of the import interface (docs/api/openapi.yaml, tag Import).
const (
	kindSpace    = "space"
	kindChannel  = "channel"
	kindForumTag = "forum_tag"
	kindUser     = "user"
	kindAgent    = "agent"
	kindThread   = "thread"
	kindMessage  = "message"
	kindPin      = "pin"
	kindFile     = "file"

	itemPending = "pending"
	itemPosted  = "posted"
	itemSkipped = "skipped"

	sourceRunning = "running"

	reasonNotRunning     = "import_not_running"
	reasonOutside        = "target_outside_source"
	reasonThreadMissing  = "thread_not_imported"
	reasonAuthorMissing  = "author_not_imported"
	reasonPartConflict   = "part_conflict"
	reasonIncomplete     = "upload_incomplete"
	codeConflict         = "conflict"
	codeValidation       = "validation_failed"
	codeNotFound         = "not_found"
	codeInvalid          = "invalid"
	codeNotImported      = "not_imported"
	codeDigestMismatch   = "part_digest_mismatch"
	targetSkip           = "skip"
	memberName           = "name"
	memberState          = "state"
	memberCursor         = "cursor"
	pointerContentDigest = "#header/Content-Digest"

	// Member names and path segments that occur in several answers.
	memberDisplayName = "display_name"
	memberVersion     = "version"
	memberStatus      = "status"
	memberCreatedAt   = "created_at"
	memberItems       = "items"
	memberKind        = "kind"
	memberHandle      = "handle"
	memberPartNumber  = "part_number"
	segImports        = "imports"
	segFiles          = "files"
	segChannels       = "channels"
	memberMode        = "mode"
	memberSortKey     = "sort_key"
	memberCaps        = "capabilities"
)

// Limits of the fake that the contract fixes, not a configuration of the fake.
const (
	// batchMaxItems is the maxItems of ImportBatch.items in docs/api/openapi.yaml.
	batchMaxItems = 100
	// pageSizeDefault and pageSizeMax stand for api.page_size_default and api.page_size_max.
	pageSizeDefault = 50
	pageSizeMax     = 200
	// defaultPartSize is the part size of a new fake; SetPartSize changes it.
	defaultPartSize = 5 << 20
	// sessionTTL is the expires_at of an upload session; the fake never expires one.
	sessionTTL = time.Hour
)

// Item is one import item of a source, in insertion order.
type Item struct {
	// OriginKey and ItemKind name the item; State is pending, posted or skipped.
	OriginKey, ItemKind, State string
	// TargetID is the UUID of the target, "" for a skipped item.
	TargetID string
	// SHA is the stored content_sha256, "" without one.
	SHA string
}

// Batch is one accepted applyImportBatch call.
type Batch struct {
	// ThreadOriginKey is the thread_origin_key, "" for a structure batch.
	ThreadOriginKey string
	// Keys are the origin keys of the items in request order.
	Keys []string
}

// File is the file of one file item.
type File struct {
	// OriginKey is file:<attachment_id>, FileID the reserved UUID, Name the declared file name.
	OriginKey, FileID, Name string
	// Size and SHA256 are the declared size and hex SHA-256.
	Size   int64
	SHA256 string
	// Posted reports a completed upload; Bytes are the uploaded bytes then.
	Posted bool
	Bytes  []byte
}

// imports is the state of the import interface, guarded by Server.mu.
type imports struct {
	me          person
	users       []*person
	agents      []*person
	spaces      []*space
	sources     []*source
	partSize    int64
	afterStore  []*afterFault
	batchFaults map[string]*batchFault
}

// batchFault refuses the next times thread batches of one thread with status and reason.
type batchFault struct {
	status int
	reason string
	times  int
}

// person is a user or an agent of the directory.
type person struct {
	id, handle, displayName string
	active, owner           bool
	created                 time.Time
}

// space is a space with its channels.
type space struct {
	id, name, sortKey string
	created           time.Time
	channels          []*channel
}

// channel is a channel of a space; mainThread, operator and threads are the read model of chat.go.
type channel struct {
	id, name, kind, sortKey, topic string
	readOnly                       bool
	created                        time.Time
	// mainThread is the main thread of a text channel, created on first use; operator the agent
	// UUID of mode agent, "" for mode none.
	mainThread *chatThread
	operator   string
	threads    []*chatThread
}

// source is an import source with its items.
type source struct {
	id, state string
	config    map[string]any
	cursor    map[string]any
	stats     map[string]any
	created   time.Time
	items     []*item
	byKey     map[string]*item
	batches   []Batch
	threads   map[string][]string
	pins      []string
	files     []*file
	// earlier are the earlier sessions written per thread key (M8-06); late marks threads that
	// have had a current session, so their earlier sessions are dropped_late.
	earlier map[string][]EarlierSession
	late    map[string]bool
}

// EarlierSession is the member earlier_session of a thread item as the fake stored it (M8-06):
// the agent UUID, the reference, the baseline (0 when absent) and the currency ("" when absent).
type EarlierSession struct {
	AgentID, Ref, Currency string
	Baseline               int64
}

// item is one stored import item.
type item struct {
	id, key, kind, state, targetKind, targetID, sha string
	created                                         time.Time
	data                                            map[string]any
}

// file is the file of a file item with its upload session.
type file struct {
	File
	partCount int
	partSize  int64
	parts     map[int]filePart
}

// filePart is one uploaded part.
type filePart struct {
	digest, etag string
	data         []byte
}

// afterFault answers the next times matching requests with status after they were processed.
type afterFault struct {
	method, prefix string
	status, times  int
}

// apiError is a problem answer of a handler.
type apiError struct {
	status       int
	code, reason string
	fields       []fieldError
}

// fieldError is one entry of Problem.errors.
type fieldError struct {
	Pointer string `json:"pointer"`
	Code    string `json:"code"`
}

// Error returns the code, so an apiError is an error.
func (e *apiError) Error() string { return e.code }

// conflict is 409 conflict with reason.
func conflict(reason string) *apiError {
	return &apiError{status: http.StatusConflict, code: codeConflict, reason: reason}
}

// invalid is 400 validation_failed with fields.
func invalid(fields ...fieldError) *apiError {
	return &apiError{status: http.StatusBadRequest, code: codeValidation, fields: fields}
}

// notFound is 404 not_found.
func notFound() *apiError { return &apiError{status: http.StatusNotFound, code: codeNotFound} }

// newImports returns the empty state: the person of the session is an active owner.
func newImports(now time.Time) *imports {
	return &imports{me: person{id: UserID, handle: "alex", displayName: "Alex", active: true, owner: true, created: now},
		partSize: defaultPartSize}
}

// newID returns a new UUIDv7 string.
func newID() string { return ids.New().String() }

// AddUser adds a person with handle and display name (status active or disabled) and returns its
// UUID. listUsers answers the person of the session first, then the added ones in order.
func (s *Server) AddUser(handle, displayName string, active bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := &person{id: newID(), handle: handle, displayName: displayName, active: active, created: s.clk.Now()}
	s.imp.users = append(s.imp.users, p)
	return p.id
}

// AddAgent adds an agent (status active or disabled) and returns its UUID.
func (s *Server) AddAgent(handle, displayName string, active bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := &person{id: newID(), handle: handle, displayName: displayName, active: active, created: s.clk.Now()}
	s.imp.agents = append(s.imp.agents, p)
	return p.id
}

// SetMe changes the person of the session: is_owner of getMe and its status in listUsers.
func (s *Server) SetMe(isOwner, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.imp.me.owner, s.imp.me.active = isOwner, active
}

// AddSpace adds a space and returns its UUID.
func (s *Server) AddSpace(name string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	sp := &space{id: newID(), name: name, sortKey: "a" + strconv.Itoa(len(s.imp.spaces)), created: s.clk.Now()}
	s.imp.spaces = append(s.imp.spaces, sp)
	return sp.id
}

// AddChannel adds a text channel to the space and returns its UUID; "" for an unknown space.
func (s *Server) AddChannel(spaceID, name string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	sp := s.imp.space(spaceID)
	if sp == nil {
		return ""
	}
	ch := &channel{id: newID(), name: name, kind: "text", sortKey: "a" + strconv.Itoa(len(sp.channels)), created: s.clk.Now()}
	sp.channels = append(sp.channels, ch)
	return ch.id
}

// AddSource adds a source of kind discord named "Example import" in state (draft, running,
// paused, completed or failed) with config as its stored mapping and returns its UUID. Pass the
// normalized JSON form the server stores (domain.Mapping.JSON): the fake stores it as given.
func (s *Server) AddSource(state string, config map[string]any) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := &source{id: newID(), state: state, config: copyObject(config), cursor: map[string]any{}, stats: map[string]any{},
		created: s.clk.Now(), byKey: map[string]*item{}, threads: map[string][]string{}, earlier: map[string][]EarlierSession{},
		late: map[string]bool{}}
	s.imp.sources = append(s.imp.sources, src)
	return src.id
}

// SetConfig replaces the stored mapping of the source, as the web console does when the owner
// changes a lead.
func (s *Server) SetConfig(id string, config map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if src := s.imp.source(id); src != nil {
		src.config = copyObject(config)
	}
}

// SetCursor replaces the stored cursor of the source (a lease of another run, for example).
func (s *Server) SetCursor(id string, cursor map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if src := s.imp.source(id); src != nil {
		src.cursor = copyObject(cursor)
	}
}

// FailBatch refuses the next times thread batches with thread_origin_key threadKey with status
// (409 conflict with reason, or another 4xx) before anything is stored; other threads are served.
func (s *Server) FailBatch(threadKey string, status int, reason string, times int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.imp.batchFaults == nil {
		s.imp.batchFaults = map[string]*batchFault{}
	}
	s.imp.batchFaults[threadKey] = &batchFault{status: status, reason: reason, times: times}
}

// SetState changes the state of the source, as the web console would.
func (s *Server) SetState(id, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if src := s.imp.source(id); src != nil {
		src.state = state
	}
}

// SourceCursor returns a copy of the cursor of the source; nil for an unknown source.
func (s *Server) SourceCursor(id string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if src := s.imp.source(id); src != nil {
		return copyObject(src.cursor)
	}
	return nil
}

// SourceStats returns a copy of the stats of the source with the members server and tool.
func (s *Server) SourceStats(id string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if src := s.imp.source(id); src != nil {
		return copyObject(src.stats)
	}
	return nil
}

// Items returns the items of the source in insertion order.
func (s *Server) Items(sourceID string) []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.imp.source(sourceID)
	if src == nil {
		return nil
	}
	out := make([]Item, 0, len(src.items))
	for _, it := range src.items {
		out = append(out, Item{OriginKey: it.key, ItemKind: it.kind, State: it.state, TargetID: it.targetID, SHA: it.sha})
	}
	return out
}

// ThreadMessages returns the origin keys of the messages appended to the thread of
// threadOriginKey (thread:<id>, or channel:<id> for a main thread) in append order.
func (s *Server) ThreadMessages(sourceID, threadOriginKey string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if src := s.imp.source(sourceID); src != nil {
		return append([]string(nil), src.threads[threadOriginKey]...)
	}
	return nil
}

// Pins returns the origin keys of the pinned messages in pin order.
func (s *Server) Pins(sourceID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if src := s.imp.source(sourceID); src != nil {
		return append([]string(nil), src.pins...)
	}
	return nil
}

// MessageData returns a copy of the data of the posted message item messageKey; nil without one.
func (s *Server) MessageData(sourceID, messageKey string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if src := s.imp.source(sourceID); src != nil {
		if it := src.byKey[messageKey]; it != nil && it.kind == kindMessage {
			return copyObject(it.data)
		}
	}
	return nil
}

// EarlierSessions returns the earlier sessions the source stored for the thread key, in order.
func (s *Server) EarlierSessions(sourceID, threadKey string) []EarlierSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	if src := s.imp.source(sourceID); src != nil {
		return append([]EarlierSession(nil), src.earlier[threadKey]...)
	}
	return nil
}

// SetSessionLate marks the thread key as having had a current session since the import: its
// earlier sessions answer dropped_late and are not stored (22 9 step 6).
func (s *Server) SetSessionLate(sourceID, threadKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if src := s.imp.source(sourceID); src != nil {
		src.late[threadKey] = true
	}
}

// Batches returns the accepted batches of the source in order.
func (s *Server) Batches(sourceID string) []Batch {
	s.mu.Lock()
	defer s.mu.Unlock()
	if src := s.imp.source(sourceID); src != nil {
		return append([]Batch(nil), src.batches...)
	}
	return nil
}

// Files returns the files of the file items of the source in creation order.
func (s *Server) Files(sourceID string) []File {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.imp.source(sourceID)
	if src == nil {
		return nil
	}
	out := make([]File, 0, len(src.files))
	for _, f := range src.files {
		c := f.File
		c.Bytes = append([]byte(nil), f.Bytes...)
		out = append(out, c)
	}
	return out
}

// SetPartSize sets the part size of new upload sessions in bytes (positive).
func (s *Server) SetPartSize(n int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.imp.partSize = n
}

// FailAfterStore makes the next times requests with method whose path starts with pathPrefix be
// processed and stored normally and then answered with status (the "stored, then 502" case of
// 22 16). Only answers below 300 count: a refused request keeps its own answer.
func (s *Server) FailAfterStore(method, pathPrefix string, status, times int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.imp.afterStore = append(s.imp.afterStore, &afterFault{method: method, prefix: pathPrefix, status: status, times: times})
}

// Methods returns the number of requests per HTTP method over all requests.
func (s *Server) Methods() map[string]int {
	out := map[string]int{}
	for _, r := range s.Requests() {
		out[r.Method]++
	}
	return out
}

// Writes returns the number of requests that are not GET, without the POST of refreshToken.
func (s *Server) Writes() int {
	n := 0
	for _, r := range s.Requests() {
		if r.Method != http.MethodGet && (r.Method != http.MethodPost || r.Path != PathRefresh) {
			n++
		}
	}
	return n
}

// space returns the space id, nil when unknown.
func (im *imports) space(id string) *space {
	for _, sp := range im.spaces {
		if sp.id == id {
			return sp
		}
	}
	return nil
}

// source returns the source id, nil when unknown.
func (im *imports) source(id string) *source {
	for _, src := range im.sources {
		if src.id == id {
			return src
		}
	}
	return nil
}

// byHandle returns the active or inactive person of list with handle, nil when unknown.
func byHandle(list []*person, handle string) *person {
	for _, p := range list {
		if p.handle == handle {
			return p
		}
	}
	return nil
}

// copyObject returns a deep copy of m through JSON, so numbers are float64 as on the wire.
func copyObject(m map[string]any) map[string]any {
	out := map[string]any{}
	b, err := json.Marshal(m)
	if err == nil {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

// afterStoreFault returns the after-store fault that matches r, nil without one.
func (s *Server) afterStoreFault(r *http.Request) *afterFault {
	for _, f := range s.imp.afterStore {
		if f.times > 0 && f.method == r.Method && strings.HasPrefix(r.URL.Path, f.prefix) {
			return f
		}
	}
	return nil
}

// serveAfterStore processes r into a recorder; a 2xx answer is replaced by the fault status,
// any other answer is passed through.
func (s *Server) serveAfterStore(w http.ResponseWriter, r *http.Request, body []byte, f *afterFault) {
	rec := httptest.NewRecorder()
	s.route(rec, r, body)
	if rec.Code < http.StatusMultipleChoices {
		f.times--
		s.problem(w, r, f.status, "fault", body)
		return
	}
	for k, v := range rec.Header() {
		w.Header()[k] = v
	}
	// Every answer of the fake is a JSON or problem document; the type is set from constants so
	// a passed-through body is never served as anything else.
	if rec.Header().Get("Content-Type") == "application/problem+json" {
		w.Header().Set("Content-Type", "application/problem+json")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(rec.Code)
	_, _ = w.Write(rec.Body.Bytes())
}

// importRoute is one route of the import interface below APIPrefix; "*" matches one segment.
type importRoute struct {
	method  string
	pattern []string
	handle  func(s *Server, r *http.Request, body []byte, params []string) (int, any, *apiError)
}

// importRoutes are the routes of the import interface.
var importRoutes = []importRoute{
	{http.MethodGet, []string{"users"}, (*Server).listUsers},
	{http.MethodGet, []string{"agents"}, (*Server).listAgents},
	{http.MethodGet, []string{"spaces"}, (*Server).listSpaces},
	{http.MethodGet, []string{"spaces", "*", segChannels}, (*Server).listSpaceChannels},
	{http.MethodGet, []string{segImports}, (*Server).listImports},
	{http.MethodGet, []string{segImports, "*"}, (*Server).getImport},
	{http.MethodGet, []string{segImports, "*", memberItems}, (*Server).listImportItems},
	{http.MethodPost, []string{segImports, "*", "batches"}, (*Server).applyBatch},
	{http.MethodPost, []string{segImports, "*", segFiles}, (*Server).createFile},
	{http.MethodPut, []string{segImports, "*", segFiles, "*", "parts", "*"}, (*Server).uploadPart},
	{http.MethodPost, []string{segImports, "*", segFiles, "*", "complete"}, (*Server).completeFile},
	{http.MethodPut, []string{segImports, "*", "cursor"}, (*Server).setCursor},
	{http.MethodGet, []string{segChannels, "*"}, (*Server).getChannel},
	{http.MethodGet, []string{segChannels, "*", "threads"}, (*Server).listChannelThreads},
	{http.MethodGet, []string{"threads", "*", "messages"}, (*Server).listThreadMessages},
}

// match returns the path parameters of r for the route, ok false when it does not match.
func (rt importRoute) match(r *http.Request) ([]string, bool) {
	rest, ok := strings.CutPrefix(r.URL.Path, APIPrefix+"/")
	if !ok || r.Method != rt.method {
		return nil, false
	}
	segs := strings.Split(rest, "/")
	if len(segs) != len(rt.pattern) {
		return nil, false
	}
	var params []string
	for i, p := range rt.pattern {
		switch {
		case p == "*":
			params = append(params, segs[i])
		case p != segs[i]:
			return nil, false
		}
	}
	return params, true
}

// serveImports answers a request of the import interface; false when no route matches.
func (s *Server) serveImports(w http.ResponseWriter, r *http.Request, body []byte) bool {
	for _, rt := range importRoutes {
		params, ok := rt.match(r)
		if !ok {
			continue
		}
		if s.protect(w, r, body) == nil {
			return true
		}
		status, out, e := rt.handle(s, r, body, params)
		if e != nil {
			s.importProblem(w, r, e, body)
			return true
		}
		writeJSON(w, status, out)
		return true
	}
	return false
}

// importProblem writes the problem e like problem, with reason and errors.
func (s *Server) importProblem(w http.ResponseWriter, r *http.Request, e *apiError, body []byte) {
	w.Header().Set("DPoP-Nonce", s.currentNonce())
	p := map[string]any{"type": APIPrefix + "/problems/" + e.code, "title": e.code, memberStatus: e.status, "code": e.code,
		"correlation_id": "0192f4a0-7b1c-7577-8bf0-c4f4ee4f887e"}
	if e.reason != "" {
		p["reason"] = e.reason
	}
	if len(e.fields) > 0 {
		p["errors"] = e.fields
	}
	if s.echo {
		p["detail"] = r.Header.Get("Authorization") + " " + r.Header.Get("DPoP") + " " + string(body)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(e.status)
	_ = json.NewEncoder(w).Encode(p)
}

// page cuts one page of n entries by the query parameters limit and cursor; the cursor is the
// base64 of the offset. It returns the bounds and the next cursor (nil on the last page).
func page(r *http.Request, n int) (int, int, *string, *apiError) {
	q := r.URL.Query()
	limit := pageSizeDefault
	if v := q.Get("limit"); v != "" {
		l, err := strconv.Atoi(v)
		if err != nil || l < 1 {
			return 0, 0, nil, invalid(fieldError{Pointer: "#query/limit", Code: codeInvalid})
		}
		limit = min(l, pageSizeMax)
	}
	from := 0
	if v := q.Get("cursor"); v != "" {
		raw, derr := base64.RawURLEncoding.DecodeString(v)
		f, err := strconv.Atoi(string(raw))
		if derr != nil || err != nil || f < 0 || f > n {
			return 0, 0, nil, invalid(fieldError{Pointer: "#query/cursor", Code: codeInvalid})
		}
		from = f
	}
	to := min(from+limit, n)
	if to == n {
		return from, to, nil, nil
	}
	next := base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(to)))
	return from, to, &next, nil
}

// pageOf answers one page of items.
func pageOf(r *http.Request, items []any) (int, any, *apiError) {
	from, to, next, e := page(r, len(items))
	if e != nil {
		return 0, nil, e
	}
	out := map[string]any{memberItems: items[from:to]}
	if next != nil {
		out["next_cursor"] = *next
	}
	return http.StatusOK, out, nil
}

// status is the status member of a person or agent.
func status(active bool) string {
	if active {
		return "active"
	}
	return "disabled"
}

// userJSON is the schema User of p.
func userJSON(p *person) map[string]any {
	return map[string]any{"id": p.id, memberHandle: p.handle, memberDisplayName: p.displayName, "is_owner": p.owner,
		memberStatus: status(p.active), memberCreatedAt: stamp(p.created)}
}

// agentJSON is the schema Agent of p.
func agentJSON(p *person) map[string]any {
	return map[string]any{"id": p.id, memberHandle: p.handle, memberDisplayName: p.displayName, memberStatus: status(p.active),
		"adapter_kind": "hermes", "billing_mode": "organization", "description": "", "operator_config": map[string]any{},
		"presence": map[string]any{memberState: "offline"}, "tool_class": "chat", memberVersion: 1}
}

// listUsers answers listUsers: the person of the session, then the added ones; status filters.
func (s *Server) listUsers(r *http.Request, _ []byte, _ []string) (int, any, *apiError) {
	want := r.URL.Query().Get(memberStatus)
	var items []any
	for _, p := range append([]*person{&s.imp.me}, s.imp.users...) {
		if want == "" || status(p.active) == want {
			items = append(items, userJSON(p))
		}
	}
	return pageOf(r, items)
}

// listAgents answers listAgents.
func (s *Server) listAgents(r *http.Request, _ []byte, _ []string) (int, any, *apiError) {
	items := make([]any, 0, len(s.imp.agents))
	for _, p := range s.imp.agents {
		items = append(items, agentJSON(p))
	}
	return pageOf(r, items)
}

// listSpaces answers listSpaces (SpaceViewList, no pages).
func (s *Server) listSpaces(_ *http.Request, _ []byte, _ []string) (int, any, *apiError) {
	items := make([]any, 0, len(s.imp.spaces))
	for _, sp := range s.imp.spaces {
		items = append(items, view("space", map[string]any{
			"id": sp.id, memberName: sp.name, memberSortKey: sp.sortKey, memberCreatedAt: stamp(sp.created), memberVersion: 1}))
	}
	return http.StatusOK, map[string]any{memberItems: items}, nil
}

// listSpaceChannels answers listSpaceChannels (ChannelViewList); 404 for an unknown space.
func (s *Server) listSpaceChannels(_ *http.Request, _ []byte, params []string) (int, any, *apiError) {
	sp := s.imp.space(params[0])
	if sp == nil {
		return 0, nil, notFound()
	}
	items := make([]any, 0, len(sp.channels))
	for _, ch := range sp.channels {
		items = append(items, view("channel", map[string]any{
			"id": ch.id, memberName: ch.name, memberKind: ch.kind, "space_id": sp.id, memberSortKey: ch.sortKey, "topic": ch.topic,
			"read_only": ch.readOnly, "operator": map[string]any{memberMode: "inherit"}, "settings_version": 1, memberVersion: 1,
			memberCreatedAt: stamp(ch.created)}))
	}
	return http.StatusOK, map[string]any{memberItems: items}, nil
}

// sourceJSON is the schema ImportSource of src.
func sourceJSON(src *source) map[string]any {
	return map[string]any{"id": src.id, memberKind: "discord", memberName: "Example import", "config": src.config, memberState: src.state,
		"stats": src.stats, memberCursor: src.cursor, memberCreatedAt: stamp(src.created)}
}

// itemJSON is the schema ImportItem of it; changed adds source_changed.
func itemJSON(it *item, changed bool) map[string]any {
	out := map[string]any{"id": it.id, "origin_key": it.key, "item_kind": it.kind, memberState: it.state, memberCreatedAt: stamp(it.created)}
	if it.targetID != "" {
		out["target_id"], out["target_kind"] = it.targetID, it.targetKind
	}
	if changed {
		out["source_changed"] = true
	}
	return out
}

// listImports answers listImports.
func (s *Server) listImports(r *http.Request, _ []byte, _ []string) (int, any, *apiError) {
	items := make([]any, 0, len(s.imp.sources))
	for _, src := range s.imp.sources {
		items = append(items, sourceJSON(src))
	}
	return pageOf(r, items)
}

// getImport answers getImport; 404 for an unknown source.
func (s *Server) getImport(_ *http.Request, _ []byte, params []string) (int, any, *apiError) {
	src := s.imp.source(params[0])
	if src == nil {
		return 0, nil, notFound()
	}
	return http.StatusOK, sourceJSON(src), nil
}

// listImportItems answers listImportItems in insertion order; state filters.
func (s *Server) listImportItems(r *http.Request, _ []byte, params []string) (int, any, *apiError) {
	src := s.imp.source(params[0])
	if src == nil {
		return 0, nil, notFound()
	}
	want := r.URL.Query().Get(memberState)
	var items []any
	for _, it := range src.items {
		if want == "" || it.state == want {
			items = append(items, itemJSON(it, false))
		}
	}
	return pageOf(r, items)
}

// running returns the source id when it is running: 404 for an unknown one, 409
// import_not_running otherwise.
func (s *Server) running(id string) (*source, *apiError) {
	src := s.imp.source(id)
	if src == nil {
		return nil, notFound()
	}
	if src.state != sourceRunning {
		return nil, conflict(reasonNotRunning)
	}
	return src, nil
}

// count adds one new transition of kind into state to stats.server.<kind>.<state>.
func (src *source) count(kind, state string) {
	srv := namespace(src.stats, "server")
	k := namespace(srv, kind)
	n, _ := k[state].(float64)
	k[state] = n + 1
}

// countChanged adds one to stats.server.source_changed.
func (src *source) countChanged() {
	srv := namespace(src.stats, "server")
	n, _ := srv["source_changed"].(float64)
	srv["source_changed"] = n + 1
}

// namespace returns the object member name of m, created when missing.
func namespace(m map[string]any, name string) map[string]any {
	ns, ok := m[name].(map[string]any)
	if !ok {
		ns = map[string]any{}
		m[name] = ns
	}
	return ns
}

// setCursor answers setImportCursor: the cursor is replaced, the members of stats merged into
// stats.tool.
func (s *Server) setCursor(_ *http.Request, body []byte, params []string) (int, any, *apiError) {
	var in struct {
		Cursor map[string]any `json:"cursor"`
		Stats  map[string]any `json:"stats"`
	}
	if json.Unmarshal(body, &in) != nil || in.Cursor == nil {
		return 0, nil, invalid(fieldError{Pointer: "/cursor", Code: "required"})
	}
	src, e := s.running(params[0])
	if e != nil {
		return 0, nil, e
	}
	src.cursor = in.Cursor
	tool := namespace(src.stats, "tool")
	for k, v := range in.Stats {
		tool[k] = v
	}
	return http.StatusOK, sourceJSON(src), nil
}

// sha256Hex reports a lowercase hex SHA-256.
func sha256Hex(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == sha256.Size && strings.ToLower(s) == s
}

// digits reports a non-empty string of ASCII digits.
func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// keyID returns the part after "<kind>:" of key, "" when the prefix differs.
func keyID(key, kind string) string {
	rest, ok := strings.CutPrefix(key, kind+":")
	if !ok {
		return ""
	}
	return rest
}

// partDigest returns the base64 SHA-256 of a Content-Digest header (RFC 9530, member sha-256);
// ok is false without exactly one.
func partDigest(h string) ([]byte, bool) {
	var sum []byte
	for _, m := range strings.Split(h, ",") {
		v, found := strings.CutPrefix(strings.TrimSpace(m), "sha-256=:")
		if !found {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(v, ":"))
		if err != nil || len(b) != sha256.Size || sum != nil {
			return nil, false
		}
		sum = b
	}
	return sum, sum != nil
}

// equalDigest reports that data has the SHA-256 want.
func equalDigest(data, want []byte) bool {
	got := sha256.Sum256(data)
	return bytes.Equal(got[:], want)
}

// batchIn is the body of applyImportBatch (schema ImportBatch).
type batchIn struct {
	ThreadOriginKey *string    `json:"thread_origin_key"`
	Items           []objectIn `json:"items"`
}

// objectIn is one item of a batch (schema ImportObject).
type objectIn struct {
	OriginKey     string         `json:"origin_key"`
	ItemKind      string         `json:"item_kind"`
	ContentSHA256 string         `json:"content_sha256"`
	Data          map[string]any `json:"data"`
}

// tx is one batch as one unit: new items and their side effects are staged and committed only
// when every item passed, so a refused batch stores nothing (22 7.2).
type tx struct {
	im        *imports
	src       *source
	now       time.Time
	threadKey string
	items     []*item
	byKey     map[string]*item
	appended  []string
	pins      []string
	spaces    []*space
	channels  []stagedChannel
	counts    [][2]string
	changed   int
	// earlier are the staged earlier sessions of earlierKey.
	earlier    []EarlierSession
	earlierKey string
}

// stagedChannel is a new channel with the space it goes into.
type stagedChannel struct {
	spaceID string
	ch      *channel
}

// item returns the staged or stored item of key, nil without one.
func (t *tx) item(key string) *item {
	if it := t.byKey[key]; it != nil {
		return it
	}
	return t.src.byKey[key]
}

// posted returns the posted item of key with a target, nil for a missing, pending or skipped one.
func (t *tx) posted(key string) *item {
	it := t.item(key)
	if it == nil || it.state != itemPosted || it.targetID == "" {
		return nil
	}
	return it
}

// commit stores the staged state and records the batch b.
func (t *tx) commit(b Batch) {
	for _, it := range t.items {
		t.src.items = append(t.src.items, it)
		t.src.byKey[it.key] = it
	}
	if t.threadKey != "" && len(t.appended) > 0 {
		t.src.threads[t.threadKey] = append(t.src.threads[t.threadKey], t.appended...)
	}
	t.src.pins = append(t.src.pins, t.pins...)
	if len(t.earlier) > 0 {
		t.src.earlier[t.earlierKey] = append(t.src.earlier[t.earlierKey], t.earlier...)
	}
	t.im.spaces = append(t.im.spaces, t.spaces...)
	for _, c := range t.channels {
		if sp := t.im.space(c.spaceID); sp != nil {
			sp.channels = append(sp.channels, c.ch)
		}
	}
	for _, c := range t.counts {
		t.src.count(c[0], c[1])
	}
	for range t.changed {
		t.src.countChanged()
	}
	t.src.batches = append(t.src.batches, b)
}

// applyBatch answers applyImportBatch: the shape checks (400) first, then 404 and 409
// import_not_running, then the items in request order inside one unit.
func (s *Server) applyBatch(_ *http.Request, body []byte, params []string) (int, any, *apiError) {
	var in batchIn
	if err := json.Unmarshal(body, &in); err != nil {
		return 0, nil, invalid(fieldError{Pointer: "", Code: codeInvalid})
	}
	if e := checkBatch(in); e != nil {
		return 0, nil, e
	}
	src, e := s.running(params[0])
	if e != nil {
		return 0, nil, e
	}
	if in.ThreadOriginKey != nil {
		if f := s.imp.batchFaults[*in.ThreadOriginKey]; f != nil && f.times > 0 {
			f.times--
			return 0, nil, &apiError{status: f.status, code: codeConflict, reason: f.reason}
		}
	}
	t := &tx{im: s.imp, src: src, now: s.clk.Now(), byKey: map[string]*item{}}
	var out map[string]any
	if in.ThreadOriginKey != nil {
		out, e = t.thread(*in.ThreadOriginKey, in.Items)
	} else {
		out, e = t.structure(in.Items)
	}
	if e != nil {
		return 0, nil, e
	}
	b := Batch{Keys: make([]string, 0, len(in.Items))}
	if in.ThreadOriginKey != nil {
		b.ThreadOriginKey = *in.ThreadOriginKey
	}
	for _, o := range in.Items {
		b.Keys = append(b.Keys, o.OriginKey)
	}
	t.commit(b)
	return http.StatusOK, out, nil
}

// checkBatch checks the shape of a batch (M8-03 design note 5 a, M8-11 design note 1) and reports
// every violation at once.
func checkBatch(in batchIn) *apiError {
	switch {
	case len(in.Items) == 0:
		return invalid(fieldError{Pointer: "/items", Code: "too_few_items"})
	case len(in.Items) > batchMaxItems:
		return invalid(fieldError{Pointer: "/items", Code: "too_many_items"})
	}
	thread := in.ThreadOriginKey != nil
	var f []fieldError
	if thread && !digits(keyID(*in.ThreadOriginKey, kindThread)) && !digits(keyID(*in.ThreadOriginKey, kindChannel)) {
		f = append(f, fieldError{Pointer: "/thread_origin_key", Code: codeInvalid})
	}
	for i, o := range in.Items {
		base := "/items/" + strconv.Itoa(i)
		if !kindAllowed(o.ItemKind, i, thread) {
			f = append(f, fieldError{Pointer: base + "/item_kind", Code: codeInvalid})
			continue
		}
		if !keyValid(o.OriginKey, o.ItemKind) || (o.ItemKind == kindThread && o.OriginKey != *in.ThreadOriginKey) {
			f = append(f, fieldError{Pointer: base + "/origin_key", Code: codeInvalid})
		}
		if o.ContentSHA256 != "" && !sha256Hex(o.ContentSHA256) {
			f = append(f, fieldError{Pointer: base + "/content_sha256", Code: codeInvalid})
		}
	}
	if len(f) > 0 {
		return invalid(f...)
	}
	return nil
}

// kindAllowed reports whether kind may stand at index i: structure kinds in a structure batch;
// an optional thread item first, then messages and pins in a thread batch.
func kindAllowed(kind string, i int, thread bool) bool {
	if !thread {
		return kind == kindSpace || kind == kindChannel || kind == kindForumTag || kind == kindUser
	}
	return kind == kindMessage || kind == kindPin || (kind == kindThread && i == 0)
}

// keyValid reports an origin key <kind>:<digits>[:<digits>...].
func keyValid(key, kind string) bool {
	rest := keyID(key, kind)
	for _, p := range strings.Split(rest, ":") {
		if !digits(p) {
			return false
		}
	}
	return true
}

// at puts the field pointers of a validation error of item i under /items/<i>/data, as the
// server's itemError does; other errors pass unchanged.
func at(e *apiError, i int) *apiError {
	if e.code != codeValidation {
		return e
	}
	out := *e
	out.fields = nil
	for _, f := range e.fields {
		out.fields = append(out.fields, fieldError{Pointer: "/items/" + strconv.Itoa(i) + "/data" + f.Pointer, Code: f.Code})
	}
	return &out
}

// one writes one item: an existing key answers its stored item (source_changed on another hash),
// a new key gets its target from target ("" means skipped).
func (t *tx) one(o objectIn, target func() (string, string, *apiError)) (map[string]any, *item, *apiError) {
	if old := t.item(o.OriginKey); old != nil {
		if old.kind != o.ItemKind {
			return nil, nil, &apiError{status: http.StatusInternalServerError, code: "internal_error"}
		}
		changed := o.ContentSHA256 != "" && old.sha != "" && o.ContentSHA256 != old.sha
		if changed {
			t.changed++
		}
		return itemJSON(old, changed), old, nil
	}
	kind, id, e := target()
	if e != nil {
		return nil, nil, e
	}
	it := &item{id: newID(), key: o.OriginKey, kind: o.ItemKind, state: itemPosted, targetKind: kind, targetID: id,
		sha: o.ContentSHA256, created: t.now, data: o.Data}
	if id == "" {
		it.state, it.targetKind = itemSkipped, ""
	}
	t.items = append(t.items, it)
	t.byKey[it.key] = it
	t.counts = append(t.counts, [2]string{it.kind, it.state})
	return itemJSON(it, false), it, nil
}

// structure writes a structure batch.
func (t *tx) structure(items []objectIn) (map[string]any, *apiError) {
	out := make([]any, 0, len(items))
	for i, o := range items {
		j, _, e := t.one(o, func() (string, string, *apiError) { return t.structureTarget(o) })
		if e != nil {
			return nil, at(e, i)
		}
		out = append(out, j)
	}
	return map[string]any{memberItems: out}, nil
}

// structureTarget creates or adopts the target of a new structure item.
func (t *tx) structureTarget(o objectIn) (string, string, *apiError) {
	switch o.ItemKind {
	case kindSpace:
		return t.spaceTarget(o)
	case kindChannel:
		return t.channelTarget(o)
	case kindForumTag:
		if t.posted(kindChannel+":"+strings.Split(keyID(o.OriginKey, kindForumTag), ":")[0]) == nil {
			return "", "", conflict(reasonOutside)
		}
		return kindForumTag, newID(), nil
	}
	return t.userTarget(o)
}

// entry returns the member of the list of the stored mapping whose member idKey is id.
func entry(cfg map[string]any, list, idKey, id string) map[string]any {
	l, _ := cfg[list].([]any)
	for _, e := range l {
		if m, _ := e.(map[string]any); m != nil && m[idKey] == id {
			return m
		}
	}
	return nil
}

// str returns the string member k of m, "" without one.
func str(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}

// mappingTarget reads the target of a space or channel entry: skip, an existing UUID or the name
// of a new object.
func mappingTarget(e map[string]any) (bool, string, string) {
	switch v := e["target"].(type) {
	case string:
		return v == targetSkip, "", ""
	case map[string]any:
		if ex := str(v, "existing"); ex != "" {
			return false, ex, ""
		}
		c, _ := v["create"].(map[string]any)
		return false, "", str(c, memberName)
	}
	return false, "", ""
}

// spaceTarget maps a category through the stored mapping (22 5.1 rule 1).
func (t *tx) spaceTarget(o objectIn) (string, string, *apiError) {
	e := entry(t.src.config, "spaces", "discord_category", keyID(o.OriginKey, kindSpace))
	if e == nil {
		return "", "", conflict(reasonOutside)
	}
	skip, existing, name := mappingTarget(e)
	switch {
	case skip:
		return "", "", nil
	case existing != "":
		return kindSpace, existing, nil
	}
	sp := &space{id: newID(), name: name, sortKey: str(o.Data, "sort_key"), created: t.now}
	t.spaces = append(t.spaces, sp)
	return kindSpace, sp.id, nil
}

// channelTarget maps a channel through the stored mapping (22 5.1 rules 1 to 3): a new channel
// lies in the target of its posted space item or of defaults.uncategorized_space; read_only
// follows the lead.
func (t *tx) channelTarget(o objectIn) (string, string, *apiError) {
	e := entry(t.src.config, "channels", "discord_channel", keyID(o.OriginKey, kindChannel))
	if e == nil {
		return "", "", conflict(reasonOutside)
	}
	skip, existing, name := mappingTarget(e)
	switch {
	case skip:
		return "", "", nil
	case existing != "":
		return kindChannel, existing, nil
	}
	spaceKey := str(o.Data, "space_origin_key")
	if spaceKey == "" {
		defaults, _ := t.src.config["defaults"].(map[string]any)
		u := str(defaults, "uncategorized_space")
		if u == "" || u == targetSkip {
			return "", "", nil
		}
		spaceKey = kindSpace + ":" + u
	}
	sp := t.item(spaceKey)
	switch {
	case sp == nil || sp.state != itemPosted && sp.state != itemSkipped:
		return "", "", conflict(reasonOutside)
	case sp.state == itemSkipped:
		return "", "", nil
	}
	kind := str(o.Data, "kind")
	if kind == "" {
		kind = "text"
	}
	ch := &channel{id: newID(), name: name, kind: kind, sortKey: str(o.Data, "sort_key"), topic: str(o.Data, "topic"),
		readOnly: str(e, "lead") != "bothy", created: t.now}
	t.channels = append(t.channels, stagedChannel{spaceID: sp.targetID, ch: ch})
	return kindChannel, ch.id, nil
}

// resolveUser returns the target of a Discord author like domain.Mapping.ResolveUser (22 5.3):
// a webhook is a placeholder; a mapped user its target; otherwise the default of its kind.
func resolveUser(cfg map[string]any, id, discordKind string) string {
	if discordKind == "webhook" {
		return "placeholder"
	}
	if e := entry(cfg, "users", "discord_user", id); e != nil {
		return str(e, "target")
	}
	defaults, _ := cfg["defaults"].(map[string]any)
	def := str(defaults, "unmapped_human")
	if discordKind == "bot" {
		def = str(defaults, "unmapped_bot")
	}
	if def == targetSkip {
		return targetSkip
	}
	return "placeholder"
}

// userTarget maps a Discord author: a person or agent by handle (409 target_outside_source when
// unknown), a new placeholder, or skipped.
func (t *tx) userTarget(o objectIn) (string, string, *apiError) {
	target := resolveUser(t.src.config, keyID(o.OriginKey, kindUser), str(o.Data, "discord_kind"))
	if target == targetSkip {
		return "", "", nil
	}
	if h, ok := strings.CutPrefix(target, kindUser+":"); ok {
		p := byHandle(append([]*person{&t.im.me}, t.im.users...), h)
		if p == nil {
			return "", "", conflict(reasonOutside)
		}
		return kindUser, p.id, nil
	}
	if h, ok := strings.CutPrefix(target, kindAgent+":"); ok {
		p := byHandle(t.im.agents, h)
		if p == nil {
			return "", "", conflict(reasonOutside)
		}
		return kindAgent, p.id, nil
	}
	return kindUser, newID(), nil
}

// thread writes a thread batch (M8-11 design notes 1 to 4): the main thread of a posted channel,
// the thread its first item creates or answers, or a posted thread; then messages and pins.
func (t *tx) thread(key string, items []objectIn) (map[string]any, *apiError) {
	t.threadKey = key
	out := make([]any, 0, len(items))
	start, live := 0, false
	switch {
	case keyID(key, kindChannel) != "":
		live = t.posted(key) != nil
		if !live {
			return nil, conflict(reasonThreadMissing)
		}
	case items[0].ItemKind == kindThread:
		j, it, e := t.one(items[0], func() (string, string, *apiError) { return t.threadTarget(items[0]) })
		if e != nil {
			return nil, at(e, 0)
		}
		if it.state == itemPosted {
			if outcome := t.earlierSession(key, items[0].Data); outcome != "" {
				j["session"] = outcome
			}
		}
		out, start, live = append(out, j), 1, it.state == itemPosted
	default:
		live = t.posted(key) != nil
	}
	if !live && len(items) > start {
		return nil, conflict(reasonThreadMissing)
	}
	for i := start; i < len(items); i++ {
		j, e := t.content(items[i])
		if e != nil {
			return nil, at(e, i)
		}
		out = append(out, j)
	}
	floor := len(t.src.threads[key]) + len(t.appended)
	return map[string]any{memberItems: out, "unread_floor_seq": floor}, nil
}

// earlierSession applies the member earlier_session of a thread item of a posted thread (M8-06):
// dropped_late for a thread marked late, already_present for an agent and reference stored before,
// otherwise the session is staged and taken_over; "" without the member. The agent must be an
// agent of the fake, like the operator check of the server.
func (t *tx) earlierSession(key string, data map[string]any) string {
	es, _ := data["earlier_session"].(map[string]any)
	if es == nil {
		return ""
	}
	in := EarlierSession{AgentID: str(es, "agent_id"), Ref: str(es, "external_session_ref"), Currency: str(es, "currency")}
	if f, ok := es["cost_baseline_micros"].(float64); ok {
		in.Baseline = int64(f)
	}
	if t.src.late[key] {
		return "dropped_late"
	}
	for _, old := range append(append([]EarlierSession(nil), t.src.earlier[key]...), t.earlier...) {
		if old.AgentID == in.AgentID && old.Ref == in.Ref {
			return "already_present"
		}
	}
	t.earlier, t.earlierKey = append(t.earlier, in), key
	return "taken_over"
}

// threadTarget creates the thread of a new thread item in the channel of its posted channel item;
// a skipped channel skips the thread.
func (t *tx) threadTarget(o objectIn) (string, string, *apiError) {
	chKey := str(o.Data, "channel_origin_key")
	if ch := t.item(chKey); ch != nil && ch.state == itemSkipped {
		return "", "", nil
	}
	if t.posted(chKey) == nil {
		return "", "", conflict(reasonOutside)
	}
	return kindThread, newID(), nil
}

// content writes a message or pin item of the thread.
func (t *tx) content(o objectIn) (map[string]any, *apiError) {
	target := func() (string, string, *apiError) { return t.messageTarget(o) }
	if o.ItemKind == kindPin {
		target = func() (string, string, *apiError) { return t.pinTarget(o) }
	}
	j, _, e := t.one(o, target)
	return j, e
}

// messageTarget appends a new message (M8-11 design note 4): a posted author (409
// author_not_imported), a posted reply target and posted files of this source (400).
func (t *tx) messageTarget(o objectIn) (string, string, *apiError) {
	if t.posted(str(o.Data, "author_origin_key")) == nil {
		return "", "", conflict(reasonAuthorMissing)
	}
	if r := str(o.Data, "reply_to_origin_key"); r != "" && t.posted(r) == nil {
		return "", "", invalid(fieldError{Pointer: "/reply_to_origin_key", Code: "not_in_thread"})
	}
	list, _ := o.Data["attachment_file_ids"].([]any)
	var f []fieldError
	for j, v := range list {
		id, _ := v.(string)
		if !t.src.postedTarget(id, kindFile) {
			f = append(f, fieldError{Pointer: "/attachment_file_ids/" + strconv.Itoa(j), Code: codeNotImported})
		}
	}
	if len(f) > 0 {
		return "", "", invalid(f...)
	}
	t.appended = append(t.appended, o.OriginKey)
	return kindMessage, newID(), nil
}

// pinTarget pins the message of a posted message item (22 6.6); the target is the message.
func (t *tx) pinTarget(o objectIn) (string, string, *apiError) {
	key := str(o.Data, "message_origin_key")
	m := t.posted(key)
	if m == nil {
		return "", "", invalid(fieldError{Pointer: "/message_origin_key", Code: codeNotImported})
	}
	t.pins = append(t.pins, key)
	return kindMessage, m.targetID, nil
}

// postedTarget reports a posted item of the source whose target is id with one of kinds.
func (src *source) postedTarget(id string, kinds ...string) bool {
	for _, it := range src.items {
		if it.state != itemPosted || it.targetID != id {
			continue
		}
		for _, k := range kinds {
			if it.targetKind == k {
				return true
			}
		}
	}
	return false
}

// file returns the file of a file item of the source with the UUID id, nil when unknown.
func (src *source) file(id string) *file {
	for _, f := range src.files {
		if f.FileID == id {
			return f
		}
	}
	return nil
}

// partLen is the length of part n: the part size, the rest for the last part.
func (f *file) partLen(n int) int64 {
	if n < f.partCount {
		return f.partSize
	}
	return f.Size - f.partSize*int64(f.partCount-1)
}

// slot is the schema ImportFileSlot of it; a pending file carries its upload session.
func (s *Server) slot(src *source, it *item, f *file) map[string]any {
	out := map[string]any{"item": itemJSON(it, false)}
	if f == nil || it.state != itemPending {
		return out
	}
	prefix := APIPrefix + "/imports/" + src.id + "/files/" + f.FileID + "/parts/"
	parts := make([]any, 0, f.partCount)
	for n := 1; n <= f.partCount; n++ {
		parts = append(parts, map[string]any{memberPartNumber: n, "url": prefix + strconv.Itoa(n),
			"headers": []any{map[string]any{memberName: "Content-Type", "value": "application/octet-stream"}}})
	}
	out["upload"] = map[string]any{"file_id": f.FileID, "mode": "direct", "part_size_bytes": f.partSize, "parts": parts,
		"expires_at": stamp(s.clk.Now().Add(sessionTTL))}
	return out
}

// fileIn is the body of createImportFile (schema ImportFileCreate).
type fileIn struct {
	OriginKey string `json:"origin_key"`
	ChannelID string `json:"channel_id"`
	AuthorID  string `json:"author_id"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

// createFile answers createImportFile (M8-11 design note 6): the body checks, 404 and 409
// import_not_running, the channel of a posted channel item (409 target_outside_source), the author
// of a posted user item (409 author_not_imported), then the slot: a repeated key answers the same
// file, without upload once it is posted.
func (s *Server) createFile(_ *http.Request, body []byte, params []string) (int, any, *apiError) {
	var in fileIn
	if err := json.Unmarshal(body, &in); err != nil {
		return 0, nil, invalid(fieldError{Pointer: "", Code: codeInvalid})
	}
	var f []fieldError
	if !digits(keyID(in.OriginKey, kindFile)) {
		f = append(f, fieldError{Pointer: "/origin_key", Code: codeInvalid})
	}
	if !sha256Hex(in.SHA256) {
		f = append(f, fieldError{Pointer: "/sha256", Code: codeInvalid})
	}
	if len(f) > 0 {
		return 0, nil, invalid(f...)
	}
	src, e := s.running(params[0])
	switch {
	case e != nil:
		return 0, nil, e
	case !src.postedTarget(in.ChannelID, kindChannel):
		return 0, nil, conflict(reasonOutside)
	case !src.postedTarget(in.AuthorID, kindUser, kindAgent):
		return 0, nil, conflict(reasonAuthorMissing)
	}
	if old := src.byKey[in.OriginKey]; old != nil {
		if old.kind != kindFile {
			return 0, nil, &apiError{status: http.StatusInternalServerError, code: "internal_error"}
		}
		return http.StatusOK, s.slot(src, old, src.file(old.targetID)), nil
	}
	fl := &file{File: File{OriginKey: in.OriginKey, FileID: newID(), Name: in.Name, Size: in.SizeBytes, SHA256: in.SHA256},
		partSize: s.imp.partSize, parts: map[int]filePart{}}
	fl.partCount = int(max(1, (in.SizeBytes+fl.partSize-1)/fl.partSize))
	it := &item{id: newID(), key: in.OriginKey, kind: kindFile, state: itemPending, targetKind: kindFile, targetID: fl.FileID, created: s.clk.Now()}
	src.items = append(src.items, it)
	src.byKey[it.key] = it
	src.files = append(src.files, fl)
	src.count(kindFile, itemPending)
	return http.StatusOK, s.slot(src, it, fl), nil
}

// uploadPart answers uploadImportFilePart: 404 and 409 import_not_running, 404 for a file of no
// file item of the source, then the part with the rules of uploadPart (12 3.2).
func (s *Server) uploadPart(r *http.Request, body []byte, params []string) (int, any, *apiError) {
	src, e := s.running(params[0])
	if e != nil {
		return 0, nil, e
	}
	f := src.file(params[1])
	if f == nil {
		return 0, nil, notFound()
	}
	n, err := strconv.Atoi(params[2])
	if err != nil || n < 1 || n > f.partCount {
		return 0, nil, invalid(fieldError{Pointer: "#path/part_number", Code: codeInvalid})
	}
	sum, ok := partDigest(r.Header.Get("Content-Digest"))
	if !ok {
		return 0, nil, invalid(fieldError{Pointer: pointerContentDigest, Code: codeInvalid})
	}
	if old, done := f.parts[n]; done {
		if old.digest != hex.EncodeToString(sum) {
			return 0, nil, conflict(reasonPartConflict)
		}
		return http.StatusOK, map[string]any{memberPartNumber: n, "etag": old.etag}, nil
	}
	if int64(len(body)) != f.partLen(n) || !equalDigest(body, sum) {
		return 0, nil, invalid(fieldError{Pointer: pointerContentDigest, Code: codeDigestMismatch})
	}
	p := filePart{digest: hex.EncodeToString(sum), etag: `"` + hex.EncodeToString(sum[:16]) + `"`, data: append([]byte(nil), body...)}
	f.parts[n] = p
	return http.StatusOK, map[string]any{memberPartNumber: n, "etag": p.etag}, nil
}

// completeFile answers completeImportFile (12 3.4): every part confirmed with its ETag (409
// upload_incomplete, part_conflict), the bytes against the declared size and SHA-256 (400), then
// the item posted. A posted item answers again without a change.
func (s *Server) completeFile(_ *http.Request, body []byte, params []string) (int, any, *apiError) {
	var in struct {
		Parts []struct {
			PartNumber int    `json:"part_number"`
			Etag       string `json:"etag"`
		} `json:"parts"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return 0, nil, invalid(fieldError{Pointer: "", Code: codeInvalid})
	}
	src, e := s.running(params[0])
	if e != nil {
		return 0, nil, e
	}
	f := src.file(params[1])
	if f == nil {
		return 0, nil, notFound()
	}
	it := src.byKey[f.OriginKey]
	if it.state == itemPosted {
		return http.StatusOK, s.slot(src, it, f), nil
	}
	given := map[int]bool{}
	for _, p := range in.Parts {
		stored, ok := f.parts[p.PartNumber]
		if !ok {
			return 0, nil, conflict(reasonIncomplete)
		}
		if stored.etag != p.Etag {
			return 0, nil, conflict(reasonPartConflict)
		}
		given[p.PartNumber] = true
	}
	var all []byte
	for n := 1; n <= f.partCount; n++ {
		if !given[n] {
			return 0, nil, conflict(reasonIncomplete)
		}
		all = append(all, f.parts[n].data...)
	}
	sum := sha256.Sum256(all)
	if int64(len(all)) != f.Size || hex.EncodeToString(sum[:]) != f.SHA256 {
		return 0, nil, invalid(fieldError{Pointer: "/parts", Code: codeDigestMismatch})
	}
	f.Posted, f.Bytes, it.state = true, all, itemPosted
	src.count(kindFile, itemPosted)
	return http.StatusOK, s.slot(src, it, f), nil
}

// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"
)

// baseURL is the prefix of every REST path (06 "Overview", W6). The spec must declare exactly
// this server URL, and the generated registration prefixes every path with it; both are checked
// here so that spec and server cannot drift apart.
const baseURL = "/api/v1"

// classNone is the rate-limit class of system actions and internal delivery checks.
const classNone = "none"

// methods are the HTTP methods of a path item (OpenAPI 3.1, "Path Item Object"), in the order the
// generator walks them.
var methods = []string{
	http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete,
	http.MethodOptions, http.MethodHead, http.MethodPatch, http.MethodTrace,
}

// Media types of request bodies and the body kind the route table records for each (06
// "Basic rules", row "Media types").
var bodyKinds = map[string]bodyKind{
	mediaJSON:                  bodyJSON,
	"multipart/form-data":      bodyMultipart,
	"application/octet-stream": bodyOctetStream,
}

// mediaJSON is the media type of JSON bodies and answers.
const mediaJSON = "application/json"

// reportMediaTypes is the only pair of media types one request body may declare: the Reporting
// API format and the legacy report-uri format of a Content Security Policy violation report
// (submitCspReport, docs/spec/19-security.md 5 rule 6). Both are JSON; the route table records
// them as the body kind report.
var reportMediaTypes = []string{"application/csp-report", "application/reports+json"}

// ifMatch is the If-Match mode of an operation, as rendered into the route table.
type ifMatch string

// If-Match modes.
const (
	ifMatchNone     ifMatch = "none"
	ifMatchOptional ifMatch = "optional"
	ifMatchRequired ifMatch = "required"
)

// bodyKind is the request body kind of an operation, as rendered into the route table.
type bodyKind string

// Request body kinds.
const (
	bodyNone        bodyKind = "none"
	bodyJSON        bodyKind = "json"
	bodyMultipart   bodyKind = "multipart"
	bodyOctetStream bodyKind = "octet-stream"
	bodyReport      bodyKind = "report"
)

// opMeta is everything the route table records about one operation under paths. The fields
// mirror routes.Operation; Path and Pattern include the base URL.
type opMeta struct {
	ID             string
	Method         string
	Path           string
	Pattern        string
	Action         string
	RateLimitClass string
	Public         bool
	IdempotencyKey bool
	IfMatch        ifMatch
	RequestBody    bodyKind
	BodyRequired   bool
	ETagStatuses   []int
	Download       bool
	OwnCredential  bool
}

// specWalk reads the operations of one spec and reports its problems.
type specWalk struct {
	path      string                   // spec file, for problem messages
	catalog   map[string]catalogAction // known actions by ID
	catalogAt string                   // catalogue file, for problem messages
	root      string                   // key of the root security requirements, see securityKey
	rootReqs  map[string]bool          // key of each root security requirement alone (fetch tickets, D-M3-05-14)
	p         *problemsError
}

// readOperations loads the spec exactly as oapi-codegen v2.8.0 loads it (the kin-openapi loader of
// the same version, on the copy without webhooks that oapi-codegen is given) and walks every
// operation under paths: it checks the operation against the catalogue and derives its metadata
// from the resolved model. Deriving the metadata from the model the generated server is built from
// is the point: every form the loader resolves (reference chains, any spelling of a YAML boolean,
// headers declared inline) reaches the table as it reaches the server, so the two cannot disagree
// (docs/DECISIONS.md M1-17/18). YAML merge keys are refused, because the filter works on the node
// tree, in which merged content is invisible.
//
// Webhooks are not walked: the server sends them and never serves them. ops is sorted by
// operation ID; overrides holds the IDs of operations with security requirements of their own,
// which the route table cannot describe (checked for enabled operations in loadInputs).
func readOperations(specPath string, data []byte, catalogPath string, actions []catalogAction, p *problemsError) (ops []opMeta, overrides map[string]bool) {
	doc := loadSpec(specPath, data, p)
	if doc == nil {
		return nil, nil
	}
	w := &specWalk{path: specPath, catalog: map[string]catalogAction{}, catalogAt: catalogPath, root: securityKey(doc.Security), rootReqs: requirementKeys(doc.Security), p: p}
	for _, a := range actions {
		w.catalog[a.ID] = a
	}
	w.checkServers(doc.Servers)

	ops, overrides = w.operations(doc.Paths.Map())
	slices.SortFunc(ops, func(a, b opMeta) int { return strings.Compare(a.ID, b.ID) })
	for _, id := range healthOperations(ops) {
		p.add(specPath, "%s looks like a health probe; liveness and readiness belong on the internal listener, not in the public API (Ruling L2, 06 R-API-16)", id)
	}
	return ops, overrides
}

// loadSpec checks the node tree of the spec (no merge keys, only the path item keys the filter
// reads) and loads the copy without webhooks with the loader oapi-codegen uses. It returns nil
// after adding the problem when a step fails.
func loadSpec(specPath string, data []byte, p *problemsError) *openapi3.T {
	var tree yaml.Node
	if err := yaml.Unmarshal(data, &tree); err != nil {
		p.add(specPath, "%v", err)
		return nil
	}
	if at := mergeKeys(&tree, ""); len(at) > 0 {
		p.add(specPath, "YAML merge keys (<<) are not supported by the route generator: %s", strings.Join(at, ", "))
		return nil
	}
	if !checkPathItemKeys(specPath, &tree, p) {
		return nil
	}
	stripped, err := filterSpec(data, func(string) bool { return true })
	if err != nil {
		p.add(specPath, "%v", err)
		return nil
	}
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(stripped)
	if err != nil {
		p.add(specPath, "cannot be loaded as oapi-codegen loads it (kin-openapi): %v", err)
		return nil
	}
	return doc
}

// operationSet collects the operations of a walk in walk order.
type operationSet struct {
	ops       []opMeta
	overrides map[string]bool   // IDs of operations with security requirements of their own
	firstUse  map[string]string // method and path of the first operation with each ID
}

// operations walks the operations of every path, the paths in byte order and the methods of a
// path in the order of methods. It returns the operations that have metadata and the IDs of
// those with security requirements of their own.
func (w *specWalk) operations(paths map[string]*openapi3.PathItem) (ops []opMeta, overrides map[string]bool) {
	set := operationSet{overrides: map[string]bool{}, firstUse: map[string]string{}}
	for _, path := range slices.Sorted(maps.Keys(paths)) {
		if !strings.HasPrefix(path, "/") {
			w.p.add(w.path, "paths: %q does not start with /", path)
			continue
		}
		byMethod := paths[path].Operations()
		for _, method := range methods {
			if node, ok := byMethod[method]; ok {
				w.add(&set, method, path, node)
			}
		}
	}
	return set.ops, set.overrides
}

// add walks one operation and adds it to set. An operation without metadata is left out, and so
// is one whose operation ID an operation walked before already uses; that is a problem.
func (w *specWalk) add(set *operationSet, method, path string, node *openapi3.Operation) {
	op, override, ok := w.operation(method, path, node)
	if !ok {
		return
	}
	label := op.Method + " " + path
	if first, dup := set.firstUse[op.ID]; dup {
		w.p.add(w.path, "operationId %s is used by more than one operation: %s and %s", op.ID, first, label)
		return
	}
	set.firstUse[op.ID] = label
	if override {
		set.overrides[op.ID] = true
	}
	set.ops = append(set.ops, op)
}

// pathItemKeys are the keys a path item may have: the lower-case methods the filter reads, and
// summary and description, which change no operation.
var pathItemKeys = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace", "summary", "description"}

// checkPathItemKeys refuses every other key of a path item on the node tree of the spec, before
// anything is filtered: $ref, parameters and servers would change what an operation declares,
// and a method the filter does not read (connect, a key in other case) would reach oapi-codegen
// without an entry in the route table. It reports whether the spec passed.
func checkPathItemKeys(specPath string, tree *yaml.Node, p *problemsError) bool {
	ok := true
	paths := value(tree.Content[0], "paths")
	for _, path := range keys(paths) {
		for _, key := range keys(value(paths, path)) {
			if !slices.Contains(pathItemKeys, key) {
				p.add(specPath, "paths: %s: path item key %q is not supported by the route generator (allowed: %s)", path, key, strings.Join(pathItemKeys, ", "))
				ok = false
			}
		}
	}
	return ok
}

// healthProbe matches operation IDs and path segments of health probes.
var (
	healthProbeID      = regexp.MustCompile(`(?i)(health|liveness|readiness)`)
	healthProbeSegment = []string{"health", "healthz", "livez", "liveness", "readyz", "readiness"}
)

// healthOperations returns, sorted, the IDs of operations that look like health probes: the ID
// names health, liveness or readiness, or a path segment is a usual probe path (W2).
func healthOperations(ops []opMeta) []string {
	var out []string
	for _, op := range ops {
		probe := healthProbeID.MatchString(op.ID)
		for _, seg := range strings.Split(op.Path, "/") {
			probe = probe || slices.Contains(healthProbeSegment, strings.ToLower(seg))
		}
		if probe {
			out = append(out, op.ID)
		}
	}
	slices.Sort(out)
	return out
}

// checkServers requires exactly one server with the base URL (W6).
func (w *specWalk) checkServers(servers openapi3.Servers) {
	if len(servers) == 1 && servers[0] != nil && servers[0].URL == baseURL {
		return
	}
	w.p.add(w.path, "servers must be exactly one entry with url %s (06 \"Overview\", W6)", baseURL)
}

// operation checks one operation and derives its metadata. override reports security
// requirements of its own; ok is false when the operation has no operationId and can therefore
// not be listed.
func (w *specWalk) operation(method, path string, node *openapi3.Operation) (op opMeta, override, ok bool) {
	if node.OperationID == "" {
		w.p.add(w.path, "%s %s: missing operationId", method, path)
		return opMeta{}, false, false
	}
	at := fmt.Sprintf("%s: %s (%s %s)", w.path, node.OperationID, method, path)
	op = opMeta{
		ID:          node.OperationID,
		Method:      method,
		Path:        baseURL + path,
		Pattern:     pattern(method, baseURL, path),
		IfMatch:     ifMatchNone,
		RequestBody: bodyNone,
	}
	if node.Servers != nil && len(*node.Servers) > 0 {
		w.p.add(at, "operation-level servers are not supported; every operation is served below %s", baseURL)
	}
	op.Action, op.RateLimitClass = w.extensions(at, node.Extensions)
	op.Public, override, op.OwnCredential = w.security(at, node.Security)
	op.IdempotencyKey, op.IfMatch = w.parameters(at, node.Parameters)
	op.RequestBody, op.BodyRequired = w.requestBody(at, node.RequestBody)
	op.ETagStatuses = w.etagStatuses(at, node.Responses)
	op.Download = downloads(node.Responses)
	if op.IdempotencyKey {
		w.checkKeyed(at, op, node)
	}
	return op, override, true
}

// checkKeyed holds the rules of a keyed operation (06 "Idempotency" point 4, docs/DECISIONS.md
// M1-20): the fingerprint covers method, path and body, so no query parameter; the stage stores
// only JSON, so the body is absent or application/json and every 2xx response has
// application/json content or none; idempotency_keys keeps no headers, so a 2xx with ETag has a
// required integer member version from which a replay derives the ETag.
func (w *specWalk) checkKeyed(at string, op opMeta, node *openapi3.Operation) {
	for _, ref := range node.Parameters {
		if ref != nil && ref.Value != nil && ref.Value.In == openapi3.ParameterInQuery {
			w.p.add(at, "declares Idempotency-Key and the query parameter %q; the fingerprint covers the path only", ref.Value.Name)
		}
	}
	if op.RequestBody != bodyNone && op.RequestBody != bodyJSON {
		w.p.add(at, "declares Idempotency-Key with a %s request body; a keyed body is application/json only", op.RequestBody)
	}
	if node.Responses == nil {
		return
	}
	byKey := node.Responses.Map()
	for _, key := range slices.Sorted(maps.Keys(byKey)) {
		code, isRange, ok := responseStatus(key)
		success := ok && (isRange && key[0] == '2' || code >= 200 && code < 300)
		if ref := byKey[key]; success && ref != nil && ref.Value != nil {
			w.checkKeyedSuccess(at, key, slices.Contains(op.ETagStatuses, code), ref.Value)
		}
	}
}

// checkKeyedSuccess holds the rules of one 2xx response of a keyed operation; etag reports that
// the response declares ETag.
func (w *specWalk) checkKeyedSuccess(at, key string, etag bool, resp *openapi3.Response) {
	for _, media := range slices.Sorted(maps.Keys(resp.Content)) {
		if media != mediaJSON {
			w.p.add(at, "declares Idempotency-Key and response %s with %s; a stored response is application/json or empty", key, media)
		}
	}
	if !etag {
		return
	}
	if body := resp.Content[mediaJSON]; body == nil || !requiredInteger(body.Schema, "version", 0) {
		w.p.add(at, "declares Idempotency-Key and ETag on response %s without a required integer member version; a replay derives the ETag from it", key)
	}
}

// maxSchemaDepth bounds the walk of allOf in requiredInteger.
const maxSchemaDepth = 16

// requiredInteger reports whether schema (or one member of its allOf) requires the member name
// with type integer.
func requiredInteger(ref *openapi3.SchemaRef, name string, depth int) bool {
	if ref == nil || ref.Value == nil || depth > maxSchemaDepth {
		return false
	}
	s := ref.Value
	if prop := s.Properties[name]; slices.Contains(s.Required, name) && prop != nil && prop.Value != nil &&
		prop.Value.Type != nil && prop.Value.Type.Is(openapi3.TypeInteger) {
		return true
	}
	for _, member := range s.AllOf {
		if requiredInteger(member, name, depth+1) {
			return true
		}
	}
	return false
}

// extensions checks x-authz-action and x-rate-limit-class against the catalogue (05 rules 3 and
// 10). M1-03 checks the same pairing in G2; this check guards the generated table. Class none
// marks system actions and internal delivery checks (05 "Rate-limit classes"), which have no
// REST route (06 "Authorization in the API", item 7).
func (w *specWalk) extensions(at string, ext map[string]any) (action, class string) {
	action, hasAction := ext["x-authz-action"].(string)
	class, hasClass := ext["x-rate-limit-class"].(string)
	if !hasAction || action == "" {
		w.p.add(at, "missing x-authz-action (a string)")
	}
	if !hasClass || class == "" {
		w.p.add(at, "missing x-rate-limit-class (a string)")
	}
	if class == classNone {
		w.p.add(at, "x-rate-limit-class none is not a class of a REST operation; it marks system actions and internal checks")
	}
	if action == "" {
		return action, class
	}
	known, ok := w.catalog[action]
	switch {
	case !ok:
		w.p.add(at, "x-authz-action %q is not in %s", action, w.catalogAt)
	case known.Right == rightSystem:
		w.p.add(at, "x-authz-action %q is a system action (right system); system actions are not reachable over the API (05 rule 10)", action)
	case class != "" && class != known.Class:
		w.p.add(at, "x-rate-limit-class %q differs from %s, the class of %s in %s", class, known.Class, action, w.catalogAt)
	}
	return action, class
}

// security reports whether the operation is public (security: [], the only public form) and
// whether it overrides the root security with requirements of its own. An empty requirement ({})
// would mean optional authentication, which the route table has no mode for. The fetch ticket
// beside some of the root requirements is no override (fileTicketScheme). An operation whose only
// requirement is pushPreviewTokenScheme checks its own credential (own, M3-22): no override,
// because the route flag OwnCredential tells the HTTP chain to leave the credential to it.
func (w *specWalk) security(at string, security *openapi3.SecurityRequirements) (public, override, own bool) {
	if security == nil {
		return false, false, false
	}
	if ownCredential(*security) {
		return false, false, true
	}
	for _, r := range *security {
		if len(r) == 0 {
			w.p.add(at, "security lists an empty requirement ({}); optional authentication is not a mode of the route table")
		}
	}
	if len(*security) == 0 {
		return true, false, false
	}
	rest, ticket := withoutFileTicket(*security)
	if !ticket {
		return false, securityKey(*security) != w.root, false
	}
	for k := range requirementKeys(rest) {
		if !w.rootReqs[k] {
			return false, true, false
		}
	}
	return false, len(rest) == 0, false
}

// pushPreviewTokenScheme is the security scheme of the preview token of a push registration
// (docs/spec/13-push-notifications.md 6.4, M3-22): only getPushPreview accepts it, and its use
// case checks it with the DPoP proof of the device key, so the Authenticator never sees it.
const pushPreviewTokenScheme = "pushPreviewToken"

// ownCredential reports security requirements that name exactly one scheme,
// pushPreviewTokenScheme, without scopes.
func ownCredential(reqs openapi3.SecurityRequirements) bool {
	if len(reqs) != 1 || len(reqs[0]) != 1 {
		return false
	}
	scopes, ok := reqs[0][pushPreviewTokenScheme]
	return ok && len(scopes) == 0
}

// requirementKeys returns the key of every requirement of reqs alone (securityKey of one).
func requirementKeys(reqs openapi3.SecurityRequirements) map[string]bool {
	out := map[string]bool{}
	for _, r := range reqs {
		out[securityKey(openapi3.SecurityRequirements{r})] = true
	}
	return out
}

// fileTicketScheme is the security scheme of the fetch tickets of downloadFile and
// downloadThumbnail (docs/spec/12-files.md 9.2, ADR 0048): an alternative to the header
// credentials that the file handler checks itself, after the chain let the anonymous request
// through. An operation that lists it beside some of the root requirements is no override; the
// route table records it as root security (docs/decisions.d/M3-05.md D-M3-05-14).
const fileTicketScheme = "fileTicket"

// withoutFileTicket returns reqs without the requirement that names only fileTicketScheme and
// whether reqs had it.
func withoutFileTicket(reqs openapi3.SecurityRequirements) (openapi3.SecurityRequirements, bool) {
	out := make(openapi3.SecurityRequirements, 0, len(reqs))
	found := false
	for _, r := range reqs {
		if _, ok := r[fileTicketScheme]; ok && len(r) == 1 {
			found = true
			continue
		}
		out = append(out, r)
	}
	return out, found
}

// securityKey renders security requirements in a canonical form for comparison: every
// requirement as its sorted schemes with their sorted scopes, the requirements sorted.
func securityKey(reqs openapi3.SecurityRequirements) string {
	parts := make([]string, 0, len(reqs))
	for _, r := range reqs {
		schemes := make([]string, 0, len(r))
		for _, name := range slices.Sorted(maps.Keys(r)) {
			scopes := slices.Clone(r[name])
			slices.Sort(scopes)
			schemes = append(schemes, name+"["+strings.Join(scopes, " ")+"]")
		}
		parts = append(parts, strings.Join(schemes, " "))
	}
	slices.Sort(parts)
	return strings.Join(parts, " | ")
}

// parameters finds the Idempotency-Key and If-Match header parameters of the resolved
// operation, however they are declared: the generated server binds them either way.
func (w *specWalk) parameters(at string, params openapi3.Parameters) (idempotency bool, mode ifMatch) {
	mode = ifMatchNone
	for _, ref := range params {
		if ref == nil || ref.Value == nil {
			w.p.add(at, "parameter without a resolved definition")
			continue
		}
		prm := ref.Value
		if prm.In != openapi3.ParameterInHeader {
			continue
		}
		switch {
		case strings.EqualFold(prm.Name, "Idempotency-Key"):
			if idempotency {
				w.p.add(at, "declares the header Idempotency-Key more than once")
			}
			idempotency = true
		case strings.EqualFold(prm.Name, "If-Match"):
			if mode != ifMatchNone {
				w.p.add(at, "declares If-Match more than once")
			}
			mode = ifMatchMode(prm.Required)
		}
	}
	return idempotency, mode
}

// ifMatchMode returns the If-Match mode of a declared If-Match header: required or optional.
func ifMatchMode(required bool) ifMatch {
	if required {
		return ifMatchRequired
	}
	return ifMatchOptional
}

// requestBody returns the body kind and whether the body is required. A body has exactly one
// media type of bodyKinds, or exactly the pair reportMediaTypes (body kind report).
func (w *specWalk) requestBody(at string, body *openapi3.RequestBodyRef) (bodyKind, bool) {
	if body == nil {
		return bodyNone, false
	}
	if body.Value == nil {
		w.p.add(at, "request body without a resolved definition")
		return bodyNone, false
	}
	types := slices.Sorted(maps.Keys(body.Value.Content))
	if slices.Equal(types, reportMediaTypes) {
		return bodyReport, body.Value.Required
	}
	if len(types) != 1 {
		w.p.add(at, "request body declares %d media types; the route table supports exactly one media type", len(types))
		return bodyNone, body.Value.Required
	}
	kind, ok := bodyKinds[types[0]]
	if !ok {
		w.p.add(at, "request body media type %q is not one of application/json, multipart/form-data, application/octet-stream", types[0])
		return bodyNone, body.Value.Required
	}
	return kind, body.Value.Required
}

// etagStatuses returns the 2xx statuses whose response declares the ETag header, ascending.
// Response keys are status codes, status ranges (1XX to 5XX) or default; an ETag on a range has
// no single status for the table and is rejected.
func (w *specWalk) etagStatuses(at string, responses *openapi3.Responses) []int {
	if responses == nil {
		return nil
	}
	var out []int
	byKey := responses.Map()
	for _, key := range slices.Sorted(maps.Keys(byKey)) {
		if key == "default" {
			continue
		}
		code, isRange, ok := responseStatus(key)
		if !ok {
			w.p.add(at, "response key %q is not a status code, a status range or default", key)
			continue
		}
		ref := byKey[key]
		if ref == nil || ref.Value == nil {
			w.p.add(at, "response %s without a resolved definition", key)
			continue
		}
		if !declaresETag(ref.Value.Headers) {
			continue
		}
		switch {
		case isRange:
			w.p.add(at, "response %s declares ETag on a status range; name the status", key)
		case code >= 200 && code < 300:
			out = append(out, code)
		}
	}
	slices.Sort(out)
	return out
}

// downloads reports whether a 2xx response of the operation streams a body that is not JSON
// (application/x-ndjson, application/octet-stream, image/*, application/zip): such an operation
// is in the route class of downloads, whose deadline is http.transfer_timeout (02 section 14.2).
// application/json and every +json type (RFC 6839) are JSON answers of the normal class.
func downloads(responses *openapi3.Responses) bool {
	if responses == nil {
		return false
	}
	for key, ref := range responses.Map() {
		code, isRange, ok := responseStatus(key)
		success := isRange && key[0] == '2' || code >= 200 && code < 300
		if !ok || ref == nil || ref.Value == nil || !success {
			continue
		}
		for media := range ref.Value.Content {
			if media != mediaJSON && !strings.HasSuffix(media, "+json") {
				return true
			}
		}
	}
	return false
}

// responseStatus reads a response key other than default: a status code from 100 to 599 in
// three digits, or a status range from 1XX to 5XX in any case (isRange, code 0). ok is false for
// every other key.
func responseStatus(key string) (code int, isRange, ok bool) {
	if len(key) == 3 && key[0] >= '1' && key[0] <= '5' && strings.EqualFold(key[1:], "XX") {
		return 0, true, true
	}
	code, err := strconv.Atoi(key)
	if err != nil || code < 100 || code > 599 || len(key) != 3 {
		return 0, false, false
	}
	return code, false, true
}

// declaresETag reports whether response headers include ETag (names are case-insensitive,
// RFC 9110).
func declaresETag(headers openapi3.Headers) bool {
	for name := range headers {
		if strings.EqualFold(name, "ETag") {
			return true
		}
	}
	return false
}

// pathParam is the parameter syntax oapi-codegen v2.8.0 recognises in paths
// (pkg/codegen/utils.go, pathParamRE).
var pathParam = regexp.MustCompile(`{[.;?]?([^{}*]+)\*?}`)

// pattern returns the net/http pattern under which the generated std-http server registers the
// operation, following oapi-codegen v2.8.0 exactly: upper-case method, one space, base URL and
// path, each path parameter reduced to its name with every rune that is not valid in a Go
// identifier replaced by "_" (SanitizeGoIdentifier), and "{$}" after a trailing slash
// (SwaggerUriToStdHttpUri). routes.Verify compares the patterns at start, so a drift here
// stops the server instead of mislabelling a route.
//
// Example: pattern("get", "/api/v1", "/threads/{thread_id}") is "GET /api/v1/threads/{thread_id}".
func pattern(method, base, path string) string {
	uri := pathParam.ReplaceAllStringFunc(path, func(m string) string {
		return "{" + sanitizeIdentifier(pathParam.FindStringSubmatch(m)[1]) + "}"
	})
	if strings.HasSuffix(uri, "/") {
		uri += "{$}"
	}
	return strings.ToUpper(method) + " " + base + uri
}

// sanitizeIdentifier replaces every rune that cannot appear at its place in a Go identifier with
// "_", as oapi-codegen's SanitizeGoIdentifier does: letters, digits and "_", no digit first.
func sanitizeIdentifier(s string) string {
	out := []rune(s)
	for i, r := range out {
		valid := unicode.IsLetter(r) || r == '_' || unicode.IsNumber(r)
		if !valid || (i == 0 && unicode.IsNumber(r)) {
			out[i] = '_'
		}
	}
	return string(out)
}

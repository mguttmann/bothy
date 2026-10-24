// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/OWNER/bothy/services/server/api/restclient"
)

// bothyAPI is the part of the generated client (api/restclient) that the tool calls; the client of
// bothyapi (DPoP transport over the session file) satisfies it.
type bothyAPI interface {
	GetMeWithResponse(ctx context.Context, reqEditors ...restclient.RequestEditorFn) (*restclient.GetMeResponse, error)
	GetInstanceWithResponse(ctx context.Context, reqEditors ...restclient.RequestEditorFn) (*restclient.GetInstanceResponse, error)
	ListUsersWithResponse(ctx context.Context, params *restclient.ListUsersParams, reqEditors ...restclient.RequestEditorFn) (*restclient.ListUsersResponse, error)
	ListAgentsWithResponse(ctx context.Context, params *restclient.ListAgentsParams, reqEditors ...restclient.RequestEditorFn) (*restclient.ListAgentsResponse, error)
	ListSpacesWithResponse(ctx context.Context, reqEditors ...restclient.RequestEditorFn) (*restclient.ListSpacesResponse, error)
	ListSpaceChannelsWithResponse(ctx context.Context, spaceID restclient.SpaceIdPath, reqEditors ...restclient.RequestEditorFn) (*restclient.ListSpaceChannelsResponse, error)
	ListImportsWithResponse(ctx context.Context, params *restclient.ListImportsParams, reqEditors ...restclient.RequestEditorFn) (*restclient.ListImportsResponse, error)
	GetImportWithResponse(ctx context.Context, importID restclient.ImportIdPath, reqEditors ...restclient.RequestEditorFn) (*restclient.GetImportResponse, error)
	ListImportItemsWithResponse(ctx context.Context, importID restclient.ImportIdPath, params *restclient.ListImportItemsParams, reqEditors ...restclient.RequestEditorFn) (*restclient.ListImportItemsResponse, error)
	ApplyImportBatchWithResponse(ctx context.Context, importID restclient.ImportIdPath, body restclient.ApplyImportBatchJSONRequestBody, reqEditors ...restclient.RequestEditorFn) (*restclient.ApplyImportBatchResponse, error)
	CreateImportFileWithResponse(ctx context.Context, importID restclient.ImportIdPath, body restclient.CreateImportFileJSONRequestBody, reqEditors ...restclient.RequestEditorFn) (*restclient.CreateImportFileResponse, error)
	UploadImportFilePartWithBodyWithResponse(ctx context.Context, importID restclient.ImportIdPath, fileID restclient.FileIdPath, partNumber restclient.PartNumberPath, params *restclient.UploadImportFilePartParams, contentType string, body io.Reader, reqEditors ...restclient.RequestEditorFn) (*restclient.UploadImportFilePartResponse, error)
	CompleteImportFileWithResponse(ctx context.Context, importID restclient.ImportIdPath, fileID restclient.FileIdPath, body restclient.CompleteImportFileJSONRequestBody, reqEditors ...restclient.RequestEditorFn) (*restclient.CompleteImportFileResponse, error)
	SetImportCursorWithResponse(ctx context.Context, importID restclient.ImportIdPath, body restclient.SetImportCursorJSONRequestBody, reqEditors ...restclient.RequestEditorFn) (*restclient.SetImportCursorResponse, error)
	GetChannelWithResponse(ctx context.Context, channelID restclient.ChannelIdPath, reqEditors ...restclient.RequestEditorFn) (*restclient.GetChannelResponse, error)
	ListChannelThreadsWithResponse(ctx context.Context, channelID restclient.ChannelIdPath, params *restclient.ListChannelThreadsParams, reqEditors ...restclient.RequestEditorFn) (*restclient.ListChannelThreadsResponse, error)
	ListThreadMessagesWithResponse(ctx context.Context, threadID restclient.ThreadIdPath, params *restclient.ListThreadMessagesParams, reqEditors ...restclient.RequestEditorFn) (*restclient.ListThreadMessagesResponse, error)
}

// The reasons of 409 conflict the run reacts to (22 8).
const (
	reasonNotRunning = "import_not_running"
)

// problemToken is the form of a problem code or reason the tool repeats in an error text; anything
// else is dropped, so a body can never bring content into the output.
var problemToken = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// APIError is a call to the server that ended with a status the run does not accept. Its text names the
// operation, the status, the problem code and the reason, never a body, a header or an ID.
type APIError struct {
	// Op is the operationId of the call.
	Op string
	// Status is the HTTP status.
	Status int
	// Code is the problem code ("" without one).
	Code string
	// Reason is the reason of a 409 or 403 ("" without one).
	Reason string
}

// Error returns "bothy: <op>: status <n> [<code>] [(<reason>)]".
func (e *APIError) Error() string {
	s := "bothy: " + e.Op + ": status " + strconv.Itoa(e.Status)
	if e.Code != "" {
		s += " " + e.Code
	}
	if e.Reason != "" {
		s += " (" + e.Reason + ")"
	}
	return s
}

// apiError builds the APIError of a failed call from its response and body.
func apiError(op string, resp *http.Response, body []byte) error {
	e := &APIError{Op: op}
	if resp != nil {
		e.Status = resp.StatusCode
	}
	var p struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	if json.Unmarshal(body, &p) == nil {
		if problemToken.MatchString(p.Code) {
			e.Code = p.Code
		}
		if problemToken.MatchString(p.Reason) {
			e.Reason = p.Reason
		}
	}
	return e
}

// bothyReads are the read calls of a run, each following the pages of its list.
type bothyReads struct {
	api bothyAPI
}

// person is an active or disabled person or agent of the server by handle.
type person struct {
	id     restclient.Uuid
	active bool
}

// users returns every person of the server by handle (listUsers).
func (r bothyReads) users(ctx context.Context) (map[string]person, error) {
	out := map[string]person{}
	var cursor *restclient.Cursor
	for {
		resp, err := r.api.ListUsersWithResponse(ctx, &restclient.ListUsersParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return nil, apiError("listUsers", resp.HTTPResponse, resp.Body)
		}
		for _, u := range resp.JSON200.Items {
			out[u.Handle] = person{id: u.Id, active: u.Status == restclient.UserStatusActive}
		}
		if cursor = resp.JSON200.NextCursor; cursor == nil || *cursor == "" {
			return out, nil
		}
	}
}

// agents returns every agent of the server by handle (listAgents).
func (r bothyReads) agents(ctx context.Context) (map[string]person, error) {
	out := map[string]person{}
	var cursor *restclient.Cursor
	for {
		resp, err := r.api.ListAgentsWithResponse(ctx, &restclient.ListAgentsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return nil, apiError("listAgents", resp.HTTPResponse, resp.Body)
		}
		for _, a := range resp.JSON200.Items {
			out[a.Handle] = person{id: a.Id, active: a.Status == restclient.AgentStatusActive}
		}
		if cursor = resp.JSON200.NextCursor; cursor == nil || *cursor == "" {
			return out, nil
		}
	}
}

// spaces returns the live spaces the owner sees (listSpaces).
func (r bothyReads) spaces(ctx context.Context) ([]restclient.Space, error) {
	resp, err := r.api.ListSpacesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, apiError("listSpaces", resp.HTTPResponse, resp.Body)
	}
	out := make([]restclient.Space, 0, len(resp.JSON200.Items))
	for _, v := range resp.JSON200.Items {
		out = append(out, v.Space)
	}
	return out, nil
}

// channels returns the live channels of a space (listSpaceChannels).
func (r bothyReads) channels(ctx context.Context, space restclient.Uuid) ([]restclient.Channel, error) {
	resp, err := r.api.ListSpaceChannelsWithResponse(ctx, space)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, apiError("listSpaceChannels", resp.HTTPResponse, resp.Body)
	}
	out := make([]restclient.Channel, 0, len(resp.JSON200.Items))
	for _, v := range resp.JSON200.Items {
		out = append(out, v.Channel)
	}
	return out, nil
}

// sources returns every import source (listImports).
func (r bothyReads) sources(ctx context.Context) ([]restclient.ImportSource, error) {
	var out []restclient.ImportSource
	var cursor *restclient.Cursor
	for {
		resp, err := r.api.ListImportsWithResponse(ctx, &restclient.ListImportsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return nil, apiError("listImports", resp.HTTPResponse, resp.Body)
		}
		out = append(out, resp.JSON200.Items...)
		if cursor = resp.JSON200.NextCursor; cursor == nil || *cursor == "" {
			return out, nil
		}
	}
}

// source returns one import source with its cursor (getImport).
func (r bothyReads) source(ctx context.Context, id restclient.Uuid) (*restclient.ImportSource, error) {
	resp, err := r.api.GetImportWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, apiError("getImport", resp.HTTPResponse, resp.Body)
	}
	return resp.JSON200, nil
}

// posted returns every posted item of a source by origin key (listImportItems with state
// posted).
func (r bothyReads) posted(ctx context.Context, id restclient.Uuid) (map[string]restclient.ImportItem, error) {
	out := map[string]restclient.ImportItem{}
	var cursor *restclient.Cursor
	state := restclient.ImportItemStatePosted
	params := &restclient.ListImportItemsParams{State: &state}
	for {
		params.Cursor = cursor
		resp, err := r.api.ListImportItemsWithResponse(ctx, id, params)
		if err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return nil, apiError("listImportItems", resp.HTTPResponse, resp.Body)
		}
		for _, it := range resp.JSON200.Items {
			out[it.OriginKey] = it
		}
		if cursor = resp.JSON200.NextCursor; cursor == nil || *cursor == "" {
			return out, nil
		}
	}
}

// channel returns a channel of the server (getChannel).
func (r bothyReads) channel(ctx context.Context, id restclient.Uuid) (*restclient.Channel, error) {
	resp, err := r.api.GetChannelWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, apiError("getChannel", resp.HTTPResponse, resp.Body)
	}
	return &resp.JSON200.Channel, nil
}

// threads returns the threads of a channel without its main thread (listChannelThreads, every
// page).
func (r bothyReads) threads(ctx context.Context, channel restclient.Uuid) ([]restclient.Thread, error) {
	var out []restclient.Thread
	var cursor *restclient.Cursor
	for {
		resp, err := r.api.ListChannelThreadsWithResponse(ctx, channel, &restclient.ListChannelThreadsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return nil, apiError("listChannelThreads", resp.HTTPResponse, resp.Body)
		}
		out = append(out, resp.JSON200.Items...)
		if cursor = resp.JSON200.NextCursor; cursor == nil || *cursor == "" {
			return out, nil
		}
	}
}

// eachMessage calls fn for every message of a thread in thread_seq order (listThreadMessages from
// after_seq 0, page by page).
func (r bothyReads) eachMessage(ctx context.Context, thread restclient.Uuid, fn func(restclient.Message)) error {
	after := int64(0)
	for {
		resp, err := r.api.ListThreadMessagesWithResponse(ctx, thread, &restclient.ListThreadMessagesParams{AfterSeq: &after})
		if err != nil {
			return err
		}
		if resp.JSON200 == nil {
			return apiError("listThreadMessages", resp.HTTPResponse, resp.Body)
		}
		for _, m := range resp.JSON200.Items {
			fn(m)
			after = max(after, m.ThreadSeq)
		}
		if !resp.JSON200.HasMoreAfter || len(resp.JSON200.Items) == 0 {
			return nil
		}
	}
}

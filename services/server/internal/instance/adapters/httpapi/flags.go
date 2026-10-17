// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// ListFeatureFlags implements rest.StrictServerInterface (GET /feature-flags, owner; M1-47).
func (h *Admin) ListFeatureFlags(ctx context.Context, _ rest.ListFeatureFlagsRequestObject) (rest.ListFeatureFlagsResponseObject, error) {
	list, err := h.admin.ListFlags(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	out := rest.FeatureFlagList{Items: make([]rest.FeatureFlag, 0, len(list))}
	for _, f := range list {
		out.Items = append(out.Items, toRESTFlag(f))
	}
	return rest.ListFeatureFlags200JSONResponse{Body: out}, nil
}

// PutFeatureFlag implements rest.StrictServerInterface (PUT /feature-flags/{key}, owner with
// step-up, optional If-Match; M1-47). An absent rollout means an empty one (PUT replaces).
func (h *Admin) PutFeatureFlag(ctx context.Context, req rest.PutFeatureFlagRequestObject) (rest.PutFeatureFlagResponseObject, error) {
	cmd := instance.PutFlagCmd{Key: req.Key, IfMatch: ifMatch(req.Params.IfMatch)}
	if b := req.Body; b != nil {
		cmd.Enabled, cmd.Reason = b.Enabled, b.Reason
		if b.Rollout != nil {
			cmd.Rollout = instance.FlagRollout{UserIDs: fromUUIDs(b.Rollout.UserIds), GroupIDs: fromUUIDs(b.Rollout.GroupIds)}
		}
	}
	f, err := h.admin.PutFlag(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, flagCurrentForAPI(err)
	}
	return rest.PutFeatureFlag200JSONResponse{Body: toRESTFlag(f), Headers: rest.PutFeatureFlag200ResponseHeaders{ETag: ptr(httpserver.FormatETag(f.Version))}}, nil
}

// flagCurrentForAPI replaces the current flag of a version_conflict with its API form.
func flagCurrentForAPI(err error) error {
	if e, ok := errs.As(err); ok {
		if cur, ok := e.Current.(instance.FeatureFlag); ok {
			e.Current = toRESTFlag(cur)
		}
	}
	return err
}

// toRESTFlag converts a flag (schema FeatureFlag): an orphaned row has no kind, description,
// default, module or remove_by; a flag without a row has no updated_by or updated_at; an empty
// rollout is {}.
func toRESTFlag(f instance.FeatureFlag) rest.FeatureFlag {
	out := rest.FeatureFlag{
		Key: f.Key, Orphaned: f.Orphaned, Enabled: f.Enabled, Version: f.Version,
		Rollout: rest.FlagRollout{UserIds: toUUIDs(f.Rollout.UserIDs), GroupIds: toUUIDs(f.Rollout.GroupIDs)},
	}
	if !f.Orphaned {
		kind := rest.FeatureFlagKind(f.Kind)
		out.Kind, out.Description, out.Default, out.Module = &kind, ptr(f.Description), ptr(f.Default), ptr(f.Module)
		if f.RemoveBy != "" {
			out.RemoveBy = ptr(f.RemoveBy)
		}
	}
	if !f.UpdatedBy.IsZero() {
		id := rest.Uuid(f.UpdatedBy)
		out.UpdatedBy = &id
	}
	if !f.UpdatedAt.IsZero() {
		t := f.UpdatedAt.UTC().Truncate(time.Millisecond)
		out.UpdatedAt = &t
	}
	return out
}

// toUUIDs converts IDs; nil (the member is omitted) for none.
func toUUIDs(list []ids.ID) *[]rest.Uuid {
	if len(list) == 0 {
		return nil
	}
	out := make([]rest.Uuid, 0, len(list))
	for _, id := range list {
		out = append(out, rest.Uuid(id))
	}
	return &out
}

// fromUUIDs converts the IDs of a request in their order; nil for an absent member.
func fromUUIDs(list *[]rest.Uuid) []ids.ID {
	if list == nil {
		return nil
	}
	out := make([]ids.ID, 0, len(*list))
	for _, id := range *list {
		out = append(out, ids.ID(id))
	}
	return out
}

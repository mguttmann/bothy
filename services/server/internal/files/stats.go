// SPDX-License-Identifier: Apache-2.0

package files

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The metric names of the statistics source of files (docs/observability/admin-statistics.md
// section 2, M6-05).
const (
	// MetricStorageBytes is the current sum of size_bytes of the complete, not deleted
	// attachments of a group.
	MetricStorageBytes = "storage_bytes"
	// MetricScanErrors and MetricScanInfected count the files whose scan ended with error or
	// infected in a bucket of scanned_at.
	MetricScanErrors   = "scan_errors"
	MetricScanInfected = "scan_infected"
)

// StatsValue is one aggregate: the value of Metric in the bucket starting at Bucket (zero for a
// current value) for the group Group (a space ID as text, "" for the instance). It never names a
// person, a file or carries content.
type StatsValue struct {
	Bucket time.Time
	Group  string
	Metric string
	Value  float64
}

// StatsSource is the statistics port of files (M6-05). It never authorizes: the stats use cases
// authorize first; StorageBytes reads only the passed channels, ScanStats has no channel and the
// stats use case calls it only for stats.read on the instance by an owner.
type StatsSource interface {
	// StorageBytes returns MetricStorageBytes per group: channelIDs are the only channels read and
	// groupIDs is parallel to it (the space of each channel). An empty set reads nothing.
	//
	// Example:
	//
	//	vs, err := src.StorageBytes(ctx, chans, spaces)
	StorageBytes(ctx context.Context, channelIDs, groupIDs []ids.ID) ([]StatsValue, error)
	// ScanStats returns MetricScanErrors and MetricScanInfected of every file per bucket of
	// scanned_at in the half-open range from, to, aligned to granularity (hour, day, week or
	// month) in the IANA time zone tz. Errors: validation_failed for an unknown granularity or
	// time zone, a database error.
	ScanStats(ctx context.Context, from, to time.Time, granularity, tz string) ([]StatsValue, error)
}

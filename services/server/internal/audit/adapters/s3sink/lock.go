// SPDX-License-Identifier: Apache-2.0

package s3sink

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// The lengths of the retention units of S3: a day is 24 hours and a year 365 days, the
// shortest calendar year, so a lock in years is never counted longer than it holds.
const (
	day  = 24 * time.Hour
	year = 365 * day
)

// ErrNotImmutable is wrapped by CheckLock when the bucket has no effective retention lock: the
// checkpoints in it could be deleted or replaced, so the target is refused (19 SEC-AUD-2,
// runbook backup-restore 8).
var ErrNotImmutable = errors.New(keySink + ": the bucket is not immutable")

// CheckLock reads the Object Lock configuration of the bucket and returns nil only when Object
// Lock is enabled with a default retention in mode COMPLIANCE whose period is at least minimum
// (retention.audit). GOVERNANCE fails, because a user with the permission
// s3:BypassGovernanceRetention can lift it (runbook backup-restore 8). bothy config check and the
// server start call it (docs/DECISIONS.md M7-22/5).
//
// Errors: ErrNotImmutable with the reason, naming audit.checkpoint_sink (and retention.audit for
// a short period); storage failures named like those of Put, never the endpoint or credentials.
//
// Example:
//
//	err := sink.CheckLock(ctx, 365*24*time.Hour)
func (s *Sink) CheckLock(ctx context.Context, minimum time.Duration) error {
	out, err := s.api.GetObjectLockConfiguration(ctx, &awss3.GetObjectLockConfigurationInput{Bucket: aws.String(s.bucket)})
	if err != nil {
		if code, _ := apiStatus(err); code == "ObjectLockConfigurationNotFoundError" {
			return notImmutable("Object Lock is not enabled on the bucket")
		}
		return opError("get object lock configuration", err)
	}
	cfg := out.ObjectLockConfiguration
	if cfg == nil || cfg.ObjectLockEnabled != types.ObjectLockEnabledEnabled {
		return notImmutable("Object Lock is not enabled on the bucket")
	}
	if cfg.Rule == nil || cfg.Rule.DefaultRetention == nil {
		return notImmutable("the bucket has no default retention")
	}
	r := cfg.Rule.DefaultRetention
	if r.Mode != types.ObjectLockRetentionModeCompliance {
		return notImmutable("the default retention is not in mode COMPLIANCE")
	}
	period := time.Duration(aws.ToInt32(r.Days))*day + time.Duration(aws.ToInt32(r.Years))*year
	if period < minimum {
		return notImmutable("the default retention is shorter than " + keyRetention)
	}
	return nil
}

// notImmutable is ErrNotImmutable with its reason.
func notImmutable(reason string) error {
	return &lockError{reason: reason}
}

// lockError is a refused lock configuration.
type lockError struct{ reason string }

func (e *lockError) Error() string {
	return ErrNotImmutable.Error() + ": " + e.reason + "; it needs Object Lock with a default retention in mode COMPLIANCE of at least " + keyRetention
}

func (e *lockError) Is(target error) bool { return target == ErrNotImmutable }

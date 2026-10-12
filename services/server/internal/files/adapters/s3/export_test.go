// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"context"
	"net/http"
	"reflect"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// ObjectName is objectName for the tests of the key mapping.
var ObjectName = objectName

// CoreAPI returns the type of the narrow core interface the driver holds.
func CoreAPI() reflect.Type { return reflect.TypeFor[coreAPI]() }

// PresignAPI returns the type of the narrow presign interface the driver holds.
func PresignAPI() reflect.Type { return reflect.TypeFor[presignAPI]() }

// ChecksumModes is checksumModes for TestChecksumModeMapping.
func ChecksumModes(mode string) (aws.RequestChecksumCalculation, aws.ResponseChecksumValidation, error) {
	return checksumModes(mode)
}

// Recorded is one request the SDK handed to the HTTP client of the driver.
type Recorded struct {
	Method   string
	Path     string
	RawQuery string
	Header   http.Header
}

// Recorder sees every request of a store built with NewRecorded before the egress client does.
type Recorder struct {
	mu   sync.Mutex
	reqs []Recorded
}

// Requests returns a copy of the recorded requests in order.
func (r *Recorder) Requests() []Recorded {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Recorded(nil), r.reqs...)
}

// Reset forgets the recorded requests.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = nil
}

func (r *Recorder) add(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, Recorded{Method: req.Method, Path: req.URL.EscapedPath(), RawQuery: req.URL.RawQuery, Header: req.Header.Clone()})
}

// recordingClient records every request and hands it to the client of the driver.
type recordingClient struct {
	next httpDoer
	rec  *Recorder
}

func (c recordingClient) Do(req *http.Request) (*http.Response, error) {
	c.rec.add(req)
	return c.next.Do(req)
}

// NewRecorded is New with rec wrapped around the HTTP client of the driver.
func NewRecorded(ctx context.Context, cfg Config, deps Deps, rec *Recorder) (*Store, error) {
	return newStore(ctx, cfg, deps, func(next httpDoer) httpDoer { return recordingClient{next: next, rec: rec} })
}

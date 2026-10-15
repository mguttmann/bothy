// SPDX-License-Identifier: Apache-2.0

package s3sink

import (
	"context"
	"reflect"
	"sync/atomic"
	"time"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// SetPageSize sets the page size of List, so the conformance suite pages with few objects.
func SetPageSize(s *Sink, n int32) { s.pageSize = n }

// APIMethods returns the method names of the interface the sink holds.
func APIMethods() []string {
	t := reflect.TypeFor[objectAPI]()
	out := make([]string, t.NumMethod())
	for i := range out {
		out[i] = t.Method(i).Name
	}
	return out
}

// CountingAPI wraps the API of s: every GetObject is counted and delayed by delay, as a remote
// object store answers.
func CountingAPI(s *Sink, delay time.Duration) *atomic.Int64 {
	n := &atomic.Int64{}
	s.api = slowAPI{objectAPI: s.api, gets: n, delay: delay}
	return n
}

type slowAPI struct {
	objectAPI
	gets  *atomic.Int64
	delay time.Duration
}

func (a slowAPI) GetObject(ctx context.Context, in *awss3.GetObjectInput, opts ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	a.gets.Add(1)
	time.Sleep(a.delay)
	return a.objectAPI.GetObject(ctx, in, opts...)
}

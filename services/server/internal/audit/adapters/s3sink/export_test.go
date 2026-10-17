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

// GetCounters counts the GetObject calls of a CountingAPI and the most that ran at once.
type GetCounters struct {
	Gets, Peak atomic.Int64
	inFlight   atomic.Int64
}

// ReadParallel is the bound of concurrent GETs of one List.
const ReadParallel = readParallel

// CountingAPI wraps the API of s: every GetObject is counted and delayed by delay, as a remote
// object store answers, and the peak of concurrent calls is kept.
func CountingAPI(s *Sink, delay time.Duration) *GetCounters {
	c := &GetCounters{}
	s.api = slowAPI{objectAPI: s.api, c: c, delay: delay}
	return c
}

type slowAPI struct {
	objectAPI
	c     *GetCounters
	delay time.Duration
}

func (a slowAPI) GetObject(ctx context.Context, in *awss3.GetObjectInput, opts ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	a.c.Gets.Add(1)
	now := a.c.inFlight.Add(1)
	defer a.c.inFlight.Add(-1)
	for {
		peak := a.c.Peak.Load()
		if now <= peak || a.c.Peak.CompareAndSwap(peak, now) {
			break
		}
	}
	time.Sleep(a.delay)
	return a.objectAPI.GetObject(ctx, in, opts...)
}

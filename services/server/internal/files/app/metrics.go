// SPDX-License-Identifier: Apache-2.0

package app

import "context"

// Metrics records bothy.file.upload.size (signals.md 4.4); adapters/otelmetrics implements it.
type Metrics interface {
	// UploadCompleted records the declared size of one completed upload.
	UploadCompleted(ctx context.Context, sizeBytes int64)
}

// noMetrics records nothing.
type noMetrics struct{}

func (noMetrics) UploadCompleted(context.Context, int64) {
	// Without a meter provider nothing is recorded.
}

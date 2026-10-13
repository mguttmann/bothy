// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// maxQueueWorkers is River's upper bound for the parallelism of one queue.
const maxQueueWorkers = "10000"

// jobsConfigKeys returns the keys of the background jobs from docs/spec/02-architecture.md
// "Configuration values of this chapter" (M1-14): the schema of River's tables and the
// parallelism of the five queues of 02 "Queues". The cross-check jobs_schema
// (configchecks.go) holds the schema name to an unquoted identifier.
func jobsConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "jobs.schema", Kind: config.KindString, Default: "jobs",
			Doc:     "PostgreSQL schema of River's job tables, separate from the domain tables.",
			Chapter: chapter02,
		},
		queueKey("default", "8", "Parallel workers of the queue default for short domain jobs."),
		queueKey("files", "2", "Parallel workers of the queue files for scans, thumbnails and transcription."),
		queueKey("delivery", "16", "Parallel workers of the queue delivery for push and webhooks."),
		queueKey("maintenance", "2", "Parallel workers of the queue maintenance for cleanup, partitions and checks."),
		queueKey("privacy", "1", "Parallel workers of the queue privacy for export and erasure."),
	}
}

// queueKey is the key jobs.queue.<queue>.max_workers.
func queueKey(queue, def, doc string) config.Key {
	return config.Key{
		Name: "jobs.queue." + queue + ".max_workers", Kind: config.KindInt, Default: def, Min: "1", Max: maxQueueWorkers,
		Doc: doc, Chapter: chapter02,
	}
}

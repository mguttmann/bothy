// SPDX-License-Identifier: Apache-2.0

package ratelimit

// Tracked reports whether the limiter holds a bucket for class and key; only tests use it.
func (l *Limiter) Tracked(class Class, key string) bool { return l.tracked(class, key) }

// ShardOf returns the index of the shard that holds the bucket of class and key; only tests use it.
func (l *Limiter) ShardOf(class Class, key string) int {
	s := l.shard(internalKey(class, key))
	for i := range l.shards {
		if &l.shards[i] == s {
			return i
		}
	}
	return -1
}

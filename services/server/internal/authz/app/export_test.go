// SPDX-License-Identifier: Apache-2.0

package app

// CacheLen returns the number of cached snapshots (tests of the LRU bound).
func (a *Authorizer) CacheLen() int { return a.cache.len() }

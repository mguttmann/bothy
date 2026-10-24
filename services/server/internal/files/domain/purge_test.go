// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestObjectKeysOf: the originals up to revision r+1 and the four variants of the file, and an
// error for every key that is not an original key of the same file (brief M3-08 design note 4 a).
func TestObjectKeysOf(t *testing.T) {
	id := ids.MustParse(exampleID)
	dir := "f/5a/6b/" + exampleID + "/"
	variants := []string{dir + "small", dir + "medium", dir + "large", dir + "poster"}
	for _, tc := range []struct {
		rev  int
		want []string
	}{
		{1, append([]string{dir + "orig-1", dir + "orig-2"}, variants...)},
		{2, append([]string{dir + "orig-1", dir + "orig-2", dir + "orig-3"}, variants...)},
	} {
		key, _ := domain.OriginalKey(id, tc.rev)
		got, err := domain.ObjectKeysOf(id, key)
		if err != nil {
			t.Fatalf("revision %d: %v", tc.rev, err)
		}
		if !slices.Equal(got, tc.want) {
			t.Fatalf("revision %d: got %v, want %v", tc.rev, got, tc.want)
		}
	}
	other := ids.MustParse("0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f0000")
	otherKey, _ := domain.OriginalKey(other, 1)
	variant, _ := domain.VariantKey(id, domain.VariantSmall)
	part, _ := domain.UploadPartKey(id, 1)
	for name, key := range map[string]string{"another file": otherKey, "variant": variant, "part": part, "malformed": "f/x", "empty": ""} {
		if keys, err := domain.ObjectKeysOf(id, key); !errors.Is(err, domain.ErrInvalidKey) || keys != nil {
			t.Fatalf("%s: got %v, %v; want ErrInvalidKey and no keys", name, keys, err)
		}
	}
}

// FuzzObjectKeysOf is the property of 12 15 row "Key schema" for the purge: every returned key
// passes ValidateKey, carries the file ID and contains no other ID.
func FuzzObjectKeysOf(f *testing.F) {
	for _, seed := range []struct {
		seed int64
		rev  int
	}{{1, 1}, {2, 2}, {3, 9}, {4, 120}} {
		f.Add(seed.seed, seed.rev)
	}
	f.Fuzz(func(t *testing.T, seed int64, rev int) {
		if rev < 1 || rev > 1<<16 {
			return // the contract: a stored original revision is at least 1; bound the key count
		}
		id := randomID(rand.New(rand.NewSource(seed))) //nolint:gosec // reproducible test input
		key, err := domain.OriginalKey(id, rev)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		keys, err := domain.ObjectKeysOf(id, key)
		if err != nil {
			t.Fatalf("seed %d revision %d: %v", seed, rev, err)
		}
		if len(keys) != rev+1+4 {
			t.Fatalf("seed %d revision %d: %d keys", seed, rev, len(keys))
		}
		for _, k := range keys {
			if err := domain.ValidateKey(k); err != nil {
				t.Fatalf("seed %d: key %q: %v", seed, k, err)
			}
			if !strings.Contains(k, id.String()) {
				t.Fatalf("seed %d: key %q without the file id", seed, k)
			}
			for _, m := range uuidPattern.FindAllString(k, -1) {
				if m != id.String() {
					t.Fatalf("seed %d: key %q holds another id %q", seed, k, m)
				}
			}
		}
	})
}

// TestPlanPurge: no row nothing, a deleted row objects and row in every state, a failed or
// blocked file its objects only, a live file nothing (design note 4 b).
func TestPlanPurge(t *testing.T) {
	states := [][2]string{
		{domain.UploadPending, domain.ScanPending}, {domain.UploadComplete, domain.ScanClean}, {domain.UploadComplete, domain.ScanSkipped},
		{domain.UploadFailed, domain.ScanPending}, {domain.UploadComplete, domain.ScanInfected}, {domain.UploadComplete, domain.ScanError},
		{domain.UploadPending, domain.ScanClean},
	}
	if got := domain.PlanPurge(domain.PurgeFacts{}); got != domain.PurgeNothing {
		t.Fatalf("no row: %v", got)
	}
	for _, st := range states {
		if got := domain.PlanPurge(domain.PurgeFacts{Exists: true, Deleted: true, UploadState: st[0], ScanStatus: st[1]}); got != domain.PurgeObjectsAndRow {
			t.Fatalf("deleted %v: %v", st, got)
		}
	}
	for st, want := range map[[2]string]domain.PurgePlan{
		{domain.UploadFailed, domain.ScanPending}:    domain.PurgeObjects,
		{domain.UploadComplete, domain.ScanInfected}: domain.PurgeObjects,
		{domain.UploadComplete, domain.ScanError}:    domain.PurgeObjects,
		{domain.UploadPending, domain.ScanPending}:   domain.PurgeNothing,
		{domain.UploadPending, domain.ScanClean}:     domain.PurgeNothing,
		{domain.UploadComplete, domain.ScanClean}:    domain.PurgeNothing,
		{domain.UploadComplete, domain.ScanSkipped}:  domain.PurgeNothing,
	} {
		if got := domain.PlanPurge(domain.PurgeFacts{Exists: true, UploadState: st[0], ScanStatus: st[1]}); got != want {
			t.Fatalf("%v: got %v, want %v", st, got, want)
		}
	}
}

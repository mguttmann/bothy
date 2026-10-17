// SPDX-License-Identifier: Apache-2.0

//go:build platform

// Command bighome creates a synthetic home for the measurement of the entry scan of HD11: files
// and folders mixed, up to twelve levels deep (brief M2-42 design note 19).
//
// Example:
//
//	go run -tags platform ./test/platform/bighome -dir /home/bothy-s-alex/bighome -entries 1000000
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func main() {
	dir := flag.String("dir", "", "the folder to fill")
	entries := flag.Int("entries", 1000000, "number of entries")
	depth := flag.Int("depth", 12, "maximum depth")
	flag.Parse()
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "bighome: -dir is required")
		os.Exit(2)
	}
	n := 0
	var fill func(p string, level int)
	fill = func(p string, level int) {
		for i := 0; i < 10 && n < *entries; i++ {
			name := filepath.Join(p, "e"+strconv.Itoa(i))
			n++
			if i%3 == 0 && level < *depth {
				if err := os.Mkdir(name, 0o700); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				fill(name, level+1)
				continue
			}
			if err := os.WriteFile(name, nil, 0o600); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	}
	if err := os.MkdirAll(*dir, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for n < *entries {
		sub := filepath.Join(*dir, "r"+strconv.Itoa(n))
		if err := os.Mkdir(sub, 0o700); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		n++
		fill(sub, 1)
	}
}

// SPDX-License-Identifier: Apache-2.0

// Package txn is the kernel port of the fixture.
package txn

// Runner runs a unit of work.
type Runner interface {
	Run(fn func() error) error
}

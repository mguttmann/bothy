// SPDX-License-Identifier: Apache-2.0

package app

// SetStrict switches the panic on an invalid key of r on or off, so a test of this package can
// show the production behaviour (a counted drop) inside a test binary.
func SetStrict(r *Measurement, on bool) { r.strict = on }

// Take swaps the counters of r out, as a flush does.
func Take(r *Measurement) []MinuteRow { return r.take() }

// FlushChunkRows is the chunk size of a flush.
const FlushChunkRows = flushChunkRows

// SPDX-License-Identifier: Apache-2.0

// Package stats is the port of the stats module: statistics for the console and apps (usage per
// space, runs, tool calls, costs, error rates), the own measurement (in-memory counters, flush,
// retention), the SLO traffic light, the live frame stats.live and the timeline. Other modules
// import only this package (docs/spec/02-architecture.md, "Layers within a module").
package stats

// SPDX-License-Identifier: Apache-2.0

package report

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Mode is the measurement mode of a host against perf.reference_profile (ADR 0063 point 3).
type Mode string

// The modes: match (exactly the CPUs of the profile, at least its memory), emulated (a larger
// host, limited to the profile) and below (a smaller host). Only match and emulated can end green.
const (
	ModeMatch    Mode = "match"
	ModeEmulated Mode = "emulated"
	ModeBelow    Mode = "below"
)

// Reference is the CPU count and memory of perf.reference_profile.
type Reference struct {
	CPUs        int
	MemoryBytes int64
}

var (
	cpuPattern = regexp.MustCompile(`(\d+)\s*vCPU`)
	memPattern = regexp.MustCompile(`(\d+)\s*(GiB|MiB)\s*RAM`)
)

// ParseReference reads "<n> vCPU" and "<m> GiB RAM" (or MiB) from the text of
// perf.reference_profile. The error reports a text without both.
func ParseReference(text string) (Reference, error) {
	c := cpuPattern.FindStringSubmatch(text)
	m := memPattern.FindStringSubmatch(text)
	if c == nil || m == nil {
		return Reference{}, fmt.Errorf("report: perf.reference_profile %q names no vCPU count and RAM", text)
	}
	cpus, err := strconv.Atoi(c[1])
	if err != nil {
		return Reference{}, fmt.Errorf("report: perf.reference_profile: %w", err)
	}
	mem, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return Reference{}, fmt.Errorf("report: perf.reference_profile: %w", err)
	}
	shift := 30
	if m[2] == "MiB" {
		shift = 20
	}
	return Reference{CPUs: cpus, MemoryBytes: mem << shift}, nil
}

// ModeFor returns the mode of a host with cpus logical CPUs and mem bytes of memory: match with
// exactly the CPUs of ref and at least its memory, emulated with more CPUs and at least its
// memory, below otherwise.
func ModeFor(ref Reference, cpus int, mem int64) Mode {
	switch {
	case mem < ref.MemoryBytes || cpus < ref.CPUs:
		return ModeBelow
	case cpus == ref.CPUs:
		return ModeMatch
	default:
		return ModeEmulated
	}
}

// Runtime is the CPU count and memory of the container runtime that runs the database (the Docker
// VM on macOS, the host itself on Linux); zero values mean unknown.
type Runtime struct {
	CPUs        int
	MemoryBytes int64
}

// ModeWith returns the mode of a host with cpus and mem whose database runs in rt: below when the
// host or a known runtime is below ref (the database could not get the CPUs or memory of the
// profile), the mode of the host otherwise.
func ModeWith(ref Reference, cpus int, mem int64, rt Runtime) Mode {
	if rt.CPUs > 0 && ModeFor(ref, rt.CPUs, rt.MemoryBytes) == ModeBelow {
		return ModeBelow
	}
	return ModeFor(ref, cpus, mem)
}

// Profile is the measured environment stored in results.json. Fsync and SynchronousCommit are
// the values of SHOW of the measured databases ("on" only when every database said "on").
type Profile struct {
	Mode              Mode   `json:"mode"`
	CPUs              int    `json:"cpus"`
	MemoryBytes       int64  `json:"memory_bytes"`
	OS                string `json:"os"`
	PostgresVersion   string `json:"postgres_version"`
	GOMAXPROCS        int    `json:"gomaxprocs"`
	Fsync             string `json:"fsync"`
	SynchronousCommit string `json:"synchronous_commit"`
}

// Durable reports whether fsync and synchronous_commit were both on (ADR 0063 point 2).
func (p Profile) Durable() bool { return p.Fsync == "on" && p.SynchronousCommit == "on" }

// Limits returns the limits a measured server and its database get in mode: in emulated mode the
// CPU count of ref as GOMAXPROCS and container CPUs and the memory of ref for the container;
// otherwise zero values (no limit).
func Limits(mode Mode, ref Reference) (gomaxprocs int, cpus float64, memoryBytes int64) {
	if mode != ModeEmulated {
		return 0, 0, 0
	}
	return ref.CPUs, float64(ref.CPUs), ref.MemoryBytes
}

// Host returns the profile of this host against ref with the database in rt (ModeWith): logical
// CPUs, physical memory, operating system and mode; GOMAXPROCS of the measured server is the CPU
// count of ref in emulated mode and the CPU count of the host otherwise. Database fields stay
// empty until the test fills them.
func Host(ref Reference, rt Runtime) (Profile, error) {
	mem, err := HostMemory()
	if err != nil {
		return Profile{}, err
	}
	cpus := runtime.NumCPU()
	mode := ModeWith(ref, cpus, mem, rt)
	procs, _, _ := Limits(mode, ref)
	if procs == 0 {
		procs = cpus
	}
	return Profile{Mode: mode, CPUs: cpus, MemoryBytes: mem, OS: runtime.GOOS, GOMAXPROCS: procs}, nil
}

// HostMemory returns the physical memory of the host: MemTotal of /proc/meminfo on Linux,
// sysctl hw.memsize on macOS. The error reports another system or an unreadable value.
func HostMemory() (int64, error) {
	switch runtime.GOOS {
	case "linux":
		b, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0, fmt.Errorf("report: %w", err)
		}
		return ParseMeminfo(b)
	case "darwin":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0, fmt.Errorf("report: sysctl hw.memsize: %w", err)
		}
		return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	}
	return 0, fmt.Errorf("report: no memory probe for %s", runtime.GOOS)
}

// ParseMeminfo returns MemTotal of a /proc/meminfo text in bytes.
func ParseMeminfo(b []byte) (int64, error) {
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("report: MemTotal: %w", err)
			}
			return kb << 10, nil
		}
	}
	return 0, errors.New("report: /proc/meminfo has no MemTotal")
}

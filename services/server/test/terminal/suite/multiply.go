// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"fmt"
	"slices"
	"strings"
)

// Multiply returns a copy of sc in which every case drives n sessions at once instead of the one
// session s1 of the file (TestChaosTMT16, Design note 10): the runs of steps of the session
// (device steps, its step-ups, the session, audit and processes checks of it) become one parallel
// step with one lane per session; each lane uses a device of its own (a second enrollment of the
// person, so the step-ups of the lanes never meet), its own marker, and after its start a
// background process that prints continuously. Every other step (a restart of the server, a fault
// of the host, a check of the host) stays one step between the lanes.
func Multiply(sc *Scenario, n int) *Scenario {
	out := *sc
	out.ID = sc.ID
	out.World.Devices = slices.Clone(sc.World.Devices)
	people := map[string]bool{}
	for _, p := range append(slices.Clone(sc.World.People), "owner") {
		people[p] = true
	}
	for k := 1; k <= n; k++ {
		for p := range people {
			out.World.Devices = append(out.World.Devices, DeviceSpec{Name: laneDevice(p, k), Person: p})
		}
	}
	out.Cases = nil
	for _, c := range sc.Cases {
		mc := c
		mc.Steps = nil
		var run []Step
		flush := func() {
			if len(run) == 0 {
				return
			}
			lanes := make([]Step, 0, n)
			for k := 1; k <= n; k++ {
				lane := make([]Step, 0, len(run))
				for _, s := range run {
					lane = append(lane, laneStep(s, k, people))
					if s.Do == "start" && s.Device != "" {
						lane = append(lane, Step{Device: laneDevice(s.Device, k), Do: "send", Session: laneSession(s.Session, k),
							Text: "(while :; do printf '%s\\n' tick; sleep 1; done) & \n"})
					}
				}
				lanes = append(lanes, Step{Sequence: lane})
			}
			mc.Steps = append(mc.Steps, Step{Parallel: lanes})
			run = nil
		}
		for _, s := range c.Steps {
			if ofSession(s) {
				run = append(run, s)
				continue
			}
			flush()
			mc.Steps = append(mc.Steps, s)
		}
		flush()
		out.Cases = append(out.Cases, mc)
	}
	return &out
}

// ofSession reports whether s belongs to the lane of the session: a device step (a step-up
// included), or a check of the session, of its audit entries or of its processes.
func ofSession(s Step) bool {
	switch {
	case s.Device != "" && s.Do != "":
		return true
	case s.Check == "session", s.Check == "audit" && s.Expect != nil && s.Expect.Resource == "s1":
		return true
	case s.Check == "processes" && s.Marker != "":
		return true
	}
	return false
}

// laneStep returns s for lane k: the device of the lane, the session sK and the marker of the
// lane.
func laneStep(s Step, k int, people map[string]bool) Step {
	if people[s.Device] {
		s.Device = laneDevice(s.Device, k)
	}
	if s.Session != "" {
		s.Session = laneSession(s.Session, k)
	}
	s.Text = laneMarker(s.Text, k)
	s.Marker = laneMarker(s.Marker, k)
	if s.Expect != nil {
		e := *s.Expect
		e.Output = laneMarker(e.Output, k)
		if e.Resource != "" {
			e.Resource = laneSession(e.Resource, k)
		}
		s.Expect = &e
	}
	return s
}

// laneDevice, laneSession and laneMarker name the device, the session and the marker of lane k.
func laneDevice(person string, k int) string { return fmt.Sprintf("%s_l%d", person, k) }

func laneSession(session string, k int) string { return fmt.Sprintf("%s_l%d", session, k) }

func laneMarker(s string, k int) string {
	return strings.ReplaceAll(s, "${marker}", fmt.Sprintf("${marker_l%d}", k))
}

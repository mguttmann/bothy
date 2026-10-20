// SPDX-License-Identifier: Apache-2.0

package suite

// fakeHost is M2-44's Fake-Host paired through pairHost and connected through the TLS front, the
// manipulated host service of TM-T11 that never reports idle.
type fakeHost struct {
	id       string
	noiseKey []byte
}

// addFakeHost pairs and connects a Fake-Host of the world.
func (w *instance) addFakeHost(f FakeHostSpec) {
	w.t.Fatalf("%s/%s: the Fake-Host %s is not built yet", w.sc.ID, w.c.Name, f.Name)
}

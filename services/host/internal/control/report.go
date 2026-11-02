// SPDX-License-Identifier: Apache-2.0

package control

import (
	"net"
	"slices"
	gosync "sync"

	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/startup"
)

// ReportSource holds the current report of the host service: the last one startup handed to
// Report. The client sends it as the first frame after every ready and again on every change.
// It is safe for concurrent use.
type ReportSource struct {
	mu     gosync.Mutex
	report startup.Report
	set    bool
}

// Set stores r and reports whether it differs from the stored one.
func (s *ReportSource) Set(r startup.Report) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := !s.set || !sameReport(s.report, r)
	s.report, s.set = r, true
	return changed
}

// Get returns the stored report and whether there is one.
func (s *ReportSource) Get() (startup.Report, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.report, s.set
}

// sameReport compares two reports member by member.
func sameReport(a, b startup.Report) bool {
	return slices.Equal(a.Profiles, b.Profiles) && slices.EqualFunc(a.SessionUsers, b.SessionUsers, func(x, y startupUser) bool {
		return x.OSUser == y.OSUser && x.OwnerAccount == y.OwnerAccount && slices.Equal(x.Workdirs, y.Workdirs)
	}) && slices.Equal(a.Doctor, b.Doctor) && a.IsServerHost == b.IsServerHost && a.RequireLocalConfirm == b.RequireLocalConfirm &&
		a.OSVersion == b.OSVersion
}

// withPeer returns r with is_server_host or-ed with whether the server of this connection runs on
// this machine (24 section 2.3): HD5 alone misses a server reached through its own address.
func withPeer(r startup.Report, peerLocal bool) startup.Report {
	r.IsServerHost = r.IsServerHost || peerLocal
	// The lists must never be null on the wire (ADR 0053).
	if r.Profiles == nil {
		r.Profiles = []profile{}
	}
	if r.SessionUsers == nil {
		r.SessionUsers = []startupUser{}
	}
	if r.Doctor == nil {
		r.Doctor = []entry{}
	}
	return r
}

// PeerIsLocal reports whether remote, the address of the server of the connection, is a loopback
// address or one of own, the addresses of this machine (24 section 2.3).
func PeerIsLocal(remote net.IP, own []net.IP) bool {
	if remote == nil {
		return false
	}
	if remote.IsLoopback() {
		return true
	}
	return slices.ContainsFunc(own, func(ip net.IP) bool { return ip.Equal(remote) })
}

// OwnAddresses returns the addresses of the interfaces of this machine.
func OwnAddresses() ([]net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	out := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			out = append(out, n.IP)
		}
	}
	return out, nil
}

// The list types of a report (doctor, M2-42).
type (
	profile     = doctor.Profile
	startupUser = doctor.ReportedSessionUser
	entry       = doctor.ReportEntry
)

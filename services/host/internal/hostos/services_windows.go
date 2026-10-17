// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"context"

	"golang.org/x/sys/windows/svc/mgr"
)

// SCM is the ServiceManager of Windows: QueryServiceStatusEx through the service control manager.
type SCM struct{}

// MainPID implements ServiceManager.
func (SCM) MainPID(_ context.Context, name string) (int, error) {
	m, err := mgr.Connect()
	if err != nil {
		return 0, err
	}
	defer func() { _ = m.Disconnect() }()
	s, err := m.OpenService(name)
	if err != nil {
		return 0, ErrNotRunning
	}
	defer func() { _ = s.Close() }()
	st, err := s.Query()
	if err != nil || st.ProcessId == 0 {
		return 0, ErrNotRunning
	}
	return int(st.ProcessId), nil
}

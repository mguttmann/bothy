// SPDX-License-Identifier: Apache-2.0

package install

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"os"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// OSOps are the Ops of Windows.
type OSOps struct {
	Runner hostos.Runner
	Users  hostos.Users
	// passwords holds the random password of each session user created in this run; it is handed
	// only to the service control manager and never stored or printed.
	passwords map[string]string
}

var (
	advapi32                     = windows.NewLazySystemDLL("advapi32.dll")
	procSetFileSecurityW         = advapi32.NewProc("SetFileSecurityW")
	procSetServiceObjectSecurity = advapi32.NewProc("SetServiceObjectSecurity")
	procLsaAddAccountRights      = advapi32.NewProc("LsaAddAccountRights")
	procLsaOpenPolicy            = advapi32.NewProc("LsaOpenPolicy")
	procLsaClose                 = advapi32.NewProc("LsaClose")
	netapi32                     = windows.NewLazySystemDLL("netapi32.dll")
	procNetUserAdd               = netapi32.NewProc("NetUserAdd")
	procNetUserSetInfo           = netapi32.NewProc("NetUserSetInfo")
	userenv                      = windows.NewLazySystemDLL("userenv.dll")
	procCreateProfile            = userenv.NewProc("CreateProfile")
	errServiceStopTimeout        = errors.New("service did not stop")
	errUnsupportedWindowsOps     = errors.New("unknown step")
)

// Run implements Ops.
func (o *OSOps) Run(ctx context.Context, argv []string) error {
	out, err := o.Runner.Run(ctx, hostos.Cmd{Path: argv[0], Args: argv[1:]})
	if err != nil {
		return err
	}
	if out.ExitCode != 0 {
		return fmt.Errorf("exit code %d", out.ExitCode)
	}
	return nil
}

// Mkdir implements Ops; the DACL comes from a win.acl step.
func (o *OSOps) Mkdir(path string, _ fs.FileMode, _ string) error { return os.MkdirAll(path, 0o700) }

// Write implements Ops; the file inherits the DACL of its folder.
func (o *OSOps) Write(path string, content []byte, _ fs.FileMode, _ string) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, content) { //nolint:gosec // a path of the plan
		return nil
	}
	return os.WriteFile(path, content, 0o600)
}

// Copy implements Ops. A repeated install replaces the image that the host and helper services
// run: Windows refuses to replace a running image but lets it be renamed, so the old file moves
// aside first and is removed when no process runs it any more. Installing from the installed
// binary itself copies nothing.
func (o *OSOps) Copy(src, dst string, _ fs.FileMode, _ string) error {
	if a, err := os.Stat(src); err == nil {
		if b, err := os.Stat(dst); err == nil && os.SameFile(a, b) {
			return nil
		}
	}
	in, err := os.Open(src) //nolint:gosec // the running binary
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	tmp := dst + ".new"
	out, err := os.Create(tmp) //nolint:gosec // a path of the plan
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	aside, err := moveAside(dst)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		if aside != "" {
			_ = os.Rename(aside, dst)
		}
		return err
	}
	if aside != "" {
		_ = os.Remove(aside) // fails while a service still runs the old image; the next install retries
	}
	return nil
}

// moveAside renames an existing dst to a free name next to it and returns that name ("" when dst
// does not exist). A name whose earlier image a process still runs cannot be reused, so the next
// one is tried.
func moveAside(dst string) (string, error) {
	if _, err := os.Stat(dst); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	var err error
	for i := range maxAsideImages {
		aside := fmt.Sprintf("%s.old%d", dst, i)
		_ = os.Remove(aside) // an earlier image that no process runs any more
		if err = os.Rename(dst, aside); err == nil {
			return aside, nil
		}
	}
	return "", err
}

// maxAsideImages bounds the names moveAside tries; each taken one is an earlier image that runs.
const maxAsideImages = 8

// Chmod implements Ops; Windows closes a home with a win.acl step instead.
func (o *OSOps) Chmod(string, fs.FileMode, string) error { return nil }

// Remove implements Ops.
func (o *OSOps) Remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Home implements Ops.
func (o *OSOps) Home(user string) (string, error) {
	a, err := o.Users.Lookup(user)
	if err != nil {
		return "", err
	}
	return a.Home, nil
}

// Windows implements Ops.
func (o *OSOps) Windows(ctx context.Context, s Step) error {
	switch s.Kind {
	case KindWinUser:
		return o.user(s.Name)
	case KindWinProfile:
		return profile(s.Name)
	case KindWinService:
		return o.service(s.Content)
	case KindWinServiceDel:
		return deleteService(ctx, s.Name)
	case KindWinRestart:
		return restartService(ctx, s.Name)
	case KindWinACL:
		return protectedACL(s.Path, s.Name, s.Content)
	case KindWinDriveRoot:
		return driveRoot(s.Path)
	}
	return errUnsupportedWindowsOps
}

func randomPassword() (string, error) {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789-_.!"
	var b strings.Builder
	b.WriteString("Aa1!")
	for range 28 {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		b.WriteByte(alphabet[n.Int64()])
	}
	return b.String(), nil
}

type userInfo1 struct {
	Name, Password *uint16
	PasswordAge    uint32
	Priv           uint32
	HomeDir        *uint16
	Comment        *uint16
	Flags          uint32
	ScriptPath     *uint16
}

// user creates the local account name and grants SeServiceLogonRight. An existing account whose
// helper service exists keeps its password (a reset would lose the data DPAPI protects for the
// user, its login included); an existing account without the service gets a new password, which
// only the service control manager receives.
func (o *OSOps) user(name string) error {
	if _, _, _, err := windows.LookupSID("", name); err == nil && serviceExists(hostos.HelperServiceName(hostos.Windows, name)) {
		return serviceLogonRight(name)
	}
	pw, err := randomPassword()
	if err != nil {
		return err
	}
	n, _ := windows.UTF16PtrFromString(name)
	p, _ := windows.UTF16PtrFromString(pw)
	info := userInfo1{Name: n, Password: p, Priv: 1, Flags: 0x0001 | 0x10000} // UF_SCRIPT | UF_DONT_EXPIRE_PASSWD
	r, _, _ := procNetUserAdd.Call(0, 1, uintptr(unsafe.Pointer(&info)), 0)   //nolint:gosec // documented signature
	if r == 2224 {                                                            // NERR_UserExists: reset the password (level 1003)
		r, _, _ = procNetUserSetInfo.Call(0, uintptr(unsafe.Pointer(n)), 1003, uintptr(unsafe.Pointer(&p)), 0) //nolint:gosec // documented signature
	}
	if r != 0 {
		return fmt.Errorf("account %d", r)
	}
	if o.passwords == nil {
		o.passwords = map[string]string{}
	}
	o.passwords[name] = pw
	return serviceLogonRight(name)
}

type lsaUnicodeString struct {
	Length, MaximumLength uint16
	Buffer                *uint16
}

type lsaObjectAttributes struct {
	Length                   uint32
	RootDirectory            windows.Handle
	ObjectName               *lsaUnicodeString
	Attributes               uint32
	SecurityDescriptor       uintptr
	SecurityQualityOfService uintptr
}

func serviceLogonRight(name string) error {
	sid, _, _, err := windows.LookupSID("", name)
	if err != nil {
		return err
	}
	var attrs lsaObjectAttributes
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	var policy windows.Handle
	const policyAll = 0x00F0FFF
	if r, _, _ := procLsaOpenPolicy.Call(0, uintptr(unsafe.Pointer(&attrs)), policyAll, uintptr(unsafe.Pointer(&policy))); r != 0 { //nolint:gosec // documented signature
		return fmt.Errorf("lsa open %x", r)
	}
	defer func() { _, _, _ = procLsaClose.Call(uintptr(policy)) }()
	right, _ := windows.UTF16FromString("SeServiceLogonRight")
	us := lsaUnicodeString{Length: uint16((len(right) - 1) * 2), MaximumLength: uint16(len(right) * 2), Buffer: &right[0]}               //nolint:gosec // a constant name
	if r, _, _ := procLsaAddAccountRights.Call(uintptr(policy), uintptr(unsafe.Pointer(sid)), uintptr(unsafe.Pointer(&us)), 1); r != 0 { //nolint:gosec // documented signature
		return fmt.Errorf("lsa right %x", r)
	}
	return nil
}

// profile creates the profile of name with CreateProfile and gives its folder an ACL only for the
// user and the trusted owners.
func profile(name string) error {
	sid, _, _, err := windows.LookupSID("", name)
	if err != nil {
		return err
	}
	s, _ := windows.UTF16PtrFromString(sid.String())
	n, _ := windows.UTF16PtrFromString(name)
	buf := make([]uint16, windows.MAX_PATH)
	r, _, _ := procCreateProfile.Call(uintptr(unsafe.Pointer(s)), uintptr(unsafe.Pointer(n)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))) //nolint:gosec // documented signature
	if r != 0 && r != 0x800700b7 {                                                                                                                 // ERROR_ALREADY_EXISTS as HRESULT
		return fmt.Errorf("profile %x", r)
	}
	home := windows.UTF16ToString(buf)
	if home == "" {
		a, err := (hostos.WindowsUsers{}).Lookup(name)
		if err != nil {
			return err
		}
		home = a.Home
	}
	return protectedACL(home, name, closedHomeSDDL)
}

// protectedACL sets a protected DACL from sddl on path; the placeholders name the SID of the
// virtual account of the host service and of user.
func protectedACL(path, user, sddl string) error {
	if strings.Contains(sddl, ServiceSIDPlaceholder) {
		sid, _, _, err := windows.LookupSID("", HostAccountWindows)
		if err != nil {
			return err
		}
		sddl = strings.ReplaceAll(sddl, ServiceSIDPlaceholder, sid.String())
	}
	if strings.Contains(sddl, "<user-sid>") {
		if user == "" {
			user = strings.TrimPrefix(path, "~")
		}
		sid, _, _, err := windows.LookupSID("", user)
		if err != nil {
			return err
		}
		sddl = strings.ReplaceAll(sddl, "<user-sid>", sid.String())
	}
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

// serviceExists reports whether the service control manager knows name.
func serviceExists(name string) bool {
	m, err := mgr.Connect()
	if err != nil {
		return false
	}
	defer func() { _ = m.Disconnect() }()
	s, err := m.OpenService(name)
	if err != nil {
		return false
	}
	_ = s.Close()
	return true
}

func parseDefinition(content string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			out[k] = v
		}
	}
	return out
}

// service creates or updates the service of a rendered definition: account, start type, failure
// actions on crashes only, and its DACL; a helper service is started.
func (o *OSOps) service(content string) error {
	d := parseDefinition(content)
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()
	account := d["account"]
	password := ""
	if user, ok := strings.CutPrefix(account, `.\`); ok {
		password = o.passwords[user]
	}
	name, _ := windows.UTF16PtrFromString(d["name"])
	display, _ := windows.UTF16PtrFromString(d["display"])
	bin, _ := windows.UTF16PtrFromString(d["binary"])
	acc, _ := windows.UTF16PtrFromString(account)
	var pw *uint16
	if password != "" {
		pw, _ = windows.UTF16PtrFromString(password)
	}
	h, err := windows.CreateService(m.Handle, name, display, windows.SERVICE_ALL_ACCESS, windows.SERVICE_WIN32_OWN_PROCESS,
		windows.SERVICE_AUTO_START, windows.SERVICE_ERROR_NORMAL, bin, nil, nil, nil, acc, pw)
	if errors.Is(err, windows.ERROR_SERVICE_EXISTS) {
		s, oerr := m.OpenService(d["name"])
		if oerr != nil {
			return oerr
		}
		h = s.Handle
		err = windows.ChangeServiceConfig(h, windows.SERVICE_NO_CHANGE, windows.SERVICE_AUTO_START, windows.SERVICE_NO_CHANGE, bin, nil, nil, nil, acc, pw, display)
	}
	if err != nil {
		return err
	}
	s := &mgr.Service{Name: d["name"], Handle: h}
	defer func() { _ = s.Close() }()
	actions := []mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 5 * time.Second}, {Type: mgr.ServiceRestart, Delay: 5 * time.Second}, {Type: mgr.ServiceRestart, Delay: 5 * time.Second}}
	if err := s.SetRecoveryActions(actions, 86400); err != nil {
		return err
	}
	if err := s.SetRecoveryActionsOnNonCrashFailures(false); err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString(d["sddl"])
	if err != nil {
		return err
	}
	if r, _, e := procSetServiceObjectSecurity.Call(uintptr(h), uintptr(windows.DACL_SECURITY_INFORMATION), uintptr(unsafe.Pointer(sd))); r == 0 { //nolint:gosec // documented signature
		return e
	}
	if strings.HasPrefix(d["name"], "bothy-host-helper-") {
		if err := s.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
			return err
		}
	}
	return nil
}

func stopService(ctx context.Context, s *mgr.Service) error {
	st, err := s.Query()
	if err != nil {
		return err
	}
	if st.State == svc.Stopped {
		return nil
	}
	if _, err := s.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return err
	}
	for range 300 {
		if st, err := s.Query(); err == nil && st.State == svc.Stopped {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errServiceStopTimeout
}

func deleteService(ctx context.Context, name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()
	s, err := m.OpenService(name)
	if err != nil {
		return nil // already gone
	}
	defer func() { _ = s.Close() }()
	if err := stopService(ctx, s); err != nil {
		return err
	}
	return s.Delete()
}

// restartService stops the service when it runs and starts it; a service stopped after a refused
// start is just started.
func restartService(ctx context.Context, name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()
	s, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()
	if err := stopService(ctx, s); err != nil {
		return err
	}
	return s.Start()
}

// driveRoot removes Authenticated Users:(AD) from the DACL of root, written with SetFileSecurityW
// so nothing propagates to the children (24 2.2 point 6); the control bits stay.
func driveRoot(root string) error {
	sd, err := windows.GetNamedSecurityInfo(root, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return errors.New("no DACL")
	}
	// AclSize is the third and fourth byte of the ACL header (MS-DTYP 2.4.5).
	hdr := unsafe.Slice((*byte)(unsafe.Pointer(dacl)), aclHeaderLen)               //nolint:gosec // the ACL header
	raw := unsafe.Slice((*byte)(unsafe.Pointer(dacl)), int(hdr[2])|int(hdr[3])<<8) //nolint:gosec // the ACL of the descriptor
	out, removed, err := RemoveAuthenticatedUsersAD(raw)
	if err != nil || !removed {
		return err
	}
	control, _, err := sd.Control()
	if err != nil {
		return err
	}
	abs, err := windows.NewSecurityDescriptor()
	if err != nil {
		return err
	}
	keep := control & (windows.SE_DACL_PROTECTED | windows.SE_DACL_AUTO_INHERITED | windows.SE_DACL_AUTO_INHERIT_REQ)
	if err := abs.SetControl(keep, keep); err != nil {
		return err
	}
	if err := abs.SetDACL((*windows.ACL)(unsafe.Pointer(&out[0])), true, false); err != nil { //nolint:gosec // the rebuilt binary ACL
		return err
	}
	p, _ := windows.UTF16PtrFromString(root)
	if r, _, e := procSetFileSecurityW.Call(uintptr(unsafe.Pointer(p)), uintptr(windows.DACL_SECURITY_INFORMATION), uintptr(unsafe.Pointer(abs))); r == 0 { //nolint:gosec // documented signature
		return e
	}
	return nil
}

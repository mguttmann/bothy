# SPDX-License-Identifier: Apache-2.0
#
# Starts the computer helper in the desktop session of the person logged in at the console, right
# after the MSI installed it (docs/spec/27-computer-use.md 9.3), so no login is needed first; the
# Run key starts it at every later login. The MSI runs it as a deferred custom action as the
# system account when the feature ComputerHelper is installed:
#
#   powershell -NoProfile -ExecutionPolicy Bypass -File start-computer-helper.ps1 -Helper <exe>
#
# The helper declares uiAccess, which a plain CreateProcess refuses (error 740): the script takes
# the user token of the active console session (WTSQueryUserToken), sets TokenUIAccess on a copy
# (the system account holds SeTcbPrivilege) and starts the helper with CreateProcessAsUser on
# winsta0\default, as the AppInfo service does for a person who starts it. Without a person at the
# console it does nothing; a helper that already runs in that session ends at once by its own
# single-instance rule. It prints one line and exits 0 in every case, so an install never fails
# over it (the custom action ignores the exit code too).
param([Parameter(Mandatory = $true)][string]$Helper)

$ErrorActionPreference = 'Stop'
$source = @'
using System;
using System.Runtime.InteropServices;

public static class ComputerHelperStart {
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    struct StartupInfo {
        public int cb; public string lpReserved; public string lpDesktop; public string lpTitle;
        public int dwX, dwY, dwXSize, dwYSize, dwXCountChars, dwYCountChars, dwFillAttribute, dwFlags;
        public short wShowWindow, cbReserved2; public IntPtr lpReserved2, hStdInput, hStdOutput, hStdError;
    }
    [StructLayout(LayoutKind.Sequential)]
    struct ProcessInformation { public IntPtr hProcess, hThread; public int dwProcessId, dwThreadId; }

    [DllImport("kernel32.dll")] static extern uint WTSGetActiveConsoleSessionId();
    [DllImport("wtsapi32.dll", SetLastError = true)] static extern bool WTSQueryUserToken(uint session, out IntPtr token);
    [DllImport("advapi32.dll", SetLastError = true)]
    static extern bool DuplicateTokenEx(IntPtr token, uint access, IntPtr attributes, int level, int type, out IntPtr copy);
    [DllImport("advapi32.dll", SetLastError = true)]
    static extern bool SetTokenInformation(IntPtr token, int infoClass, ref int value, int length);
    [DllImport("userenv.dll", SetLastError = true)] static extern bool CreateEnvironmentBlock(out IntPtr env, IntPtr token, bool inherit);
    [DllImport("userenv.dll")] static extern bool DestroyEnvironmentBlock(IntPtr env);
    [DllImport("advapi32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
    static extern bool CreateProcessAsUserW(IntPtr token, string app, string cmd, IntPtr pa, IntPtr ta, bool inherit,
        uint flags, IntPtr env, string dir, ref StartupInfo si, out ProcessInformation pi);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr h);

    const uint NoSession = 0xFFFFFFFF;
    const uint TokenAllAccess = 0xF01FF;
    const int SecurityImpersonation = 2, TokenPrimary = 1, TokenUIAccess = 26;
    const uint CreateUnicodeEnvironment = 0x400;

    // Start returns a line for the MSI log: started with the process ID, or why not.
    public static string Start(string exe, string dir) {
        uint session = WTSGetActiveConsoleSessionId();
        if (session == NoSession || session == 0) return "no console session";
        IntPtr user;
        if (!WTSQueryUserToken(session, out user)) return "nobody logged in at the console (error " + Marshal.GetLastWin32Error() + ")";
        IntPtr token = IntPtr.Zero, env = IntPtr.Zero;
        try {
            if (!DuplicateTokenEx(user, TokenAllAccess, IntPtr.Zero, SecurityImpersonation, TokenPrimary, out token))
                return "token copy failed (error " + Marshal.GetLastWin32Error() + ")";
            int on = 1;
            if (!SetTokenInformation(token, TokenUIAccess, ref on, 4))
                return "uiAccess refused (error " + Marshal.GetLastWin32Error() + ")";
            if (!CreateEnvironmentBlock(out env, token, false)) env = IntPtr.Zero;
            StartupInfo si = new StartupInfo();
            si.cb = Marshal.SizeOf(typeof(StartupInfo));
            si.lpDesktop = @"winsta0\default";
            ProcessInformation pi;
            if (!CreateProcessAsUserW(token, exe, "\"" + exe + "\"", IntPtr.Zero, IntPtr.Zero, false,
                    env == IntPtr.Zero ? 0 : CreateUnicodeEnvironment, env, dir, ref si, out pi))
                return "start failed (error " + Marshal.GetLastWin32Error() + ")";
            CloseHandle(pi.hThread);
            CloseHandle(pi.hProcess);
            return "started in session " + session + " (process " + pi.dwProcessId + ")";
        } finally {
            if (env != IntPtr.Zero) DestroyEnvironmentBlock(env);
            if (token != IntPtr.Zero) CloseHandle(token);
            CloseHandle(user);
        }
    }
}
'@

try {
    Add-Type -TypeDefinition $source
    'start-computer-helper: ' + [ComputerHelperStart]::Start($Helper, (Split-Path -Parent $Helper))
} catch {
    'start-computer-helper: ' + $_.Exception.Message
}
exit 0

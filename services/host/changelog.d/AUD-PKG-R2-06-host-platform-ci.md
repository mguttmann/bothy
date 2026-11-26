### Fixed

- HD3 detects a passwordless `sudo` on macOS: the probe runs `sudo -n /usr/bin/true` with an
  absolute command, because without `PATH` sudo found a bare `true` only through a `secure_path`,
  which macOS does not set (AUD-PKG-R2-06/2).
- Windows: `install` makes Administrators the owner of the folders and files it creates, so an
  install by an administrator without UAC no longer leaves files of the installing account that
  HD4, HD10 and HD11 refuse; it refuses a path that existed before with another owner or as a
  junction or link (AUD-PKG-R2-06/3, /4).
- Windows: HD2 reads the local groups of the virtual account of the host service by its SID; it
  read none and reported `forbidden_group` on every Windows host (AUD-PKG-R2-06/5).
- Platform suite: it reads the exit code that `launchctl print` writes with its name, types no
  `!"` into the zsh of macOS, stages the replaced helper of TM-T26 on macOS as on Linux, creates
  and removes the Windows profiles of its accounts, owns its Windows files by Administrators and
  prints the log file of the service for a failed test on macOS and Windows (AUD-PKG-R2-06/6).

# infra/host

Installation packages of the host service `bothy-host` (brief M7-23,
[16-operations.md](../../docs/spec/16-operations.md) 4.4,
[18-release.md](../../docs/spec/18-release.md) 6.3). The release steps that build them are
`tools/distribution/hostdist.py`, started by `make -C services/host release-build`,
`release-apple` and `release-windows`.

| Path | Content |
|---|---|
| `linux/nfpm.yaml` | recipe of the `.deb` and `.rpm` (nfpm, pinned in `tools/bin/tools.lock.json`) |
| `linux/bothy-host.service` | the systemd unit of the host service, byte for byte the one `bothy-host install` writes |
| `linux/postinstall.sh`, `preremove.sh`, `postremove.sh` | package scripts: a `daemon-reload` when systemd runs; before a removal (not an upgrade) the host service is stopped and disabled |
| `macos/bothy-host.plist` | the LaunchDaemon of the host service, byte for byte the one `bothy-host install` writes |
| `macos/scripts/postinstall` | keeps the launchd job disabled until the service user exists |
| `windows/bothy-host.wxs` | the MSI (WiX Toolset): binary and the service `bothy-host`, stopped, start type demand |

What every package does: it installs the binary into the program folder of
`bothy-host install` (Linux `/opt/bothy-host/bin`, macOS `/Library/Application
Support/bothy-host/bin`, Windows `%ProgramFiles%\bothy-host`) and the service definition of the
host service, both owned by `root` (Windows: the trusted owners) and writable by them only. It
creates no user, enables or starts no service and never pairs. Setup stays:

```sh
sudo /opt/bothy-host/bin/bothy-host install --session-user <name>
sudo /opt/bothy-host/bin/bothy-host pair --server <url> --code <code>
```

The unit and the plist are held equal to `bothy-host install` by
`TestPackagedDefinitionsEqualInstall`, the service of the MSI by `TestMSIServiceEqualsInstall`
(`services/host/internal/install`); `go test ./internal/install -update` rewrites the two files
after a change of the templates. Removal: `apt remove bothy-host` or `dnf remove bothy-host` and
the uninstall of the MSI stop the host service and remove binary and definition; the state
folder, the configuration and what `install` created stay. macOS has no uninstaller: remove the
two files and `pkgutil --forget com.example.bothy-host.binary` and
`com.example.bothy-host.launchd` by hand.

Update: install the newer package of the same kind, then `bothy-host restart` (after an MSI
upgrade `bothy-host install`). The host service names this route when the server refuses its
version with `4114`.

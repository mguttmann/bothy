# infra/host

Installation packages of the host service `bothy-host` (brief M7-23,
[16-operations.md](../../docs/spec/16-operations.md) 4.4,
[18-release.md](../../docs/spec/18-release.md) 6.3). The release steps that build them are
`tools/distribution/hostdist.py`, started by `make -C services/host release-build`,
`release-apple` and `release-windows`.

| Path | Content |
|---|---|
| `linux/nfpm.yaml` | recipe of the `.deb` and `.rpm` (nfpm, pinned in `tools/bin/tools.lock.json`) |
| `linux/bothy-host.service` | the systemd unit of the host service as `bothy-host install` writes it for the default configuration path; the packages carry it as a reference in `/opt/bothy-host/share` |
| `linux/postinstall.sh`, `preremove.sh`, `postremove.sh` | package scripts: a `daemon-reload` when systemd runs; before a removal (not an upgrade) the host service is stopped and disabled |
| `macos/bothy-host.plist` | the LaunchDaemon of the host service as `bothy-host install` writes it; the `.pkg` carries it as a reference in `/Library/Application Support/bothy-host/share` |
| `windows/bothy-host.wxs` | the MSI (WiX Toolset): binary and the service `bothy-host`, stopped, start type demand |

What every package does: it installs the binary into the program folder of
`bothy-host install` (Linux `/opt/bothy-host/bin`, macOS `/Library/Application
Support/bothy-host/bin`, Windows `%ProgramFiles%\bothy-host`) and the service definition of the
host service: the `.deb`, `.rpm` and `.pkg` as a reference copy in `share` next to `bin`, which no
service manager loads, the MSI as the service `bothy-host`, stopped with the start type demand.
Everything belongs to `root` (Windows: the trusted owners) and only they write it. No package
creates a user, writes the live definition, enables or starts a service or pairs; `bothy-host
install` writes the live definition (also with `--config`), and a package upgrade never touches
it. Setup stays:

```sh
sudo /opt/bothy-host/bin/bothy-host install --session-user <name>
sudo /opt/bothy-host/bin/bothy-host pair --server <url> --code <code>
```

The unit and the plist are held equal to `bothy-host install` by
`TestPackagedDefinitionsEqualInstall`, the service of the MSI by `TestMSIServiceEqualsInstall`
(`services/host/internal/install`); `go test ./internal/install -update` rewrites the two files
after a change of the templates. Removal: `apt remove bothy-host` or `dnf remove bothy-host` and
the uninstall of the MSI stop the host service and remove binary and reference (MSI: the
service); the state folder, the configuration and what `install` created stay. macOS has no
uninstaller: remove `/Library/Application Support/bothy-host/bin` and `share` and run
`pkgutil --forget com.example.bothy-host` by hand.

Update: install the newer package of the same kind, then `bothy-host restart` (after an MSI
upgrade `bothy-host install --config "<file>"` with the configuration file in use). The host service names this route when the server refuses its
version with `4114`.

## Owner account mode

The packages install the system setup of `bothy-host install`. For the owner account mode (ADR
0073, docs/spec/24-terminal.md 2.4) the instance operator uses the release binary instead and runs
`bothy-host install --owner-account` as the owner's own account; it needs no package and no root
([services/host/README.md](../../services/host/README.md), "Owner account mode").

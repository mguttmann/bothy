# SPDX-License-Identifier: Apache-2.0
"""Release steps of the host service bothy-host (brief M7-23, docs/spec/18-release.md 2 and 6).

Builds the binaries and the installation packages of the component `host` (tag `host/v<X.Y.Z>`):

* `build` (`make -C services/host release-build`, release.yml job build): the release binaries
  `bothy-host_<v>_<os>_<arch>[.exe]` for darwin/arm64, darwin/amd64, linux/amd64, linux/arm64 and
  windows/amd64, and the Linux packages `bothy-host_<v>_linux_<arch>.deb` and `.rpm` from
  `infra/host/linux/nfpm.yaml` with nfpm (pinned in tools/bin/tools.lock.json). Every binary is
  built without cgo from `services/host/VERSION` (`-trimpath -buildvcs=false`, the version and the
  package kind with `-ldflags -X`, internal/buildinfo); a package carries its own build with the
  package kind, so its `4114` line names its update route.
* `apple-release` (release.yml job build-apple, environment release-apple): the binaries of the
  package kind `pkg` for arm64 and amd64, signed with the Developer ID Application identity and the
  hardened runtime (`codesign --verify --strict` must pass and name the team of the
  distribution), one `.pkg` per architecture from `pkgbuild` and `productbuild`, signed with the
  Developer ID Installer identity, notarized (`notarytool submit --wait`), stapled, and accepted
  by `spctl --assess --type install`.
* `windows-release` (job build-windows, environment release-windows): the binary of the package
  kind `msi`, signed with `signtool` (`signtool verify /pa` must pass), the MSI from
  `infra/host/windows/bothy-host.wxs` with the WiX Toolset (WIX_VERSION, nupkg checked against
  WIX_SHA256), signed and verified the same way.

Without its secrets a release step stops before anything is built or signed, naming the missing
ones; the distribution file (apps/distribution) gives the Apple team and the timestamp server and
must hold no placeholder.

The computer helper (brief CU-02, D-CU-01-29, docs/decisions.d/CU-02.md D-CU-02-10) is not
published: no release step above carries it. On the owner's own machines these steps sign it and
build a host package with the helper as an optional component:

* `helper-sign-macos`: signs `Bothy Computer Helper.app` with a Developer ID Application identity of
  the login keychain and the hardened runtime (no notarization), verifies it and compares its
  designated requirement with the one recorded for the previous signed build.
* `helper-package-macos`: one `.pkg` for one architecture with the choices `host` (always) and
  `computer-helper` (off unless selected); the helper's postinstall sets
  `host_service.computer_use` and loads its LaunchAgent.
* `helper-cert-windows`: creates the self-signed code signing certificate of the machine once.
* `helper-sign-windows`: signs `bothy-computer-helper.exe` with a certificate of the machine store
  and checks `signtool verify /pa`.
* `helper-package-windows`: the host MSI with the feature `ComputerHelper` (off unless selected,
  `ADDLOCAL=ComputerHelper`); without a thumbprint everything stays unsigned (a development
  package, as the CI jobs build it).

`apple-trial` and `windows-trial` (the CI jobs `host trial (macOS)` and `host trial (Windows)`,
D-W-127) run the same steps with a test publisher: a self-signed identity in a temporary keychain
or certificate store, trusted for the run only, no notarization; productbuild signs only with an
Apple-issued installer certificate, so the trial `.pkg` stays unsigned (its binary is signed). They
then install the package on the disposable runner and check what an operator gets: owners and modes
of the binary and the service definition, no running service before `bothy-host install`, the
version, and on Windows an uninstall that removes binary and service and keeps the state folder.
`spctl` rejects a test publisher; the trial records its answer (not proven,
docs/decisions.d/M7-23.md). The result goes to `<out>/trial-host-apple.json` or
`trial-host-windows.json`.

Usage:
  hostdist.py build --version X.Y.Z --out DIR [--nfpm PATH]
  hostdist.py apple-release --version X.Y.Z --out DIR --distribution FILE
  hostdist.py apple-trial --out DIR
  hostdist.py windows-release --version X.Y.Z --out DIR --distribution FILE
  hostdist.py windows-trial --out DIR
  hostdist.py helper-sign-macos --app APP --identity NAME --requirement-file FILE
  hostdist.py helper-package-macos --version X.Y.Z --out DIR --app APP [--arch ARCH]
  hostdist.py helper-cert-windows [--subject SUBJECT]
  hostdist.py helper-sign-windows --exe EXE --thumbprint SHA1
  hostdist.py helper-package-windows --version X.Y.Z --out DIR --helper EXE [--thumbprint SHA1]
Exit codes: 0 done, 1 a step failed (named on stderr), 2 usage error.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import plistlib
import re
import shutil
import sys
import tempfile
import urllib.request
import zipfile
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import appdist
import apple
import windows
from common import REPO, Runner, StepError, fail, require_env

HOST = REPO / "services" / "host"
INFRA = REPO / "infra" / "host"
MODULE = "github.com/OWNER/bothy/services/host"
# The five platforms of docs/spec/18-release.md 2 (row host).
PLATFORMS = (
    ("darwin", "arm64"),
    ("darwin", "amd64"),
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("windows", "amd64"),
)
# Package kinds as internal/buildinfo names them.
KIND_DEB, KIND_RPM, KIND_PKG, KIND_MSI = "deb", "rpm", "pkg", "msi"
# The Linux packages (kind, architecture) of infra/host/linux/nfpm.yaml.
LINUX_PACKAGES = (
    (KIND_DEB, "amd64"),
    (KIND_DEB, "arm64"),
    (KIND_RPM, "amd64"),
    (KIND_RPM, "arm64"),
)
APPLE_ARCHES = ("arm64", "amd64")
# Identifier of the .pkg component (white label: example.com, brief rule 8).
PKG_IDENTIFIER = "com.example.bothy-host"
# Where the packages put the binary and the service definition (internal/install, 16 4.4).
DARWIN_PROGRAM_DIR = Path("/Library/Application Support/bothy-host/bin")
DARWIN_PLIST = Path("/Library/LaunchDaemons/bothy-host.plist")
DARWIN_SHARE = Path("/Library/Application Support/bothy-host/share")
# WiX Toolset: the nupkg of the .NET tool, checked against its SHA-256 before use (brief rule 9).
# Version 5 is the last one without the maintenance fee agreement of later versions.
WIX_VERSION = "5.0.2"
WIX_SHA256 = "f30ef0c74e2a986126539c5780be93ac24e8136eaf723b1937b26272703ae173"
WIX_URL = f"https://api.nuget.org/v3-flatcontainer/wix/{WIX_VERSION}/wix.{WIX_VERSION}.nupkg"
WIX_CACHE = REPO / ".cache" / "wix"
APPLE_SECRETS = (
    "APPLE_DEVELOPER_ID_P12",
    "APPLE_DEVELOPER_ID_P12_PASSWORD",
    "APPLE_INSTALLER_P12",
    "APPLE_INSTALLER_P12_PASSWORD",
    "APPLE_NOTARY_KEY",
    "APPLE_NOTARY_KEY_ID",
    "APPLE_NOTARY_ISSUER_ID",
)
WINDOWS_SECRETS = windows.WINDOWS_SECRETS
TRIAL_SUBJECT = "CN=bothy-host test publisher"
# The computer helper (docs/decisions.d/CU-02.md D-CU-02-3); white label like PKG_IDENTIFIER.
HELPER_APP = "Bothy Computer Helper.app"
HELPER_EXE_MACOS = "bothy-computer-helper"
HELPER_EXE_WINDOWS = "bothy-computer-helper.exe"
HELPER_BUNDLE_ID = "com.example.bothy-computer-helper"
HELPER_PKG_IDENTIFIER = HELPER_BUNDLE_ID
DARWIN_HELPER_DIR = Path("/Library/Application Support/bothy-host/desktop")
HELPER_LAUNCH_AGENT = Path(f"/Library/LaunchAgents/{HELPER_BUNDLE_ID}.plist")
HELPER_CERT_SUBJECT = "CN=Bothy Computer Helper (local)"
_THUMBPRINT = re.compile(r"^[0-9A-Fa-f]{40}$")
_SEMVER = re.compile(
    r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$"
)


# ---- names and the Go build -----------------------------------------------------------------


def binary_name(version: str, goos: str, goarch: str) -> str:
    """Return the file name of a release binary, for example `bothy-host_1.2.3_linux_amd64`."""
    return f"bothy-host_{version}_{goos}_{goarch}" + (".exe" if goos == "windows" else "")


def package_name(version: str, goos: str, goarch: str, kind: str) -> str:
    """Return the file name of a package, for example `bothy-host_1.2.3_linux_arm64.rpm`."""
    return f"bothy-host_{version}_{goos}_{goarch}.{kind}"


def component_version() -> str:
    """Return `services/host/VERSION` without white space."""
    return (HOST / "VERSION").read_text(encoding="utf-8").strip()


def check_version(version: str) -> None:
    """Stop unless `version` is SemVer and equals `services/host/VERSION` (the tag, 18 2 rule 4).

    Raises:
        StepError: it is not.
    """
    if not _SEMVER.match(version):
        raise StepError(f"version {version!r} is not SemVer without a v")
    if version != component_version():
        raise StepError(f"services/host/VERSION says {component_version()}, not {version}")


def ldflags(version: str, kind: str) -> str:
    """Return the linker flags of a release build: no symbol table, version and package kind."""
    return (
        f"-s -w -X {MODULE}/internal/buildinfo.Version={version} "
        f"-X {MODULE}/internal/buildinfo.PackageKind={kind}"
    )


def go_build(run: Runner, goos: str, goarch: str, version: str, kind: str, out: Path) -> Path:
    """Build `cmd/bothy-host` for goos/goarch without cgo into `out`; return `out`.

    The build depends only on the source and `version` (`-trimpath -buildvcs=false`), so the
    same commit and version give the same binary with the same Go (brief requirement 13).
    """
    out.parent.mkdir(parents=True, exist_ok=True)
    run(
        [
            "go",
            "build",
            "-trimpath",
            "-buildvcs=false",
            "-ldflags",
            ldflags(version, kind),
            "-o",
            out,
            "./cmd/bothy-host",
        ],
        cwd=HOST,
        env={
            "CGO_ENABLED": "0",
            "GOOS": goos,
            "GOARCH": goarch,
            "GOTOOLCHAIN": "local",
            "GOFLAGS": "-mod=readonly",
        },
    )
    return out


def build(run: Runner, version: str, out: Path, nfpm: str) -> list[Path]:
    """The release binaries of every platform and the .deb and .rpm packages (module docstring).

    Raises:
        StepError: the version is not that of services/host/VERSION or a build fails.
    """
    check_version(version)
    out.mkdir(parents=True, exist_ok=True)
    written = [
        go_build(run, goos, goarch, version, "", out / binary_name(version, goos, goarch))
        for goos, goarch in PLATFORMS
    ]
    with tempfile.TemporaryDirectory(prefix="bothy-host-packages-") as tmp:
        for kind, arch in LINUX_PACKAGES:
            stage = Path(tmp) / f"{kind}-{arch}"
            go_build(run, "linux", arch, version, kind, stage / "bothy-host")
            for src in (
                INFRA / "linux" / "bothy-host.service",
                INFRA / "linux" / "postinstall.sh",
                INFRA / "linux" / "preremove.sh",
                INFRA / "linux" / "postremove.sh",
                REPO / "LICENSE",
                REPO / "NOTICE",
            ):
                shutil.copy2(src, stage / src.name)
            target = out / package_name(version, "linux", arch, kind)
            run(
                [
                    nfpm,
                    "package",
                    "--config",
                    INFRA / "linux" / "nfpm.yaml",
                    "--packager",
                    kind,
                    "--target",
                    target,
                ],
                cwd=stage,
                env={"BOTHY_HOST_VERSION": version, "BOTHY_HOST_ARCH": arch},
            )
            if not target.is_file():
                raise StepError(f"nfpm wrote no {target.name}")
            written.append(target)
    return written


# ---- macOS ----------------------------------------------------------------------------------


@dataclass
class ApplePlan:
    """What one macOS package step needs: the identities and, for a release, the notary key.

    Attributes:
        version: the component version.
        out: where the packages go.
        signer: the identity of the binaries (Developer ID Application or the test publisher).
        installer: the identity of the packages (Developer ID Installer); None in the trial,
            because productbuild signs only with an Apple-issued installer certificate.
        team: the team the binary signature must name (None for the test publisher).
        notary: the App Store Connect API key (None in the trial: no notarization).
    """

    version: str
    out: Path
    signer: apple.Signer
    installer: apple.Signer | None
    team: str | None = None
    notary: apple.Notary | None = None


def sign_binary(run: Runner, signer: apple.Signer, binary: Path, team: str | None) -> None:
    """Sign `binary` with the hardened runtime and check it with `codesign --verify --strict`.

    install keeps such a signature and signs only an unsigned binary ad hoc (16 4.4).

    Raises:
        StepError: the signature does not verify or names another team.
    """
    run(
        [
            "codesign",
            "--force",
            "--options",
            "runtime",
            "--keychain",
            signer.keychain,
            "--sign",
            signer.identity,
            "--timestamp" if signer.timestamp else "--timestamp=none",
            binary,
        ]
    )
    run(["codesign", "--verify", "--strict", "--verbose=2", binary])
    details = run(["codesign", "-dv", "--verbose=2", binary], capture=True)
    text = details.stdout + details.stderr
    if "flags=0x10000(runtime)" not in text:
        raise StepError(f"{binary.name} carries no hardened runtime")
    if team is not None and f"TeamIdentifier={team}" not in text:
        raise StepError(f"{binary.name} is not signed by the team {team}")


def host_component(run: Runner, version: str, arch: str, binary: Path, workdir: Path) -> Path:
    """Build the component package of the host service from `binary`; return its path.

    One component installed below `/Library/Application Support`, so the installer touches no
    existing folder: `bothy-host/bin/bothy-host` and the reference copy of the LaunchDaemon in
    `bothy-host/share`, owned by root (`--ownership recommended`). launchd never loads the
    reference; `bothy-host install` writes the live LaunchDaemon, which an upgrade leaves alone.
    """
    root = workdir / f"pkg-{arch}" / "root"
    program = root / "bothy-host" / "bin"
    share = root / "bothy-host" / "share"
    program.mkdir(parents=True)
    share.mkdir(parents=True)
    shutil.copy2(binary, program / "bothy-host")
    (program / "bothy-host").chmod(0o755)
    shutil.copy2(INFRA / "macos" / "bothy-host.plist", share / DARWIN_PLIST.name)
    (share / DARWIN_PLIST.name).chmod(0o644)
    component = workdir / f"pkg-{arch}" / f"{PKG_IDENTIFIER}.pkg"
    run(
        [
            "pkgbuild",
            "--root",
            root,
            "--identifier",
            PKG_IDENTIFIER,
            "--version",
            version,
            "--install-location",
            DARWIN_PROGRAM_DIR.parent.parent,
            "--ownership",
            "recommended",
            component,
        ]
    )
    return component


def build_pkg(run: Runner, plan: ApplePlan, arch: str, binary: Path, workdir: Path) -> Path:
    """Build the .pkg of one architecture from `binary` (already signed); signed with an installer.

    The product holds the one component of `host_component`.
    """
    components = [host_component(run, plan.version, arch, binary, workdir)]
    pkg = plan.out / package_name(plan.version, "darwin", arch, KIND_PKG)
    plan.out.mkdir(parents=True, exist_ok=True)
    cmd = ["productbuild"]
    for component in components:
        cmd += ["--package", str(component)]
    if plan.installer is not None:
        cmd += ["--sign", plan.installer.identity, "--keychain", str(plan.installer.keychain)]
        cmd += ["--timestamp"] if plan.installer.timestamp else ["--timestamp=none"]
    run([*cmd, pkg])
    return pkg


def notarize_pkg(run: Runner, pkg: Path, notary: apple.Notary) -> None:
    """Notarize `pkg` with notarytool, staple the ticket and check that Gatekeeper accepts it.

    Raises:
        StepError: notarytool does not answer Accepted or `spctl` rejects the package.
    """
    done = run(
        [
            "xcrun",
            "notarytool",
            "submit",
            pkg,
            "--key",
            notary.key,
            "--key-id",
            notary.key_id,
            "--issuer",
            notary.issuer,
            "--wait",
            "--output-format",
            "json",
        ],
        capture=True,
    )
    status = json.loads(done.stdout or "{}").get("status")
    if status != "Accepted":
        raise StepError(f"notarization of {pkg.name} ended with {status!r}")
    run(["xcrun", "stapler", "staple", pkg])
    run(["xcrun", "stapler", "validate", pkg])
    accepted, answer = assess_pkg(run, pkg)
    if not accepted:
        raise StepError(f"spctl rejects {pkg.name}: {answer}")


def assess_pkg(run: Runner, pkg: Path) -> tuple[bool, str]:
    """Run `spctl --assess --type install` on `pkg`; return whether it accepts and its answer."""
    done = run(
        ["spctl", "--assess", "--type", "install", "--verbose=4", pkg], capture=True, check=False
    )
    return done.returncode == 0, (done.stdout + done.stderr).strip()


def apple_packages(run: Runner, plan: ApplePlan, workdir: Path) -> list[Path]:
    """Build, sign and package the binaries of both architectures; notarize with a notary key.

    Returns:
        The packages, one per architecture of APPLE_ARCHES.
    """
    pkgs = []
    for arch in APPLE_ARCHES:
        binary = go_build(
            run, "darwin", arch, plan.version, KIND_PKG, workdir / f"bin-{arch}" / "bothy-host"
        )
        sign_binary(run, plan.signer, binary, plan.team)
        pkg = build_pkg(run, plan, arch, binary, workdir)
        if plan.notary is not None:
            notarize_pkg(run, pkg, plan.notary)
        pkgs.append(pkg)
    return pkgs


def apple_release(
    run: Runner, *, version: str, out: Path, distribution: Path, environ: Mapping[str, str]
) -> list[Path]:
    """The step of release.yml job build-apple with the publisher's accounts (module docstring).

    Raises:
        StepError: no macOS host, a secret is missing, or a step fails.
        appdist.DistributionError: the distribution holds placeholders or is malformed.
    """
    apple.require_macos()
    secret = require_env(APPLE_SECRETS, environ, "release-apple")
    check_version(version)
    cfg = appdist.load(distribution)
    appdist.require_publisher_values(cfg)
    with tempfile.TemporaryDirectory(prefix="release-host-apple-") as tmp:
        workdir = Path(tmp)
        app_p12 = _b64file(
            workdir, "developer-id-application.p12", secret["APPLE_DEVELOPER_ID_P12"]
        )
        inst_p12 = _b64file(workdir, "developer-id-installer.p12", secret["APPLE_INSTALLER_P12"])
        notary = apple.Notary(
            key=_b64file(workdir, "notary.p8", secret["APPLE_NOTARY_KEY"]),
            key_id=secret["APPLE_NOTARY_KEY_ID"],
            issuer=secret["APPLE_NOTARY_ISSUER_ID"],
        )
        with apple.temporary_keychain(run, workdir) as (keychain, password):
            app_id = apple.import_identity(
                run, keychain, password, app_p12, secret["APPLE_DEVELOPER_ID_P12_PASSWORD"]
            )
            inst_id = installer_identity(
                run, keychain, password, inst_p12, secret["APPLE_INSTALLER_P12_PASSWORD"]
            )
            plan = ApplePlan(
                version=version,
                out=out,
                signer=apple.Signer(identity=app_id, keychain=keychain, timestamp=True),
                installer=apple.Signer(identity=inst_id, keychain=keychain, timestamp=True),
                team=cfg["apple"]["team_id"],
                notary=notary,
            )
            return apple_packages(run, plan, workdir)


def installer_identity(
    run: Runner, keychain: Path, password: str, p12: Path, p12_password: str
) -> str:
    """Import the installer identity into `keychain`; return its name for productbuild.

    productbuild takes the name of the identity (for example "Developer ID Installer: <name>
    (<team>)"), not its hash, and `security find-identity -p codesigning` lists no installer
    certificate, so the name is the new one among all identities of the keychain (policy basic).

    Raises:
        StepError: the keychain holds no identity of that certificate afterwards.
    """
    before = _identities(run, keychain)
    run(
        [
            "security",
            "import",
            p12,
            "-k",
            keychain,
            "-P",
            p12_password,
            "-T",
            "/usr/bin/productbuild",
        ],
        redact=[p12_password],
    )
    run(
        [
            "security",
            "set-key-partition-list",
            "-S",
            "apple-tool:,apple:",
            "-s",
            "-k",
            password,
            keychain,
        ],
        capture=True,
        redact=[password],
    )
    new = [i for i in _identities(run, keychain) if i not in before]
    if not new:
        raise StepError("the keychain holds no installer identity")
    return new[0]


def _identities(run: Runner, keychain: Path) -> list[str]:
    """Return the names of the identities of `keychain` (`security find-identity -p basic`)."""
    listing = run(["security", "find-identity", "-p", "basic", keychain], capture=True)
    out = []
    for line in listing.stdout.splitlines():
        m = re.match(r'^\s*\d+\) [0-9A-F]{40} "(.+)"', line)
        if m and m.group(1) not in out:
            out.append(m.group(1))
    return out


def _b64file(workdir: Path, name: str, text: str) -> Path:
    """Write a base64 secret of the release environment to a file of the run (mode 0600).

    The same writer as the desktop release (apple.py), so the signing material of both chains
    takes one path to disk inside the temporary directory of the run.
    """
    return apple._b64file(workdir, name, text)


def _stat_line(run: Runner, path: Path | str) -> str:
    """Return `<owner>:<group> <octal mode>` of a path as `stat` on macOS reports it."""
    return run(["stat", "-f", "%Su:%Sg %Lp", path], capture=True).stdout.strip()


def check_installed_apple(run: Runner, version: str) -> list[str]:
    """Checks after `installer -pkg` on the runner (brief Design note 1); return them.

    Raises:
        StepError: an owner, a mode, the version or the signature is wrong, the package wrote the
            live LaunchDaemon, or launchd runs the host service.
    """
    binary = DARWIN_PROGRAM_DIR / "bothy-host"
    want = {
        DARWIN_PROGRAM_DIR.parent: "root:wheel 755",
        DARWIN_PROGRAM_DIR: "root:wheel 755",
        binary: "root:wheel 755",
        DARWIN_SHARE: "root:wheel 755",
        DARWIN_SHARE / DARWIN_PLIST.name: "root:wheel 644",
    }
    checks = []
    for path, expected in want.items():
        got = _stat_line(run, path)
        if got != expected:
            raise StepError(f"{path}: {got}, want {expected}")
        checks.append(f"{path} {got}")
    printed = run([binary, "version"], capture=True).stdout.strip()
    if printed != version:
        raise StepError(f"the installed binary prints {printed!r}, want {version}")
    checks.append(f"bothy-host version prints {version}")
    run(["codesign", "--verify", "--strict", "--verbose=2", binary])
    checks.append("codesign --verify --strict passes for the installed binary")
    if run(["test", "-e", DARWIN_PLIST], check=False).returncode == 0:
        raise StepError(f"the package wrote {DARWIN_PLIST}, which only bothy-host install writes")
    loaded = run(["launchctl", "print", "system/bothy-host"], capture=True, check=False)
    if loaded.returncode == 0:
        raise StepError("launchd runs the host service before bothy-host install")
    checks.append(f"no {DARWIN_PLIST} and nothing loaded in launchd before bothy-host install")
    return checks


def apple_trial(run: Runner, *, out: Path) -> list[str]:
    """The macOS chain with a test publisher and an installation on the runner; return checks.

    Raises:
        StepError: no macOS host or a step or check fails.
    """
    apple.require_macos()
    version = component_version()
    out.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="trial-host-apple-") as tmp:
        workdir = Path(tmp)
        with apple.trial_publisher(run, workdir) as signer:
            plan = ApplePlan(version=version, out=out, signer=signer, installer=None)
            pkgs = apple_packages(run, plan, workdir)
        checks = [
            f"codesign --verify --strict passes for the pkg binaries of {', '.join(APPLE_ARCHES)}"
        ]
        native = "arm64" if platform.machine() == "arm64" else "amd64"
        pkg = next(p for p in pkgs if f"_darwin_{native}." in p.name)
        signature = run(["pkgutil", "--check-signature", pkg], capture=True, check=False)
        assessed, answer = assess_pkg(run, pkg)
        run(["sudo", "-n", "installer", "-pkg", pkg, "-target", "/"])
        checks += check_installed_apple(run, version)
    report = {
        "version": version,
        "packages": [p.name for p in pkgs],
        "checks": checks,
        "pkgutil_check_signature": (signature.stdout + signature.stderr).strip(),
        "spctl": {"accepted": assessed, "answer": answer},
        "not_proven": [
            "Developer ID signature",
            "signature of the .pkg",
            "notarization",
            "spctl acceptance",
        ],
    }
    (out / "trial-host-apple.json").write_text(
        json.dumps(report, indent=2) + "\n", encoding="utf-8"
    )
    return checks


# ---- Windows --------------------------------------------------------------------------------


def msi_version(version: str) -> str:
    """Return the MSI ProductVersion of a SemVer: `X.Y.Z` without pre-release and build parts.

    Raises:
        StepError: a part exceeds what Windows Installer accepts (255, 255, 65535).
    """
    m = _SEMVER.match(version)
    if not m:
        raise StepError(f"version {version!r} is not SemVer without a v")
    major, minor, patch = (int(m.group(i)) for i in (1, 2, 3))
    if major > 255 or minor > 255 or patch > 65535:
        raise StepError(f"{version} exceeds the MSI version range 255.255.65535")
    return f"{major}.{minor}.{patch}"


def fetch_wix(cache: Path = WIX_CACHE, download: Any = None) -> Path:
    """Return `wix.dll` of the pinned WiX Toolset, downloaded and checked once into `cache`.

    Raises:
        StepError: the download does not match WIX_SHA256.
    """
    target = cache / WIX_VERSION
    dll = target / "tools" / "net6.0" / "any" / "wix.dll"
    stamp = target / ".sha256"
    if dll.is_file() and stamp.is_file() and stamp.read_text(encoding="utf-8") == WIX_SHA256:
        return dll
    cache.mkdir(parents=True, exist_ok=True)
    nupkg = cache / f"wix.{WIX_VERSION}.nupkg"
    (download or _download)(WIX_URL, nupkg)
    digest = hashlib.sha256(nupkg.read_bytes()).hexdigest()
    if digest != WIX_SHA256:
        nupkg.unlink()
        raise StepError(f"wix.{WIX_VERSION}.nupkg has the SHA-256 {digest}, want {WIX_SHA256}")
    shutil.rmtree(target, ignore_errors=True)
    with zipfile.ZipFile(nupkg) as archive:
        archive.extractall(target)
    stamp.write_text(WIX_SHA256, encoding="utf-8")
    if not dll.is_file():
        raise StepError(f"wix.{WIX_VERSION}.nupkg holds no {dll.name}")
    return dll


def _download(url: str, target: Path) -> None:  # pragma: no cover - network
    if not url.startswith("https://api.nuget.org/"):
        raise StepError(f"refusing to download {url}")
    with urllib.request.urlopen(url, timeout=120) as resp:  # noqa: S310  # nosec B310
        target.write_bytes(resp.read())


def build_msi(run: Runner, version: str, binary: Path, out: Path, wix: Path) -> Path:
    """Build the MSI of infra/host/windows/bothy-host.wxs around `binary` (already signed)."""
    msi = out / package_name(version, "windows", "amd64", KIND_MSI)
    out.mkdir(parents=True, exist_ok=True)
    run(
        [
            "dotnet",
            wix,
            "build",
            INFRA / "windows" / "bothy-host.wxs",
            "-arch",
            "x64",
            "-d",
            f"ProductVersion={msi_version(version)}",
            "-d",
            f"BinaryPath={binary}",
            "-o",
            msi,
        ]
    )
    return msi


def windows_package(
    run: Runner, version: str, out: Path, cert: windows.Certificate, workdir: Path
) -> Path:
    """Build and sign the binary of the package kind msi, then the MSI; return the MSI.

    Raises:
        StepError: a build fails or `signtool verify /pa` rejects the binary or the MSI.
    """
    binary = go_build(
        run, "windows", "amd64", version, KIND_MSI, workdir / "msi-bin" / "bothy-host.exe"
    )
    windows.sign(run, binary, cert)
    windows.verify(run, binary)
    msi = build_msi(run, version, binary, out, fetch_wix())
    windows.sign(run, msi, cert)
    windows.verify(run, msi)
    return msi


def windows_release(
    run: Runner, *, version: str, out: Path, distribution: Path, environ: Mapping[str, str]
) -> Path:
    """The step of release.yml job build-windows with the publisher's certificate.

    Raises:
        StepError: no Windows host, a secret is missing, or a step fails.
        appdist.DistributionError: the distribution holds placeholders or is malformed.
    """
    windows.require_windows()
    secret = require_env(WINDOWS_SECRETS, environ, "release-windows")
    check_version(version)
    cfg = appdist.load(distribution)
    appdist.require_publisher_values(cfg)
    with tempfile.TemporaryDirectory(prefix="release-host-windows-") as tmp:
        workdir = Path(tmp)
        pfx = _b64file(workdir, "signing.pfx", secret["WINDOWS_SIGNING_PFX"])
        cert = windows.Certificate(
            pfx=pfx,
            password=secret["WINDOWS_SIGNING_PFX_PASSWORD"],
            timestamp_url=cfg["windows"].get("timestamp_url", ""),
        )
        return windows_package(run, version, out, cert, workdir)


# PowerShell of the Windows checks; the paths come in through the environment, never inline.
_PS_OWNER = "(Get-Acl -LiteralPath $env:CHECK_PATH).Owner"
# Allow entries of everyone but the trusted owners that grant a write right on the path.
_PS_WRITERS = (
    "$trusted = @('NT AUTHORITY\\SYSTEM', 'BUILTIN\\Administrators', "
    "'NT SERVICE\\TrustedInstaller'); "
    "(Get-Acl -LiteralPath $env:CHECK_PATH).Access | Where-Object { "
    "$_.AccessControlType -eq 'Allow' -and $trusted -notcontains $_.IdentityReference.Value -and "
    "($_.FileSystemRights.ToString() -match "
    "'Write|Modify|FullControl|Delete|ChangePermissions|TakeOwnership') "
    "} | ForEach-Object { $_.IdentityReference.Value + ' ' + $_.FileSystemRights }"
)
_PS_SERVICE = (
    "$s = Get-CimInstance Win32_Service -Filter \"Name='bothy-host'\"; "
    "if ($s) { $s.StartMode + ' ' + $s.State + ' ' + $s.StartName } else { 'missing' }"
)
_TRUSTED_OWNERS = (
    "BUILTIN\\Administrators",
    "NT AUTHORITY\\SYSTEM",
    "NT SERVICE\\TrustedInstaller",
)


def _msiexec(run: Runner, action: str, msi: Path, log: Path) -> None:
    """Run msiexec quietly and wait for it; raise StepError on any exit code but 0 and 3010."""
    code = windows.powershell(
        run,
        "$p = Start-Process -FilePath msiexec.exe -Wait -PassThru -ArgumentList "
        f"@('/{action}', $env:MSI_PATH, '/qn', '/norestart', '/l*v', $env:MSI_LOG); $p.ExitCode",
        env={"MSI_PATH": str(msi), "MSI_LOG": str(log)},
    )
    if code.strip() not in ("0", "3010"):
        raise StepError(f"msiexec /{action} {msi.name} exited with {code.strip()!r} (log {log})")


def check_installed_windows(run: Runner, version: str, program_dir: Path) -> list[str]:
    """Checks after the MSI installed (brief Design note 1); return them.

    Raises:
        StepError: an owner, a write right, the service, the version or the signature is wrong.
    """
    exe = program_dir / "bothy-host.exe"
    checks = []
    for path in (program_dir, exe):
        owner = windows.powershell(run, _PS_OWNER, env={"CHECK_PATH": str(path)})
        if owner not in _TRUSTED_OWNERS:
            raise StepError(f"{path} belongs to {owner!r}, not to a trusted owner")
        writers = windows.powershell(run, _PS_WRITERS, env={"CHECK_PATH": str(path)})
        if writers:
            raise StepError(f"{path} is writable for {writers}")
        checks.append(f"{path} owner {owner}, writable by the trusted owners only")
    service = windows.powershell(run, _PS_SERVICE)
    if service != "Manual Stopped NT SERVICE\\bothy-host":
        raise StepError(
            f"service bothy-host: {service!r}, want Manual Stopped NT SERVICE\\bothy-host"
        )
    checks.append(
        "service bothy-host registered with the start type demand, stopped, "
        "as NT SERVICE\\bothy-host"
    )
    printed = run([exe, "version"], capture=True).stdout.strip()
    if printed != version:
        raise StepError(f"the installed binary prints {printed!r}, want {version}")
    checks.append(f"bothy-host version prints {version}")
    windows.verify(run, exe)
    checks.append("signtool verify /pa passes for the installed binary")
    return checks


def windows_trial(run: Runner, *, out: Path) -> list[str]:
    """The Windows chain with a test publisher, an install and an uninstall; return the checks.

    Raises:
        StepError: no Windows host or a step or check fails.
    """
    windows.require_windows()
    version = component_version()
    out.mkdir(parents=True, exist_ok=True)
    program_dir = Path(os.environ.get("PROGRAMFILES", r"C:\Program Files")) / "bothy-host"
    state_dir = Path(os.environ.get("PROGRAMDATA", r"C:\ProgramData")) / "bothy-host"
    with tempfile.TemporaryDirectory(prefix="trial-host-windows-") as tmp:
        workdir = Path(tmp)
        with windows.trial_certificate(run, TRIAL_SUBJECT, workdir) as cert:
            msi = windows_package(run, version, out, cert, workdir)
            checks = ["signtool verify /pa passes for the binary and the MSI"]
            _msiexec(run, "i", msi, out / "install.log")
            try:
                checks += check_installed_windows(run, version, program_dir)
                # The state folder of install (here a marker): an uninstall keeps it (16 4.4).
                state_dir.mkdir(parents=True, exist_ok=True)
                (state_dir / "trial-marker").write_text("state\n", encoding="utf-8")
                _msiexec(run, "x", msi, out / "uninstall.log")
                if (program_dir / "bothy-host.exe").exists():
                    raise StepError("the uninstall left bothy-host.exe")
                if windows.powershell(run, _PS_SERVICE) != "missing":
                    raise StepError("the uninstall left the service bothy-host")
                if not (state_dir / "trial-marker").is_file():
                    raise StepError("the uninstall removed the state folder")
                checks.append("the uninstall removes binary and service and keeps the state folder")
            finally:
                if (program_dir / "bothy-host.exe").exists():
                    _msiexec(run, "x", msi, out / "cleanup.log")
                shutil.rmtree(state_dir, ignore_errors=True)
    report = {
        "version": version,
        "msi": msi.name,
        "checks": checks,
        "not_proven": ["a certificate of a certificate authority", "SmartScreen reputation"],
    }
    (out / "trial-host-windows.json").write_text(
        json.dumps(report, indent=2) + "\n", encoding="utf-8"
    )
    return checks


# ---- computer helper (D-CU-01-29) ----------------------------------------------------------


def designated_requirement(run: Runner, app: Path) -> str:
    """Return the line `designated => ...` of `codesign -d -r-` for `app`.

    Raises:
        StepError: codesign prints no designated requirement.
    """
    done = run(["codesign", "-d", "-r-", app], capture=True)
    for line in (done.stdout + done.stderr).splitlines():
        if line.startswith("designated =>"):
            return line.strip()
    raise StepError(f"codesign prints no designated requirement for {app.name}")


def helper_sign_macos(
    run: Runner, *, app: Path, identity: str, requirement_file: Path
) -> list[str]:
    """Sign the helper bundle `app` with a Developer ID identity; return the checks.

    The signature carries the hardened runtime and no timestamp (the helper is not notarized and
    never leaves the machine, D-CU-01-29). Screen recording and accessibility grants follow the
    designated requirement, so a requirement other than the one in `requirement_file` stops the
    step before anyone installs the build (27 9.1); the first signed build writes the file.

    Args:
        run: the command runner.
        app: `Bothy Computer Helper.app` as `bundle.sh` assembled it.
        identity: name or SHA-1 of a Developer ID Application identity in the keychains of the
            session (on the owner's Mac a job in the GUI session, where the login keychain is
            open).
        requirement_file: the designated requirement of the previous signed build.

    Returns:
        The checks that passed, for the log of the step.

    Raises:
        StepError: the signature does not verify, carries no hardened runtime, is not a Developer
            ID, or its designated requirement differs from the recorded one.

    Example:
        helper_sign_macos(Runner(), app=Path("out/Bothy Computer Helper.app"),
                          identity="Developer ID Application: Example (TEAM123456)",
                          requirement_file=Path("signing/requirement.txt"))
    """
    run(
        [
            "codesign",
            "--force",
            "--options",
            "runtime",
            "--timestamp=none",
            "--sign",
            identity,
            app,
        ]
    )
    run(["codesign", "--verify", "--strict", "--verbose=2", app])
    details = run(["codesign", "-dv", "--verbose=2", app], capture=True)
    text = details.stdout + details.stderr
    if "flags=0x10000(runtime)" not in text:
        raise StepError(f"{app.name} carries no hardened runtime")
    if "Authority=Developer ID Application" not in text:
        raise StepError(f"{app.name} is not signed by a Developer ID Application identity")
    checks = ["codesign --verify --strict passes, hardened runtime, Developer ID Application"]
    requirement = designated_requirement(run, app)
    if requirement_file.is_file():
        before = requirement_file.read_text(encoding="utf-8").strip()
        if before != requirement:
            raise StepError(
                f"designated requirement changed: {before!r} became {requirement!r}; the screen "
                "recording and accessibility grants of the installed helper would no longer apply"
            )
        checks.append("designated requirement unchanged")
    else:
        requirement_file.parent.mkdir(parents=True, exist_ok=True)
        requirement_file.write_text(requirement + "\n", encoding="utf-8")
        checks.append(f"designated requirement recorded in {requirement_file}")
    return checks


# The component property list of the helper bundle (`pkgbuild --component-plist`).
_COMPONENT = [
    {
        "RootRelativeBundlePath": f"bothy-host/desktop/{HELPER_APP}",
        "BundleIsRelocatable": False,
        "BundleIsVersionChecked": False,
        "BundleHasStrictIdentifier": True,
        "BundleOverwriteAction": "upgrade",
    }
]


def helper_distribution_xml(version: str) -> str:
    """Return the productbuild distribution of the helper package (D-CU-02-10).

    The choice `host` is always installed; `computer-helper` is off until the person installing
    selects it (in Installer, or with `installer -applyChoiceChangesXML`).
    """
    return f"""<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="2">
    <title>bothy-host {version} with the computer helper</title>
    <options customize="always" require-scripts="false" hostArchitectures="arm64,x86_64"/>
    <domains enable_anywhere="false" enable_currentUserHome="false" enable_localSystem="true"/>
    <choices-outline>
        <line choice="host"/>
        <line choice="computer-helper"/>
    </choices-outline>
    <choice id="host" title="Host service" start_selected="true" enabled="false">
        <pkg-ref id="{PKG_IDENTIFIER}"/>
    </choice>
    <choice id="computer-helper" title="Computer helper" start_selected="false"
            description="An agent of the owner may control this desktop (computer_use).">
        <pkg-ref id="{HELPER_PKG_IDENTIFIER}"/>
    </choice>
    <pkg-ref id="{PKG_IDENTIFIER}" version="{version}">{PKG_IDENTIFIER}.pkg</pkg-ref>
    <pkg-ref id="{HELPER_PKG_IDENTIFIER}" version="{version}">{HELPER_PKG_IDENTIFIER}.pkg</pkg-ref>
</installer-gui-script>
"""


def helper_component(run: Runner, version: str, app: Path, workdir: Path) -> Path:
    """Build the component package of the helper from `app`; return its path.

    Installed below `/Library/Application Support` like the host component: the bundle in
    `bothy-host/desktop` and the LaunchAgent as a reference in `bothy-host/share`, which the
    postinstall copies to `/Library/LaunchAgents` (so the payload touches no system folder). The
    bundle is not relocatable: Installer must not move the update to another copy with the same
    bundle ID, because the peer rule trusts only the installed path (27 9.4).
    """
    base = workdir / "pkg-helper"
    root = base / "root" / "bothy-host"
    desktop = root / "desktop"
    share = root / "share"
    desktop.mkdir(parents=True)
    share.mkdir(parents=True)
    shutil.copytree(app, desktop / HELPER_APP, symlinks=True)
    agent = share / HELPER_LAUNCH_AGENT.name
    shutil.copy2(INFRA / "macos" / HELPER_LAUNCH_AGENT.name, agent)
    agent.chmod(0o644)
    scripts = base / "scripts"
    scripts.mkdir()
    shutil.copy2(INFRA / "macos" / "helper-postinstall", scripts / "postinstall")
    (scripts / "postinstall").chmod(0o755)
    shutil.copy2(INFRA / "set-computer-use.sh", scripts / "set-computer-use.sh")
    (scripts / "set-computer-use.sh").chmod(0o755)
    component_plist = base / "component.plist"
    component_plist.write_bytes(plistlib.dumps(_COMPONENT))
    component = base / f"{HELPER_PKG_IDENTIFIER}.pkg"
    run(
        [
            "pkgbuild",
            "--root",
            base / "root",
            "--component-plist",
            component_plist,
            "--scripts",
            scripts,
            "--identifier",
            HELPER_PKG_IDENTIFIER,
            "--version",
            version,
            "--install-location",
            DARWIN_PROGRAM_DIR.parent.parent,
            "--ownership",
            "recommended",
            component,
        ]
    )
    return component


def helper_package_macos(
    run: Runner, *, version: str, out: Path, app: Path, arch: str, workdir: Path
) -> Path:
    """Build the host `.pkg` of `arch` with the helper as an optional choice; return it.

    The host binary is built with the package kind `pkg` and left unsigned: `bothy-host install`
    signs it ad hoc, as it does for a release binary (16 4.4). The product stays unsigned too: it
    is built and installed on the same machine, so it carries no quarantine flag (D-CU-01-29).

    Raises:
        StepError: a build step fails.
    """
    binary = go_build(
        run, "darwin", arch, version, KIND_PKG, workdir / f"bin-{arch}" / "bothy-host"
    )
    host = host_component(run, version, arch, binary, workdir)
    helper = helper_component(run, version, app, workdir)
    distribution = workdir / "distribution.xml"
    distribution.write_text(helper_distribution_xml(version), encoding="utf-8")
    out.mkdir(parents=True, exist_ok=True)
    pkg = out / f"bothy-host-computer_{version}_darwin_{arch}.pkg"
    run(
        [
            "productbuild",
            "--distribution",
            distribution,
            "--package-path",
            host.parent,
            "--package-path",
            helper.parent,
            pkg,
        ]
    )
    return pkg


# PowerShell of helper-cert-windows; the subject comes in through the environment.
_PS_HELPER_CERT = (
    "$c = New-SelfSignedCertificate -Type CodeSigningCert -Subject $env:CERT_SUBJECT "
    "-KeyExportPolicy NonExportable -KeyUsage DigitalSignature -KeyAlgorithm RSA "
    "-KeyLength 3072 -HashAlgorithm SHA256 -CertStoreLocation Cert:\\LocalMachine\\My; "
    "$cer = Join-Path ([System.IO.Path]::GetTempPath()) ($c.Thumbprint + '.cer'); "
    "try { Export-Certificate -Cert $c -FilePath $cer | Out-Null; "
    "Import-Certificate -FilePath $cer -CertStoreLocation Cert:\\LocalMachine\\Root | Out-Null; "
    "Import-Certificate -FilePath $cer -CertStoreLocation Cert:\\LocalMachine\\TrustedPublisher "
    "| Out-Null } finally { Remove-Item -LiteralPath $cer -ErrorAction SilentlyContinue }; "
    "$c.Thumbprint"
)


def helper_cert_windows(run: Runner, *, subject: str = HELPER_CERT_SUBJECT) -> str:
    """Create the self-signed code signing certificate of this machine; return its thumbprint.

    Run once, elevated, on the owner's Windows computer (D-CU-01-29): the key stays in the
    store `My` of the local machine and cannot be exported; the certificate (without the key)
    goes into the machine's `Root` and `TrustedPublisher` stores, so `signtool verify /pa` and
    `uiAccess` accept a signature with it on this machine and on no other.

    Raises:
        StepError: PowerShell fails or prints no thumbprint.
    """
    printed = windows.powershell(run, _PS_HELPER_CERT, env={"CERT_SUBJECT": subject}).strip()
    thumb = printed.splitlines()[-1].strip() if printed else ""
    if not _THUMBPRINT.match(thumb):
        raise StepError("the new certificate has no thumbprint; nothing to sign with")
    return thumb.upper()


def sign_with_store(run: Runner, path: Path, thumbprint: str) -> None:
    """Sign `path` with the certificate `thumbprint` of the machine store and verify it.

    Raises:
        StepError: the thumbprint is malformed, or signtool fails or rejects the signature.
    """
    if not _THUMBPRINT.match(thumbprint):
        raise StepError("the thumbprint must be the 40 hex digits of the certificate")
    run(
        [
            windows.sdk_tool("signtool.exe"),
            "sign",
            "/sha1",
            thumbprint,
            "/sm",
            "/s",
            "My",
            "/fd",
            "SHA256",
            path,
        ]
    )
    windows.verify(run, path)


def helper_sign_windows(run: Runner, *, exe: Path, thumbprint: str) -> None:
    """Sign `bothy-computer-helper.exe` with the machine's certificate (`helper-cert-windows`).

    Raises:
        StepError: as `sign_with_store`.
    """
    sign_with_store(run, exe, thumbprint)


def helper_package_windows(
    run: Runner,
    *,
    version: str,
    out: Path,
    helper: Path,
    thumbprint: str | None,
    wix: Path,
    workdir: Path,
) -> Path:
    """Build the host MSI with the feature ComputerHelper around `helper`; return the MSI.

    `helper` is signed already (`helper-sign-windows`) or a development build. With a thumbprint
    the host binary and the MSI are signed with the same certificate; without one they stay
    unsigned, as in the CI jobs.

    Raises:
        StepError: a build or signing step fails.
    """
    binary = go_build(
        run, "windows", "amd64", version, KIND_MSI, workdir / "msi-bin" / "bothy-host.exe"
    )
    if thumbprint:
        sign_with_store(run, binary, thumbprint)
    msi = out / f"bothy-host-computer_{version}_windows_amd64.msi"
    out.mkdir(parents=True, exist_ok=True)
    run(
        [
            "dotnet",
            wix,
            "build",
            INFRA / "windows" / "bothy-host.wxs",
            "-arch",
            "x64",
            "-d",
            f"ProductVersion={msi_version(version)}",
            "-d",
            f"BinaryPath={binary}",
            "-d",
            f"HelperPath={helper}",
            "-d",
            f"HelperScript={INFRA / 'windows' / 'set-computer-use.ps1'}",
            "-o",
            msi,
        ]
    )
    if thumbprint:
        sign_with_store(run, msi, thumbprint)
    return msi


def _add_helper_parsers(sub: Any) -> None:
    """Add the helper subcommands of the module docstring to `sub` (D-CU-01-29)."""
    hs = sub.add_parser("helper-sign-macos", help="sign the computer helper (Developer ID)")
    hs.add_argument("--app", type=Path, required=True)
    hs.add_argument("--identity", required=True)
    hs.add_argument("--requirement-file", type=Path, required=True)
    hp = sub.add_parser("helper-package-macos", help="host .pkg with the computer helper")
    hp.add_argument("--version", required=True)
    hp.add_argument("--out", type=Path, required=True)
    hp.add_argument("--app", type=Path, required=True)
    hp.add_argument("--arch", default="arm64" if platform.machine() == "arm64" else "amd64")
    hc = sub.add_parser("helper-cert-windows", help="create the machine's signing certificate")
    hc.add_argument("--subject", default=HELPER_CERT_SUBJECT)
    hw = sub.add_parser("helper-sign-windows", help="sign the computer helper (machine store)")
    hw.add_argument("--exe", type=Path, required=True)
    hw.add_argument("--thumbprint", required=True)
    hm = sub.add_parser("helper-package-windows", help="host MSI with the computer helper")
    hm.add_argument("--version", required=True)
    hm.add_argument("--out", type=Path, required=True)
    hm.add_argument("--helper", type=Path, required=True)
    hm.add_argument("--thumbprint", default="")


def _helper_command(runner: Runner, args: argparse.Namespace) -> None:
    """Run one helper subcommand of `main` (D-CU-01-29)."""
    if args.command == "helper-sign-macos":
        apple.require_macos()
        for line in helper_sign_macos(
            runner,
            app=args.app.resolve(),
            identity=args.identity,
            requirement_file=args.requirement_file.resolve(),
        ):
            print(f"helper-sign-macos: {line}")
    elif args.command == "helper-package-macos":
        apple.require_macos()
        check_version(args.version)
        with tempfile.TemporaryDirectory(prefix="helper-package-macos-") as tmp:
            pkg = helper_package_macos(
                runner,
                version=args.version,
                out=args.out.resolve(),
                app=args.app.resolve(),
                arch=args.arch,
                workdir=Path(tmp),
            )
        print(f"helper-package-macos: {pkg.name}")
    elif args.command == "helper-cert-windows":
        windows.require_windows()
        print(helper_cert_windows(runner, subject=args.subject))
    elif args.command == "helper-sign-windows":
        windows.require_windows()
        helper_sign_windows(runner, exe=args.exe.resolve(), thumbprint=args.thumbprint)
        print(f"helper-sign-windows: {args.exe.name} signed and verified")
    else:
        windows.require_windows()
        check_version(args.version)
        with tempfile.TemporaryDirectory(prefix="helper-package-windows-") as tmp:
            msi = helper_package_windows(
                runner,
                version=args.version,
                out=args.out.resolve(),
                helper=args.helper.resolve(),
                thumbprint=args.thumbprint or None,
                wix=fetch_wix(),
                workdir=Path(tmp),
            )
        print(f"helper-package-windows: {msi.name}")


# ---- command line ---------------------------------------------------------------------------


def main(argv: Sequence[str] | None = None, run: Runner | None = None) -> int:
    """Run one subcommand of the module docstring; return the exit code."""
    parser = argparse.ArgumentParser(description="Release steps of the host service bothy-host.")
    sub = parser.add_subparsers(dest="command", required=True)
    b = sub.add_parser("build", help="release binaries and Linux packages")
    b.add_argument("--version", required=True)
    b.add_argument("--out", type=Path, required=True)
    b.add_argument("--nfpm", default="nfpm")
    for name in ("apple-release", "windows-release"):
        r = sub.add_parser(name, help="sign and package with the publisher's accounts")
        r.add_argument("--version", required=True)
        r.add_argument("--out", type=Path, required=True)
        r.add_argument("--distribution", type=Path, required=True)
    for name in ("apple-trial", "windows-trial"):
        t = sub.add_parser(name, help="the chain with a test publisher and an installation")
        t.add_argument("--out", type=Path, required=True)
    _add_helper_parsers(sub)
    args = parser.parse_args(argv)
    runner = run or Runner()
    try:
        if args.command == "build":
            written = build(runner, args.version, args.out.resolve(), args.nfpm)
            print(f"release-build: {len(written)} files in {args.out}")
        elif args.command == "apple-release":
            pkgs = apple_release(
                runner,
                version=args.version,
                out=args.out.resolve(),
                distribution=args.distribution,
                environ=os.environ,
            )
            print(f"release-apple: {', '.join(p.name for p in pkgs)}")
        elif args.command == "windows-release":
            msi = windows_release(
                runner,
                version=args.version,
                out=args.out.resolve(),
                distribution=args.distribution,
                environ=os.environ,
            )
            print(f"release-windows: {msi.name}")
        elif args.command.startswith("helper-"):
            _helper_command(runner, args)
        elif args.command == "apple-trial":
            checks = apple_trial(runner, out=args.out.resolve())
            print(f"host trial (macOS): {len(checks)} checks passed")
        else:
            checks = windows_trial(runner, out=args.out.resolve())
            print(f"host trial (Windows): {len(checks)} checks passed")
    except (StepError, appdist.DistributionError) as err:
        return fail(str(err))
    return 0


if __name__ == "__main__":
    sys.exit(main())

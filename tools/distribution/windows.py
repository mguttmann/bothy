# SPDX-License-Identifier: Apache-2.0
"""Windows release of the desktop app as MSIX with App Installer (brief M7-12, ADR 0037).

`make -C apps/client release-windows VERSION=<X.Y.Z> OUT=<dir>` (release.yml job build-windows):

1. the build branding with the publisher's bundle identifier (`branding_for`), then
   `flutter build windows --release` with the version and build number of
   `appdist.app_version`;
2. the package directory: the build output, the Visual C++ runtime the app loads (copied next
   to it, so the package needs no framework dependency), `AppxManifest.xml` from
   `appdist.appx_manifest` (identity, toast activator, URL scheme) and the logos taken from
   the app icon;
3. `makeappx pack` and `signtool sign` with the publisher's certificate (and its timestamp
   server when `windows.timestamp_url` names one), then `signtool verify /pa`;
4. `beta.appinstaller` pointing at the new MSIX (`appdist.appinstaller`); promotion writes
   `stable.appinstaller` later (`appdist.py promote-appinstaller`).

The certificate comes from the environment `release-windows` (WINDOWS_SECRETS); without it the
step stops before the build, and template placeholders stop it too.

`make -C apps/client trial-windows` runs the same steps with a test publisher (D-W-127, brief
Design note 1 and 2): a self-signed certificate whose subject is the template's
`windows.publisher`, trusted in the machine's TrustedPeople and Root stores for the run only,
and the update host on 127.0.0.1. It builds 0.1.0 and 0.1.1 and then:

- installs 0.1.0 through `beta.appinstaller` (`Add-AppxPackage -AppInstallerFile`);
- refuses a changed 0.1.1 MSIX that `beta.appinstaller` points at, and 0.1.0 stays;
- points `beta.appinstaller` at 0.1.1 and runs the same App Installer call, which updates the
  installed package to 0.1.1;
- promotes it (`stable.appinstaller` points at the same MSIX) and installs from there afresh.

The result is written to `<out>/trial-windows.json`; packages and certificates of the run are
removed at the end, also after a failure.

Usage:
  windows.py release --version X.Y.Z --out DIR [--distribution FILE] [--branding FILE]
  windows.py trial --out DIR [--branding FILE]
Exit codes: 0 done, 1 a step failed (named on stderr), 2 usage error.
"""

from __future__ import annotations

import argparse
import base64
import contextlib
import json
import os
import re
import secrets
import shutil
import struct
import sys
import tempfile
from collections.abc import Iterator, Mapping
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import appdist
import yaml
from common import (
    CLIENT,
    DEFAULT_BRANDING,
    Runner,
    StepError,
    branded_tree,
    branding_for,
    fail,
    require_env,
    serve,
)

WINDOWS_SECRETS = ("WINDOWS_SIGNING_PFX", "WINDOWS_SIGNING_PFX_PASSWORD")
TRIAL_VERSIONS = ("0.1.0", "0.1.1")
# The Visual C++ runtime a Flutter Windows app loads; copied into the package (app-local
# deployment), so the MSIX installs without a framework package.
VC_RUNTIME = ("msvcp140.dll", "vcruntime140.dll", "vcruntime140_1.dll")
LOGOS = ("StoreLogo.png", "Square150x150Logo.png", "Square44x44Logo.png")
ICON = CLIENT / "windows" / "runner" / "resources" / "app_icon.ico"
SDK_BIN = Path(r"C:\Program Files (x86)\Windows Kits\10\bin")
SYSTEM32 = Path(os.environ.get("SYSTEMROOT", r"C:\Windows")) / "System32"


@dataclass
class Certificate:
    """A code signing certificate of this run (a PFX file and its password)."""

    pfx: Path
    password: str
    timestamp_url: str = ""


@dataclass
class Release:
    """What one Windows release step produced."""

    version: appdist.AppVersion
    msix: Path
    appinstaller: Path


def require_windows() -> None:
    """Stop unless this is a Windows host (MSIX tools and App Installer).

    Raises:
        StepError: on any other system.
    """
    if sys.platform != "win32":
        raise StepError("release-windows and trial-windows need a Windows host")


def sdk_tool(name: str, sdk_bin: Path = SDK_BIN) -> Path:
    """Return the x64 `name` (makeappx.exe, signtool.exe) of the newest Windows SDK.

    Raises:
        StepError: no Windows SDK with that tool.
    """

    def version(path: Path) -> tuple[int, ...]:
        return tuple(int(p) for p in path.parent.parent.name.split("."))

    found = [
        p
        for p in sdk_bin.glob(f"10.*/x64/{name}")
        if re.fullmatch(r"10(\.\d+)+", p.parent.parent.name)
    ]
    if not found:
        raise StepError(f"no Windows SDK with {name} under {sdk_bin}")
    return max(found, key=version)


def icon_png(ico: Path = ICON) -> bytes:
    """Return the largest PNG image of the app icon (an ICO file stores it as is).

    Raises:
        StepError: the icon holds no PNG image.
    """
    data = ico.read_bytes()
    _, _, count = struct.unpack("<HHH", data[:6])
    best: tuple[int, bytes] | None = None
    for i in range(count):
        width, _, _, _, _, _, size, offset = struct.unpack(
            "<BBBBHHII", data[6 + 16 * i : 22 + 16 * i]
        )
        image = data[offset : offset + size]
        if image.startswith(b"\x89PNG\r\n\x1a\n"):
            pixels = width or 256
            if best is None or pixels > best[0]:
                best = (pixels, image)
    if best is None:
        raise StepError(f"{ico} holds no PNG image for the package logos")
    return best[1]


def build_app(
    run: Runner,
    cfg: dict[str, Any],
    version: appdist.AppVersion,
    branding: Path,
    workdir: Path,
) -> tuple[Path, dict[str, Any]]:
    """Build the Windows app of `version`; return the output directory and the branding used.

    Raises:
        StepError: the build fails.
    """
    release_branding = branding_for(branding, cfg, workdir / "branding.yaml")
    with branded_tree(run, branding, release_branding):
        run(
            [
                "flutter",
                "build",
                "windows",
                "--release",
                f"--build-name={version.short}",
                f"--build-number={version.build}",
                "--dart-define-from-file=build/branding/dart_defines.json",
            ],
            cwd=CLIENT,
        )
    output = CLIENT / "build" / "windows" / "x64" / "runner" / "Release"
    if not output.is_dir():
        raise StepError(f"the Windows build wrote no {output}")
    doc = yaml.safe_load(release_branding.read_text(encoding="utf-8"))
    return output, doc


def stage(
    output: Path,
    cfg: dict[str, Any],
    version: appdist.AppVersion,
    branding: Mapping[str, Any],
    package: Path,
    system32: Path = SYSTEM32,
) -> Path:
    """Lay out the package directory of the MSIX from the build `output`; return it.

    Raises:
        StepError: the build has not exactly one executable, or the Visual C++ runtime is
            neither in the build nor in `system32`.
    """
    shutil.rmtree(package, ignore_errors=True)
    shutil.copytree(output, package)
    executables = sorted(p.name for p in package.glob("*.exe"))
    if len(executables) != 1:
        raise StepError(f"expected one executable in {output}, found {executables}")
    for dll in VC_RUNTIME:
        if not (package / dll).is_file():
            source = system32 / dll
            if not source.is_file():
                raise StepError(f"{dll} is neither in the build nor in {system32}")
            shutil.copy2(source, package / dll)
    assets = package / "Assets"
    assets.mkdir(exist_ok=True)
    logo = icon_png()
    for name in LOGOS:
        (assets / name).write_bytes(logo)
    manifest = appdist.appx_manifest(
        cfg,
        version=version,
        app_name=str(branding["app_name"]),
        url_scheme=str(branding["url_scheme"]),
        executable=executables[0],
    )
    (package / "AppxManifest.xml").write_text(manifest, encoding="utf-8")
    return package


def pack(run: Runner, package: Path, msix: Path) -> None:
    """Pack `package` into `msix` with makeappx."""
    run([sdk_tool("makeappx.exe"), "pack", "/o", "/d", package, "/p", msix])


def sign(run: Runner, msix: Path, cert: Certificate) -> None:
    """Sign `msix` with `cert` (SHA-256, with a timestamp when the certificate names a server)."""
    cmd: list[str | Path] = [
        sdk_tool("signtool.exe"),
        "sign",
        "/fd",
        "SHA256",
        "/f",
        cert.pfx,
        "/p",
        cert.password,
    ]
    if cert.timestamp_url:
        cmd += ["/tr", cert.timestamp_url, "/td", "SHA256"]
    run([*cmd, msix], redact=[cert.password])


def verify(run: Runner, msix: Path) -> None:
    """Run `signtool verify /pa`: the signature verifies to a trusted root.

    Raises:
        StepError: it does not.
    """
    run([sdk_tool("signtool.exe"), "verify", "/pa", "/v", msix])


@dataclass
class Plan:
    """Inputs of one Windows release step."""

    cfg: dict[str, Any]
    version: appdist.AppVersion
    out: Path
    branding: Path
    certificate: Certificate


def release_step(run: Runner, plan: Plan, workdir: Path) -> Release:
    """Build, package, sign and verify one version; write its `beta.appinstaller`.

    Raises:
        StepError: a step fails.
    """
    plan.out.mkdir(parents=True, exist_ok=True)
    output, branding = build_app(run, plan.cfg, plan.version, plan.branding, workdir)
    package = stage(output, plan.cfg, plan.version, branding, workdir / "package")
    msix = plan.out / appdist.msix_name(plan.cfg, plan.version)
    pack(run, package, msix)
    sign(run, msix, plan.certificate)
    verify(run, msix)
    appinstaller = plan.out / "beta.appinstaller"
    appinstaller.write_text(
        appdist.appinstaller(plan.cfg, version=plan.version, channel=appdist.BETA),
        encoding="utf-8",
    )
    return Release(plan.version, msix, appinstaller)


def release(
    run: Runner,
    *,
    distribution: Path,
    version: str,
    out: Path,
    branding: Path,
    environ: Mapping[str, str],
) -> Release:
    """The release step of release.yml with the publisher's certificate (module docstring).

    Raises:
        StepError: a secret is missing, the distribution holds placeholders, or a step fails.
        appdist.DistributionError: the distribution is not usable.
    """
    require_windows()
    secret = require_env(WINDOWS_SECRETS, environ, "release-windows")
    cfg = appdist.load(distribution)
    appdist.require_publisher_values(cfg)
    app_version = appdist.app_version(version)
    with tempfile.TemporaryDirectory(prefix="release-windows-") as tmp:
        workdir = Path(tmp)
        pfx = workdir / "signing.pfx"
        pfx.write_bytes(base64.b64decode(secret["WINDOWS_SIGNING_PFX"]))
        cert = Certificate(
            pfx=pfx,
            password=secret["WINDOWS_SIGNING_PFX_PASSWORD"],
            timestamp_url=cfg["windows"].get("timestamp_url", ""),
        )
        plan = Plan(cfg=cfg, version=app_version, out=out, branding=branding, certificate=cert)
        return release_step(run, plan, workdir)


# ---- the trial ------------------------------------------------------------------------------


def powershell(
    run: Runner, script: str, *, env: Mapping[str, str] | None = None, check: bool = True
) -> str:
    """Run `script` in Windows PowerShell (the Appx module); return its standard output.

    Raises:
        StepError: with `check`, the script fails.
    """
    done = run(
        [
            "powershell.exe",
            "-NoProfile",
            "-NonInteractive",
            "-ExecutionPolicy",
            "Bypass",
            "-Command",
            "$ErrorActionPreference = 'Stop'; " + script,
        ],
        env=env,
        capture=True,
        check=check,
    )
    if done.returncode != 0:
        return ""
    return done.stdout.strip()


@contextlib.contextmanager
def trial_certificate(run: Runner, subject: str, workdir: Path) -> Iterator[Certificate]:
    """A self-signed code signing certificate with `subject`, trusted for this run only.

    It is created in the user's store, exported as PFX for signtool and trusted in the
    machine's TrustedPeople and Root stores (App Installer and `signtool verify /pa` need a
    trusted chain); at the end it leaves all three stores, also after a failure.
    """
    pfx = workdir / "trial.pfx"
    cer = workdir / "trial.cer"
    password = secrets.token_urlsafe(16)
    env = {
        "TRIAL_SUBJECT": subject,
        "TRIAL_PFX": str(pfx),
        "TRIAL_CER": str(cer),
        "TRIAL_PW": password,
    }
    thumbprint = (
        powershell(
            run,
            "$c = New-SelfSignedCertificate -Type Custom -Subject $env:TRIAL_SUBJECT "
            "-KeyUsage DigitalSignature -FriendlyName 'Trial Test Publisher' "
            "-CertStoreLocation Cert:\\CurrentUser\\My -NotAfter (Get-Date).AddDays(2) "
            "-TextExtension @('2.5.29.37={text}1.3.6.1.5.5.7.3.3', '2.5.29.19={text}'); "
            "$pw = ConvertTo-SecureString -String $env:TRIAL_PW -Force -AsPlainText; "
            "Export-PfxCertificate -Cert $c -FilePath $env:TRIAL_PFX -Password $pw | Out-Null; "
            "Export-Certificate -Cert $c -FilePath $env:TRIAL_CER | Out-Null; "
            "Import-Certificate -FilePath $env:TRIAL_CER "
            "-CertStoreLocation Cert:\\LocalMachine\\TrustedPeople | Out-Null; "
            "Import-Certificate -FilePath $env:TRIAL_CER "
            "-CertStoreLocation Cert:\\LocalMachine\\Root | Out-Null; "
            "Write-Output $c.Thumbprint",
            env=env,
        )
        .splitlines()[-1]
        .strip()
    )
    if not re.fullmatch(r"[0-9A-F]{40}", thumbprint):
        raise StepError(f"the trial certificate has no thumbprint: {thumbprint!r}")
    try:
        yield Certificate(pfx=pfx, password=password)
    finally:
        for store in ("CurrentUser\\My", "LocalMachine\\TrustedPeople", "LocalMachine\\Root"):
            powershell(run, f"Remove-Item -Path Cert:\\{store}\\{thumbprint} -Force", check=False)


def installed_version(run: Runner, package_name: str) -> str:
    """Return the installed version of the package `package_name` ("" when not installed)."""
    return powershell(
        run,
        f"$p = Get-AppxPackage -Name '{package_name}'; if ($p) {{ Write-Output $p.Version }}",
        check=False,
    )


def install_from(run: Runner, url: str, *, check: bool = True) -> bool:
    """Run App Installer's install and update call with the `.appinstaller` file at `url`."""
    done = powershell(
        run, f"Add-AppxPackage -AppInstallerFile '{url}'; Write-Output installed", check=check
    )
    return done.endswith("installed")


def remove_package(run: Runner, package_name: str) -> None:
    """Remove the package `package_name` for this user when it is installed."""
    powershell(
        run,
        f"Get-AppxPackage -Name '{package_name}' | Remove-AppxPackage",
        check=False,
    )


@dataclass
class TrialLog:
    """The checks a trial passed."""

    checks: list[str] = field(default_factory=list)

    def expect(self, ok: bool, check: str) -> None:
        """Record `check` as passed, or fail the trial with it.

        Raises:
            StepError: `ok` is false.
        """
        if not ok:
            raise StepError(f"trial check failed: {check}")
        print(f"trial: ok: {check}", flush=True)
        self.checks.append(check)


def update_test(
    run: Runner, cfg: dict[str, Any], old: Release, new: Release, www: Path, base_url: str
) -> list[str]:
    """The update test of brief Design note 2 against the served `www` (module docstring).

    Raises:
        StepError: a check fails.
    """
    log = TrialLog()
    name = cfg["windows"]["package_name"]
    for release_ in (old, new):
        shutil.copy2(release_.msix, www / release_.msix.name)
    beta = www / "beta.appinstaller"
    beta_url = f"{base_url}beta.appinstaller"

    shutil.copy2(old.appinstaller, beta)
    install_from(run, beta_url)
    log.expect(
        installed_version(run, name) == old.version.msix,
        f"{old.version.short} installed through beta.appinstaller",
    )

    served = www / new.msix.name
    good = served.read_bytes()
    served.write_bytes(
        good[: len(good) // 2] + bytes([good[len(good) // 2] ^ 0xFF]) + good[len(good) // 2 + 1 :]
    )
    shutil.copy2(new.appinstaller, beta)
    refused = not install_from(run, beta_url, check=False)
    log.expect(
        refused and installed_version(run, name) == old.version.msix,
        f"a changed MSIX is refused and {old.version.short} stays",
    )
    served.write_bytes(good)

    install_from(run, beta_url)
    log.expect(
        installed_version(run, name) == new.version.msix,
        f"App Installer updated {old.version.short} to {new.version.short} from beta.appinstaller",
    )

    stable = www / "stable.appinstaller"
    stable.write_text(appdist.promote_appinstaller(cfg, beta.read_text(encoding="utf-8")), "utf-8")
    remove_package(run, name)
    install_from(run, f"{base_url}stable.appinstaller")
    log.expect(
        installed_version(run, name) == new.version.msix,
        f"stable.appinstaller installs the same MSIX {new.version.short} after promotion",
    )
    return log.checks


def trial(run: Runner, *, out: Path, branding: Path) -> list[str]:
    """Run the trial of the Windows chain with a test publisher; return the passed checks.

    Raises:
        StepError: a step or check fails.
    """
    require_windows()
    out.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="trial-windows-") as tmp:
        workdir = Path(tmp)
        www = workdir / "www"
        www.mkdir()
        with serve(www) as base_url:
            cfg = appdist.trial_distribution(
                base_url, appdist.load(appdist.TEMPLATE)["apple"]["sparkle"]["public_ed_key"]
            )
            name = cfg["windows"]["package_name"]
            with trial_certificate(run, cfg["windows"]["publisher"], workdir) as cert:
                releases = []
                for version in TRIAL_VERSIONS:
                    step = workdir / f"build-{version}"
                    step.mkdir()
                    plan = Plan(
                        cfg=cfg,
                        version=appdist.app_version(version),
                        out=out / version,
                        branding=branding,
                        certificate=cert,
                    )
                    releases.append(release_step(run, plan, step))
                checks = ["signtool verify /pa passes for both builds"]
                try:
                    checks += update_test(run, cfg, releases[0], releases[1], www, base_url)
                finally:
                    remove_package(run, name)
    report = {
        "versions": list(TRIAL_VERSIONS),
        "checks": checks,
        "mechanism": "Add-AppxPackage -AppInstallerFile",
    }
    (out / "trial-windows.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    return checks


def main(argv: list[str] | None = None, run: Runner | None = None) -> int:
    """Run `release` or `trial` (see the module docstring); returns the exit code."""
    parser = argparse.ArgumentParser(description="Windows release and trial of the desktop app.")
    sub = parser.add_subparsers(dest="command", required=True)
    r = sub.add_parser("release", help="build, package and sign the MSIX of one version")
    r.add_argument("--version", required=True)
    r.add_argument("--out", type=Path, required=True)
    r.add_argument("--distribution", type=Path, default=appdist.TEMPLATE)
    r.add_argument("--branding", type=Path, default=DEFAULT_BRANDING)
    t = sub.add_parser("trial", help="the whole chain with a test publisher")
    t.add_argument("--out", type=Path, required=True)
    t.add_argument("--branding", type=Path, default=DEFAULT_BRANDING)
    args = parser.parse_args(argv)
    runner = run or Runner()
    try:
        if args.command == "release":
            result = release(
                runner,
                distribution=args.distribution,
                version=args.version,
                out=args.out,
                branding=args.branding,
                environ=os.environ,
            )
            print(
                f"release-windows: {result.msix.name} and {result.appinstaller.name} in {args.out}"
            )
        else:
            checks = trial(runner, out=args.out, branding=args.branding)
            print(f"trial-windows: {len(checks)} checks passed")
    except (StepError, appdist.DistributionError) as err:
        return fail(str(err))
    return 0


if __name__ == "__main__":
    sys.exit(main())

# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/distribution/android.py with a recording runner instead of the Android tools.

The real chain (Gradle, jarsigner, bundletool, apksigner) runs in `make -C apps/client
trial-android` and in the CI job `mobile trial (Android)`; these tests hold the order of the
steps, what each command gets, every check that must stop a release and the upload's requests.
"""

from __future__ import annotations

import base64
import copy
import io
import json
import zipfile
from pathlib import Path
from typing import Any

import android
import appdist
import common
import pytest
import yaml
from fakes import FakeRun, ok
from test_storeapi import EDITS, FakeHttp, play_routes

KEY_SHA = "ab" * 32
OTHER_SHA = "cd" * 32
PACKAGE = "org.acme.chat"


def colons(hexdigest: str) -> str:
    return ":".join(hexdigest[i : i + 2] for i in range(0, 64, 2)).upper()


@pytest.fixture
def cfg() -> dict[str, Any]:
    doc = copy.deepcopy(appdist.load(appdist.TEMPLATE))
    doc["publisher"]["name"] = "Acme Chat"
    doc["bundle_id"] = doc["apple"]["apns_topic"] = "org.acme.chat"
    doc["apple"]["team_id"] = "AB12CD34EF"
    doc["apple"]["sparkle"]["feed_url"] = "https://updates.acme.test/appcast.xml"
    doc["apple"]["sparkle"]["download_url"] = "https://updates.acme.test/macos/"
    doc["apple"]["sparkle"]["public_ed_key"] = base64.b64encode(bytes(range(32))).decode()
    doc["ios"]["update_url"] = "https://testflight.apple.com/join/AbCdEf12"
    doc["windows"]["package_name"] = "Acme.Chat"
    doc["windows"]["publisher"] = "CN=Acme Chat"
    doc["windows"]["update_url"] = "https://updates.acme.test/windows/"
    doc["android"]["package_name"] = PACKAGE
    doc["android"]["update_url"] = f"https://play.google.com/store/apps/details?id={PACKAGE}"
    doc["firebase"]["project_id"] = "acme-chat"
    appdist.validate(doc)
    assert appdist.placeholders(doc) == []
    return doc


@pytest.fixture
def tree(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """The app's folder of the run: the build writes its bundle there."""
    client = tmp_path / "client"
    (client / "android" / "app").mkdir(parents=True)
    monkeypatch.setattr(common, "CLIENT", client)
    monkeypatch.setattr(android, "CLIENT", client)
    monkeypatch.setattr(android, "GOOGLE_SERVICES", client / "android" / "app" / "gs.json")
    monkeypatch.setattr(
        android, "BUNDLE", client / "build" / "app" / "outputs" / "bundle" / "release" / "a.aab"
    )
    monkeypatch.setattr(android, "apksigner", lambda environ=None: "apksigner")
    return client


def tools(
    version: str = "1.2.3",
    code: int = 1002003,
    package: str = PACKAGE,
    signer: str = KEY_SHA,
    apk_signer: str = KEY_SHA,
    seen: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """Handlers of the Android tools that answer like a correct build of `version`."""
    record = seen if seen is not None else {}

    def flutter(args: list[str], opts: dict[str, Any]) -> None:
        record["flutter"] = (args, opts)
        record["google_services"] = android.GOOGLE_SERVICES.is_file()
        android.BUNDLE.parent.mkdir(parents=True, exist_ok=True)
        android.BUNDLE.write_bytes(b"AAB")

    def keytool(args: list[str], opts: dict[str, Any]) -> Any:
        if "-printcert" in args:
            return ok(f"Certificate fingerprints:\n\t SHA256: {colons(signer)}\n")
        if "-list" in args:
            return ok(f"Alias name: upload\n\t SHA256: {colons(KEY_SHA)}\n")
        record.setdefault("genkeypair", []).append(args)
        return None

    def java(args: list[str], opts: dict[str, Any]) -> Any:
        command = args[3]
        if command == "dump":
            xpath = args[-1].removeprefix("--xpath=")
            return ok(
                {
                    "/manifest/@package": package,
                    "/manifest/@android:versionCode": str(code),
                    "/manifest/@android:versionName": version,
                }[xpath]
                + "\n"
            )
        if command == "build-apks":
            out = Path(next(a for a in args if a.startswith("--output=")).split("=", 1)[1])
            record["passwords"] = [
                Path(a.split("file:", 1)[1]).read_text(encoding="utf-8")
                for a in args
                if "file:" in a
            ]
            buf = io.BytesIO()
            with zipfile.ZipFile(buf, "w") as z:
                z.writestr("universal.apk", b"APK")
            out.write_bytes(buf.getvalue())
        return None

    return {
        "flutter": flutter,
        "keytool": keytool,
        "jarsigner": lambda args, opts: ok("jar verified.\n\nWarning: self-signed\n"),
        "java -jar": java,
        "apksigner verify": lambda args, opts: ok(
            "Signer #1 certificate DN: CN=Upload\n"
            f"Signer #1 certificate SHA-256 digest: {apk_signer}\n"
        ),
    }


def key(tmp_path: Path) -> android.UploadKey:
    store = tmp_path / "upload.p12"
    store.write_bytes(b"store")
    return android.UploadKey(store=store, store_password="sp", alias="upload", key_password="kp")


# ---- tools ----------------------------------------------------------------------------------


def test_fetch_bundletool_checks_the_pinned_checksum(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    def bad(url: str, target: Path) -> None:
        assert url == android.BUNDLETOOL_URL
        target.write_bytes(b"not bundletool")

    with pytest.raises(common.StepError, match="checksum"):
        android.fetch_bundletool(tmp_path, download=bad)
    assert list(tmp_path.iterdir()) == []
    jar = tmp_path / f"bundletool-all-{android.BUNDLETOOL_VERSION}.jar"
    jar.write_bytes(b"x")
    monkeypatch.setattr(
        android, "BUNDLETOOL_SHA256", __import__("hashlib").sha256(b"x").hexdigest()
    )
    assert android.fetch_bundletool(tmp_path, download=bad) == jar


def test_the_bundletool_pin() -> None:
    assert android.BUNDLETOOL_URL.startswith("https://github.com/google/bundletool/releases/")
    assert android.BUNDLETOOL_VERSION in android.BUNDLETOOL_URL
    assert len(android.BUNDLETOOL_SHA256) == 64


def test_java_tools_come_from_java_home(tmp_path: Path) -> None:
    (tmp_path / "bin").mkdir()
    (tmp_path / "bin" / "keytool").write_text("", encoding="utf-8")
    assert android.java_tool("keytool", {"JAVA_HOME": str(tmp_path)}) == str(
        tmp_path / "bin" / "keytool"
    )
    assert android.java_tool("jarsigner", {"JAVA_HOME": str(tmp_path)}) == "jarsigner"
    assert android.java_tool("java", {}) == "java"


def test_apksigner_of_the_newest_build_tools(tmp_path: Path) -> None:
    for version in ("34.0.0", "36.0.0", "9.0.0", "36.1.0-rc1"):
        (tmp_path / "build-tools" / version).mkdir(parents=True)
        (tmp_path / "build-tools" / version / "apksigner").write_text("", encoding="utf-8")
    (tmp_path / "build-tools" / "40.0.0").mkdir()
    found = android.apksigner({"ANDROID_SDK_ROOT": str(tmp_path)})
    assert found == str(tmp_path / "build-tools" / "36.1.0-rc1" / "apksigner")
    with pytest.raises(common.StepError, match="apksigner not found"):
        android.apksigner({"ANDROID_HOME": str(tmp_path / "missing")})


def test_fingerprints_are_read_from_keytool() -> None:
    assert android._fingerprint(f"SHA1: 00\n SHA256: {colons(KEY_SHA)}\n") == KEY_SHA
    with pytest.raises(common.StepError, match="no SHA-256"):
        android._fingerprint("SHA1: 00:11")


# ---- Firebase -------------------------------------------------------------------------------


def services(project: str = "acme-chat", package: str = PACKAGE) -> str:
    return json.dumps(
        {
            "project_info": {"project_id": project},
            "client": [{"client_info": {"android_client_info": {"package_name": package}}}],
        }
    )


def test_the_firebase_configuration_must_be_the_instances(cfg: dict[str, Any]) -> None:
    android.check_google_services(services(), cfg)
    with pytest.raises(common.StepError, match="project other-project"):
        android.check_google_services(services(project="other-project"), cfg)
    with pytest.raises(common.StepError, match=f"no app {PACKAGE}"):
        android.check_google_services(services(package="org.other.app"), cfg)
    with pytest.raises(common.StepError, match=r"no google-services\.json"):
        android.check_google_services("{}", cfg)


def test_the_firebase_configuration_lives_for_the_build_only(tmp_path: Path) -> None:
    target = tmp_path / "gs.json"
    with android.firebase_config("{}", target):
        assert target.read_text(encoding="utf-8") == "{}"
    assert not target.exists()
    with pytest.raises(RuntimeError), android.firebase_config("{}", target):
        raise RuntimeError
    assert not target.exists()
    with android.firebase_config(None, target):
        assert not target.exists()
    target.write_text("left over", encoding="utf-8")
    with pytest.raises(common.StepError, match="exists"), android.firebase_config("{}", target):
        pass
    assert target.read_text(encoding="utf-8") == "left over"


# ---- build and checks -----------------------------------------------------------------------


def test_build_signs_with_the_upload_key_and_the_publishers_identity(
    tmp_path: Path, tree: Path, cfg: dict[str, Any]
) -> None:
    seen: dict[str, Any] = {}
    run = FakeRun(tools(seen=seen))
    work = tmp_path / "work"
    work.mkdir()
    bundle = android.build_bundle(
        run, cfg, appdist.app_version("1.2.3"), common.DEFAULT_BRANDING, key(tmp_path), work
    )
    assert bundle.read_bytes() == b"AAB"
    args, opts = seen["flutter"]
    assert args[1:4] == ["build", "appbundle", "--release"]
    assert "--build-name=1.2.3" in args
    assert "--build-number=1002003" in args
    assert opts["env"] == {
        "ORG_GRADLE_PROJECT_uploadStoreFile": str(tmp_path / "upload.p12"),
        "ORG_GRADLE_PROJECT_uploadStorePassword": "sp",
        "ORG_GRADLE_PROJECT_uploadKeyAlias": "upload",
        "ORG_GRADLE_PROJECT_uploadKeyPassword": "kp",
    }
    branding = yaml.safe_load((work / "branding.yaml").read_text(encoding="utf-8"))
    assert branding["application_id"] == PACKAGE
    assert branding["update_url"] == cfg["android"]["update_url"]
    # The tree gets the release branding for the build and the given branding back.
    assert [c[1:3] for c in run.calls if c[0] == "dart"] == [
        ["run", "tool/branding_defines.dart"]
    ] * 2


def test_a_build_without_a_bundle_fails(tmp_path: Path, tree: Path, cfg: dict[str, Any]) -> None:
    run = FakeRun({"flutter": lambda args, opts: None})
    with pytest.raises(common.StepError, match="wrote no bundle"):
        android.build_bundle(
            run, cfg, appdist.app_version("1.2.3"), common.DEFAULT_BRANDING, key(tmp_path), tmp_path
        )


def test_verify_bundle_checks_signer_structure_identity_and_apks(
    tmp_path: Path, tree: Path, cfg: dict[str, Any]
) -> None:
    seen: dict[str, Any] = {}
    run = FakeRun(tools(seen=seen))
    bundle = tmp_path / "a.aab"
    bundle.write_bytes(b"AAB")
    jar = tmp_path / "bundletool.jar"
    checked = android.verify_bundle(
        run, bundle, key(tmp_path), cfg, appdist.app_version("1.2.3"), tmp_path, jar
    )
    assert checked == android.Checked(KEY_SHA, PACKAGE, 1002003, "1.2.3")
    steps = [c[3] if Path(c[0]).name == "java" else f"{Path(c[0]).name} {c[1]}" for c in run.calls]
    assert steps == [
        "keytool -list",
        "jarsigner -verify",
        "keytool -printcert",
        "validate",
        "dump",
        "dump",
        "dump",
        "build-apks",
        "apksigner verify",
    ]
    # Passwords reach bundletool as files of the run, never on its command line; the files go.
    assert seen["passwords"] == ["sp", "kp"]
    assert not any(a in {"sp", "kp"} for c in run.calls for a in c)
    assert not (tmp_path / "apks-passwords").exists()
    # keytool reads the store password from the environment.
    assert run.calls[0][-2:] == ["-storepass:env", "BOTHY_UPLOAD_STORE_PASSWORD"]


def test_apksigner_of_build_tools_37_names_the_v3_signer(
    tmp_path: Path, tree: Path, cfg: dict[str, Any]
) -> None:
    # Build tools 37 print the signer of a v3-only APK as `V3.0 Signer: certificate ...`
    # (seen on the CI runner), build tools 36 as `Signer #1 certificate ...`.
    handlers = tools()
    handlers["apksigner verify"] = lambda args, opts: ok(
        "V3.0 Signer: certificate DN: CN=Test Publisher\n"
        f"V3.0 Signer: certificate SHA-256 digest: {KEY_SHA}\n"
        "V3.0 Signer: certificate SHA-1 digest: 00\n"
    )
    bundle = tmp_path / "a.aab"
    bundle.write_bytes(b"AAB")
    checked = android.verify_bundle(
        FakeRun(handlers),
        bundle,
        key(tmp_path),
        cfg,
        appdist.app_version("1.2.3"),
        tmp_path,
        tmp_path,
    )
    assert checked.sha256 == KEY_SHA
    # A second signer with another key fails, whatever the first one is.
    handlers["apksigner verify"] = lambda args, opts: ok(
        f"Signer #1 certificate SHA-256 digest: {KEY_SHA}\n"
        f"Signer #2 certificate SHA-256 digest: {OTHER_SHA}\n"
    )
    with pytest.raises(common.StepError, match="universal APK is not signed by the upload key"):
        android.verify_bundle(
            FakeRun(handlers),
            bundle,
            key(tmp_path),
            cfg,
            appdist.app_version("1.2.3"),
            tmp_path,
            tmp_path,
        )


@pytest.mark.parametrize(
    ("handlers", "message"),
    [
        ({"jarsigner": lambda args, opts: ok("jar is unsigned.\n")}, "jarsigner does not verify"),
        (tools(signer=OTHER_SHA), "is signed by cdcd"),
        (tools(code=7), "names org.acme.chat 7 1.2.3"),
        (tools(package="com.example.chat"), "names com.example.chat"),
        (tools(version="1.2.4"), "1.2.4, the release builds"),
        (tools(apk_signer=OTHER_SHA), "universal APK is not signed by the upload key"),
    ],
)
def test_verify_bundle_refuses(
    tmp_path: Path, tree: Path, cfg: dict[str, Any], handlers: dict[str, Any], message: str
) -> None:
    run = FakeRun({**tools(), **handlers})
    bundle = tmp_path / "a.aab"
    bundle.write_bytes(b"AAB")
    with pytest.raises(common.StepError, match=message):
        android.verify_bundle(
            run, bundle, key(tmp_path), cfg, appdist.app_version("1.2.3"), tmp_path, tmp_path / "j"
        )


# ---- release and promotion ------------------------------------------------------------------


@pytest.fixture
def account() -> str:
    from cryptography.hazmat.primitives import serialization
    from cryptography.hazmat.primitives.asymmetric import rsa

    pem = (
        rsa.generate_private_key(public_exponent=65537, key_size=2048)
        .private_bytes(
            serialization.Encoding.PEM,
            serialization.PrivateFormat.PKCS8,
            serialization.NoEncryption(),
        )
        .decode()
    )
    return json.dumps({"client_email": "release@acme.test", "private_key": pem})


def environ(account: str) -> dict[str, str]:
    return {
        "ANDROID_UPLOAD_KEYSTORE": base64.b64encode(b"store").decode(),
        "ANDROID_UPLOAD_KEYSTORE_PASSWORD": "sp",
        "ANDROID_UPLOAD_KEY_ALIAS": "upload",
        "ANDROID_UPLOAD_KEY_PASSWORD": "kp",
        "ANDROID_GOOGLE_SERVICES_JSON": services(),
        "PLAY_SERVICE_ACCOUNT_JSON": account,
    }


def write_cfg(tmp_path: Path, cfg: dict[str, Any]) -> Path:
    path = tmp_path / "distribution.yaml"
    path.write_text(yaml.safe_dump(cfg), encoding="utf-8")
    return path


def test_release_builds_checks_and_puts_the_bundle_on_the_internal_track(
    tmp_path: Path,
    tree: Path,
    cfg: dict[str, Any],
    account: str,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(android, "fetch_bundletool", lambda: tmp_path / "bundletool.jar")
    seen: dict[str, Any] = {}
    run = FakeRun(tools(seen=seen))
    http = FakeHttp(play_routes())
    out = tmp_path / "out"
    target = android.release(
        run,
        distribution=write_cfg(tmp_path, cfg),
        version="1.2.3",
        out=out,
        branding=common.DEFAULT_BRANDING,
        environ=environ(account),
        http=http,
    )
    assert target == out / "org.acme.chat-1.2.3.aab"
    assert target.read_bytes() == b"AAB"
    # The Firebase configuration was there for the build and is gone.
    assert seen["google_services"] is True
    assert not android.GOOGLE_SERVICES.exists()
    uploaded = [c for c in http.calls if "/bundles" in c[1]]
    assert len(uploaded) == 1
    assert uploaded[0][3] == b"AAB"
    track = next(c for c in http.calls if c[0] == "PUT")
    assert track[1] == f"{EDITS}/e1/tracks/{android.BETA_TRACK}"


def test_release_stops_without_secrets_before_anything_runs(tmp_path: Path) -> None:
    run = FakeRun()
    with pytest.raises(
        common.StepError,
        match=r"ANDROID_UPLOAD_KEY_PASSWORD, .*PLAY_SERVICE_ACCOUNT_JSON of the environment "
        r"release-android",
    ):
        android.release(
            run,
            distribution=appdist.TEMPLATE,
            version="1.2.3",
            out=tmp_path,
            branding=common.DEFAULT_BRANDING,
            environ={"ANDROID_UPLOAD_KEYSTORE": "x", "ANDROID_UPLOAD_KEYSTORE_PASSWORD": "x"},
        )
    assert run.calls == []


def test_release_refuses_template_values_and_foreign_firebase(
    tmp_path: Path, cfg: dict[str, Any], account: str
) -> None:
    run = FakeRun()
    with pytest.raises(appdist.DistributionError, match="placeholder values"):
        android.release(
            run,
            distribution=appdist.TEMPLATE,
            version="1.2.3",
            out=tmp_path,
            branding=common.DEFAULT_BRANDING,
            environ=environ(account),
        )
    env = environ(account)
    env["ANDROID_GOOGLE_SERVICES_JSON"] = services(project="other-project")
    with pytest.raises(common.StepError, match="other-project"):
        android.release(
            run,
            distribution=write_cfg(tmp_path, cfg),
            version="1.2.3",
            out=tmp_path,
            branding=common.DEFAULT_BRANDING,
            environ=env,
        )
    assert run.calls == []


def test_a_keystore_that_is_no_base64_is_refused(tmp_path: Path) -> None:
    with pytest.raises(common.StepError, match="no base64"):
        android._keystore(tmp_path, "not base64!")


def test_promote_releases_the_same_build_on_the_stable_track(
    tmp_path: Path, cfg: dict[str, Any], account: str
) -> None:
    http = FakeHttp(play_routes())
    track = android.promote(
        distribution=write_cfg(tmp_path, cfg),
        version_code=1002003,
        environ={"PLAY_SERVICE_ACCOUNT_JSON": account},
        http=http,
    )
    assert track == "stable"
    put = next(c for c in http.calls if c[0] == "PUT")
    assert put[1] == f"{EDITS}/e1/tracks/stable"
    assert json.loads(put[3])["releases"][0] == {  # type: ignore[arg-type]
        "name": "1.2.3",
        "versionCodes": ["1002003"],
        "status": "completed",
    }
    with pytest.raises(common.StepError, match="PLAY_SERVICE_ACCOUNT_JSON"):
        android.promote(distribution=write_cfg(tmp_path, cfg), version_code=1, environ={})


# ---- trial ----------------------------------------------------------------------------------


def test_trial_builds_with_a_throwaway_key_and_probes_a_foreign_one(
    tmp_path: Path, tree: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(android, "fetch_bundletool", lambda: tmp_path / "bundletool.jar")
    seen: dict[str, Any] = {}
    handlers = tools(
        version=android.TRIAL_VERSION, code=1000, package="com.example.chat", seen=seen
    )
    listed = iter([KEY_SHA, OTHER_SHA])
    base_keytool = handlers["keytool"]

    def keytool(args: list[str], opts: dict[str, Any]) -> Any:
        # The first key listed is the trial's upload key, the second the probe's own.
        if "-list" in args:
            return ok(f"SHA256: {colons(next(listed))}\n")
        return base_keytool(args, opts)

    handlers["keytool"] = keytool
    run = FakeRun(handlers)
    checked = android.trial(run, out=tmp_path / "out", branding=common.DEFAULT_BRANDING)
    assert checked.version_code == 1000
    assert len(seen["genkeypair"]) == 2
    first = seen["genkeypair"][0]
    assert first[first.index("-dname") + 1] == "CN=Test Publisher"
    assert "-storepass:env" in first
    report = json.loads((tmp_path / "out" / "trial-android.json").read_text(encoding="utf-8"))
    assert report["signer_sha256"] == KEY_SHA
    assert "another upload key is refused by the signer check" in report["checks"]
    # No Firebase configuration and no upload in a trial.
    assert seen["google_services"] is False


def test_trial_fails_when_the_probe_passes(
    tmp_path: Path, tree: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(android, "fetch_bundletool", lambda: tmp_path / "bundletool.jar")
    run = FakeRun(tools(version=android.TRIAL_VERSION, code=1000, package="com.example.chat"))
    with pytest.raises(common.StepError, match="accepted a key that did not sign"):
        android.trial(run, out=tmp_path / "out", branding=common.DEFAULT_BRANDING)


def test_main_reports_failures_with_exit_code_1(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    code = android.main(["release", "--version", "1.2.3", "--out", str(tmp_path)], run=FakeRun())
    assert code == 1
    assert "release-android" in capsys.readouterr().err
    assert android.main(["promote", "--version-code", "1"], run=FakeRun()) == 1
    with pytest.raises(SystemExit) as exit_info:
        android.main(["nonsense"])
    assert exit_info.value.code == 2


def test_main_runs_the_trial(
    tmp_path: Path, tree: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setattr(
        android,
        "trial",
        lambda run, out, branding: android.Checked(KEY_SHA, "com.example.chat", 1000, "0.1.0"),
    )
    assert android.main(["trial", "--out", str(tmp_path)], run=FakeRun()) == 0
    assert "trial-android: com.example.chat 0.1.0 (1000)" in capsys.readouterr().out

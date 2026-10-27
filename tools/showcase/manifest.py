# SPDX-License-Identifier: Apache-2.0
"""The showcase manifest and its check (M7-28, requirements 13 and 14).

`docs/assets/showcase/manifest.json` names, for every image and video of the showcase, the
script that captured it, the commit it was captured from and the seed of the demo instance
(M7-27), so every file can be produced again and nothing in the folder is made by hand:

    {"version": 1, "assets": [{"file": "android/threads-light.png", "kind": "image",
      "platform": "android", "view": "threads", "theme": "light",
      "script": "tools/showcase/<capture script>", "commit": "<40 hex>", "demo_seed": 7}]}

`file` is relative to `docs/assets/showcase`. The file is written sorted (entries by `file`,
keys by name) with two-space indentation and a trailing newline, so a fresh capture of the same
images changes only the values that changed.

`check` holds the folder and the manifest equal and every file clean: each file is listed and
each listed file exists, the name is `<platform>/<[a-z0-9-]+>.png` or `.mp4` in the folder of
its platform, the content is PNG for an image and MP4 for a video and carries no metadata
(media.py), and the capture script exists. Besides the files, the folder may hold only
`manifest.json` and a `README.md` at its top.
"""

from __future__ import annotations

import json
import re
from dataclasses import asdict, dataclass, fields
from pathlib import Path, PurePosixPath

import media

ASSET_DIR = Path("docs/assets/showcase")
MANIFEST = ASSET_DIR / "manifest.json"
VERSION = 1
# Files of the folder that are not showcase media.
NOT_MEDIA = frozenset({"manifest.json", "README.md"})

KINDS = {"image": "png", "video": "mp4"}
THEMES = ("light", "dark")
TOKEN = re.compile(r"[a-z0-9-]+")
FILE_NAME = re.compile(r"(?P<platform>[a-z0-9-]+)/[a-z0-9-]+\.(?:png|mp4)")
COMMIT = re.compile(r"[0-9a-f]{40}")
FORMAT_NAMES = {"png": "PNG", "mp4": "MP4"}


class ManifestError(ValueError):
    """A manifest or a manifest entry that breaks the schema.

    Attributes:
        problems: one message per problem, each naming the key (`assets[2].theme: ...`).
    """

    def __init__(self, problems: list[str]) -> None:
        """Keep every problem; the message joins them."""
        super().__init__("; ".join(problems))
        self.problems = problems


@dataclass(frozen=True)
class Asset:
    """One image or video of the showcase.

    Attributes:
        file: path relative to `docs/assets/showcase`, `<platform>/<name>.<png|mp4>`.
        kind: `image` (PNG) or `video` (MP4).
        platform: the surface it shows (`android`, `ios`, `macos`, `console`, `grafana`, ...).
        view: short name of the view (`threads`, `approval`, `flow`).
        theme: `light` or `dark`.
        script: repository path of the capture script.
        commit: the 40-hex commit the capture ran on.
        demo_seed: the seed of the demo instance the capture showed.
    """

    file: str
    kind: str
    platform: str
    view: str
    theme: str
    script: str
    commit: str
    demo_seed: int


KEYS = tuple(f.name for f in fields(Asset))


def _relative(value: str) -> bool:
    """True for a relative POSIX path without `..`, backslash or empty parts."""
    parts = value.split("/")
    return (
        bool(value)
        and "\\" not in value
        and not PurePosixPath(value).is_absolute()
        and all(p not in ("", ".", "..") for p in parts)
    )


def _entry_problems(where: str, entry: dict[str, object]) -> list[str]:
    """Schema problems of one entry; `where` prefixes every message (`assets[0]`)."""
    problems = [f"{where}: missing key {k!r}" for k in KEYS if k not in entry]
    problems += [f"{where}: unknown key {k!r}" for k in entry if k not in KEYS]

    def value(key: str) -> object:
        return entry.get(key)

    checks: list[tuple[str, bool, str]] = [
        (
            "file",
            isinstance(value("file"), str) and _relative(str(value("file"))),
            "expected a relative path",
        ),
        ("kind", value("kind") in KINDS, "expected 'image' or 'video'"),
        (
            "platform",
            isinstance(value("platform"), str)
            and TOKEN.fullmatch(str(value("platform"))) is not None,
            "expected [a-z0-9-]+",
        ),
        (
            "view",
            isinstance(value("view"), str) and bool(value("view")),
            "expected a non-empty string",
        ),
        ("theme", value("theme") in THEMES, "expected 'light' or 'dark'"),
        (
            "script",
            isinstance(value("script"), str) and _relative(str(value("script"))),
            "expected a relative repository path",
        ),
        (
            "commit",
            isinstance(value("commit"), str) and COMMIT.fullmatch(str(value("commit"))) is not None,
            "expected 40 lower-case hex digits",
        ),
        # bool is an int in Python; a seed of `true` is a mistake, not the seed 1.
        (
            "demo_seed",
            isinstance(value("demo_seed"), int) and not isinstance(value("demo_seed"), bool),
            "expected an integer",
        ),
    ]
    problems += [
        f"{where}.{key}: {message}, got {value(key)!r}"
        for key, ok, message in checks
        if key in entry and not ok
    ]
    return problems


def parse(data: object) -> list[Asset]:
    """Validate a decoded manifest and return its entries.

    Args:
        data: the JSON value of the manifest.

    Returns:
        The entries in file order.

    Raises:
        ManifestError: every schema problem, each naming its key.
    """
    if not isinstance(data, dict):
        raise ManifestError(["manifest: expected an object"])
    problems = [f"manifest: missing key {k!r}" for k in ("version", "assets") if k not in data]
    problems += [f"manifest: unknown key {k!r}" for k in data if k not in ("version", "assets")]
    if "version" in data and data["version"] != VERSION:
        problems.append(f"version: expected {VERSION}, got {data['version']!r}")
    raw = data.get("assets", [])
    if not isinstance(raw, list):
        problems.append("assets: expected a list")
        raw = []
    assets: list[Asset] = []
    seen: set[object] = set()
    for i, entry in enumerate(raw):
        where = f"assets[{i}]"
        if not isinstance(entry, dict):
            problems.append(f"{where}: expected an object")
            continue
        found = _entry_problems(where, entry)
        if entry.get("file") in seen:
            found.append(f"{where}.file: {entry['file']!r} is listed twice")
        seen.add(entry.get("file"))
        problems += found
        if not found:
            assets.append(Asset(**entry))
    if problems:
        raise ManifestError(problems)
    return assets


def load(path: Path) -> list[Asset]:
    """Read and validate a manifest file; a missing file is an empty manifest.

    Args:
        path: the manifest file.

    Returns:
        The entries in file order.

    Raises:
        ManifestError: invalid JSON or a schema problem.
    """
    if not path.exists():
        return []
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        raise ManifestError([f"manifest: not valid JSON: {exc}"]) from exc
    return parse(data)


def dump(assets: list[Asset]) -> str:
    """Serialize entries as the stable manifest text.

    Args:
        assets: the entries in any order.

    Returns:
        Sorted JSON (entries by `file`, keys by name), two-space indentation, trailing newline.
    """
    entries = [asdict(a) for a in sorted(assets, key=lambda a: a.file)]
    return json.dumps({"version": VERSION, "assets": entries}, indent=2, sort_keys=True) + "\n"


def _save(root: Path, assets: list[Asset]) -> None:
    path = root / MANIFEST
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(dump(assets), encoding="utf-8")


def record(root: Path, asset: Asset) -> None:
    """Add an entry to the manifest or replace the entry of the same file.

    Args:
        root: the repository root.
        asset: the entry.

    Raises:
        ManifestError: the entry or the existing manifest breaks the schema; nothing is written.
    """
    parse({"version": VERSION, "assets": [asdict(asset)]})
    assets = [a for a in load(root / MANIFEST) if a.file != asset.file]
    _save(root, [*assets, asset])


def drop_platform(root: Path, platform: str) -> list[str]:
    """Remove a platform's entries and every file of its folder before a fresh capture.

    Args:
        root: the repository root.
        platform: the platform, `[a-z0-9-]+`.

    Returns:
        The removed files, relative to `docs/assets/showcase`, sorted.

    Raises:
        ManifestError: the platform name or the existing manifest breaks the schema.
    """
    if TOKEN.fullmatch(platform) is None:
        raise ManifestError([f"platform: expected [a-z0-9-]+, got {platform!r}"])
    assets = load(root / MANIFEST)
    kept = [a for a in assets if a.platform != platform]
    folder = root / ASSET_DIR / platform
    files = sorted(p for p in folder.rglob("*") if p.is_file()) if folder.is_dir() else []
    for p in files:
        p.unlink()
    if folder.is_dir():
        # Deepest first, so a folder is empty when its turn comes.
        for d in sorted((p for p in folder.rglob("*") if p.is_dir()), reverse=True):
            d.rmdir()
        folder.rmdir()
    if kept != assets:
        _save(root, kept)
    return [p.relative_to(root / ASSET_DIR).as_posix() for p in files]


def _media_findings(path: Path, extension: str, kind: str | None) -> list[str]:
    """Format, kind and metadata findings of one file on disk."""
    data = path.read_bytes()
    content = media.detect(data)
    if content is None:
        return ["neither PNG nor MP4"]
    found: list[str] = []
    if extension in FORMAT_NAMES and extension != content:
        found.append(f"content is {FORMAT_NAMES[content]}, extension is .{extension}")
    elif kind is not None and extension in FORMAT_NAMES and KINDS[kind] != extension:
        found.append(f"kind {kind!r} does not match .{extension}")
    try:
        found += [f"metadata: {m}" for m in media.metadata(data)]
    except media.MediaError as exc:
        found.append(str(exc))
    return found


def check(root: Path) -> list[str]:
    """Check the showcase folder against its manifest and the metadata rule.

    Args:
        root: the repository root.

    Returns:
        Sorted findings, one line each, `showcase: <file>: <problem>`; empty when clean.
        A manifest that breaks the schema gives only its schema findings.
    """
    folder = root / ASSET_DIR
    try:
        assets = {a.file: a for a in load(root / MANIFEST)}
    except ManifestError as exc:
        return [f"showcase: manifest.json: {p}" for p in exc.problems]
    on_disk = set()
    if folder.is_dir():
        on_disk = {
            p.relative_to(folder).as_posix()
            for p in folder.rglob("*")
            if p.is_file() and p.relative_to(folder).as_posix() not in NOT_MEDIA
        }
    findings: list[str] = []
    for file in sorted(on_disk | set(assets)):
        entry = assets.get(file)
        found: list[str] = []
        name = FILE_NAME.fullmatch(file)
        if name is None:
            found.append("file name must be <platform>/<[a-z0-9-]+>.png or .mp4")
        elif entry is not None and name["platform"] != entry.platform:
            found.append(f"folder {name['platform']!r} is not the platform {entry.platform!r}")
        if entry is None:
            found.append("not in the manifest")
        elif not (root / entry.script).is_file():
            found.append(f"capture script {entry.script} is missing")
        if file not in on_disk:
            found.append("named in the manifest but missing")
        else:
            extension = file.rsplit(".", 1)[-1] if "." in file.rsplit("/", 1)[-1] else ""
            found += _media_findings(folder / file, extension, entry.kind if entry else None)
        findings += [f"showcase: {file}: {f}" for f in found]
    return sorted(findings)

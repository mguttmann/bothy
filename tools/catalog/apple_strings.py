# SPDX-License-Identifier: Apache-2.0
"""Generate the native strings of push and the share sheet (iOS, macOS) and of push (Android).

The push of docs/spec/13-push-notifications.md 6.1 carries only the keys `PUSH_TITLE` and
`PUSH_BODY_<KIND>`; the system resolves them in the app's `Localizable.strings`, and the
Notification Service Extension needs "Message not available" and the two preview formats in
its own bundle (brief M5-04 Design note 6). The native sheet of the Share Extension shows its
texts (`SHARE_*`) without the Flutter engine, so it reads them from the same files (brief M5-06
Design note 1 and rule 8). The texts have one source, the ARB files of `packages/l10n`: this
tool writes `<lang>.lproj/Localizable.strings` for every ARB locale into OUTPUT_DIR, which the
iOS and macOS runners and their extensions include.

ARB placeholders such as `{name}` become positional `%1$@`, in the order of the template's
`placeholders` metadata; a literal `%` becomes `%%`, because the system formats these strings.
Every argument is a string (`%@`): the Swift side formats numbers before it passes them.

A `.strings` file cannot hold a plural. A key of PLURAL_KEYS is an ICU plural
(`{count, plural, =1{...} other{...}}`) and becomes two lines: `<KEY>_ONE` from the `=1`
branch (or `one` when there is no `=1`) and `<KEY>_OTHER` from the `other` branch, each
converted like the other texts. The Swift side picks `_ONE` for a count of 1 and `_OTHER`
otherwise, which is right for the app's languages (en, de). Known limit: a language with more
plural forms (few, many) needs a `.stringsdict` instead; this tool does not write one.

The Android push service (brief M5-05 Design note 5) posts its placeholders itself, because an FCM
data message carries no text: the push keys (`PUSH_*`) in lower case go into
`apps/client/android/app/src/main/res/values[-<lang>]/strings.xml` (the template's language in
`values/`), with placeholders as positional `%1$s`. There `%` becomes `%%` only in a string with
placeholders, because only those are formatted, and the text is escaped for XML and for aapt
(backslash, `"`, `'`, a leading `@` or `?`, new lines).

Usage:
    apple_strings.py [--root <repo>]            write the files
    apple_strings.py [--root <repo>] --check    fail (exit 1) when a file is stale

Exit codes: 0 ok; 1 a key or placeholder is missing, a plural key holds no valid plural, or
(with `--check`) a file is stale; 2 usage error.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

ARB_DIR = Path("packages/l10n/lib/l10n")
TEMPLATE = "app_en.arb"
OUTPUT_DIR = Path("apps/client/ios/Localizations")
ANDROID_DIR = Path("apps/client/android/app/src/main/res")

# Native key to ARB key. The kinds of 13 6.1; PUSH_BODY_MESSAGE, _SECURITY and _ADMIN reuse the
# texts of the desktop notifications, so both platforms say the same.
NATIVE_KEYS: dict[str, str] = {
    "PUSH_TITLE": "pushTitle",
    "PUSH_BODY_MESSAGE": "notificationPlaceholder",
    "PUSH_BODY_MENTION": "pushBodyMention",
    "PUSH_BODY_DM": "pushBodyDm",
    "PUSH_BODY_RUN_DONE": "pushBodyRunDone",
    "PUSH_BODY_RUN_FAILED": "pushBodyRunFailed",
    "PUSH_BODY_INPUT_NEEDED": "pushBodyInputNeeded",
    "PUSH_BODY_SECURITY": "notificationSecurityText",
    "PUSH_BODY_ADMIN": "notificationAdminText",
    "PUSH_MESSAGE_NOT_AVAILABLE": "pushMessageNotAvailable",
    "PUSH_PREVIEW_TITLE": "notificationTitle",
    "PUSH_PREVIEW_TEXT": "notificationText",
    # The native sheet of the Share Extension (brief M5-06 Design note 1); the reasons fill the
    # placeholder of SHARE_REFUSED.
    "SHARE_SHEET_TITLE": "shareSheetTitle",
    "SHARE_CANCEL": "shareCancel",
    "SHARE_SEND": "shareSend",
    "SHARE_COMMENT": "shareComment",
    "SHARE_POST_TITLE": "sharePostTitle",
    "SHARE_TARGETS": "shareTargets",
    "SHARE_SEARCH": "shareSearch",
    "SHARE_WRITE_ONLY": "shareWriteOnly",
    "SHARE_NEW_POST": "shareNewPost",
    "SHARE_FILE_TOO_LARGE": "shareFileTooLarge",
    "SHARE_TOO_MANY_FILES": "shareTooManyFiles",
    "SHARE_NO_TARGET": "shareNoTarget",
    "SHARE_QUEUED": "shareQueued",
    "SHARE_REFUSED": "shareRefused",
    "SHARE_UNAVAILABLE": "shareUnavailable",
    "SHARE_LOAD_FAILED": "shareLoadFailed",
    "SHARE_REASON_FORBIDDEN": "shareReasonForbidden",
    "SHARE_REASON_TOO_LARGE": "shareReasonTooLarge",
    "SHARE_REASON_TYPE": "shareReasonType",
    "SHARE_REASON_GONE": "shareReasonGone",
    "SHARE_REASON_SAVE": "shareReasonSave",
    "SHARE_REASON_STATUS": "shareReasonStatus",
}

# The keys of the Android push service's strings.xml: only the push texts, because the Android
# share sheet is Flutter and reads the ARB files itself.
ANDROID_KEYS: dict[str, str] = {k: v for k, v in NATIVE_KEYS.items() if k.startswith("PUSH_")}

# Native key to ARB key of an ICU plural; each becomes the lines `<KEY>_ONE` and `<KEY>_OTHER`.
PLURAL_KEYS: dict[str, str] = {
    "SHARE_MORE_FILES": "shareMoreFiles",
}

_PLACEHOLDER = re.compile(r"\{([A-Za-z_][A-Za-z0-9_]*)\}")


class StringsError(Exception):
    """An ARB file lacks a key, uses an undeclared placeholder or holds no valid plural."""


def _native(text: str, order: list[str]) -> str:
    def replace(match: re.Match[str]) -> str:
        name = match.group(1)
        if name not in order:
            raise StringsError(f"placeholder {{{name}}} is not declared in the template")
        return f"%{order.index(name) + 1}$@"

    escaped = text.replace("%", "%%")
    converted = _PLACEHOLDER.sub(replace, escaped)
    quoted = converted.replace("\\", "\\\\").replace('"', '\\"').replace("\n", "\\n")
    return quoted


def _block(text: str, start: int) -> int:
    """The index just after the `}` that closes the `{` at `start`, counting nested braces."""
    depth = 0
    for index in range(start, len(text)):
        if text[index] == "{":
            depth += 1
        elif text[index] == "}":
            depth -= 1
            if depth == 0:
                return index + 1
    raise StringsError(f"unbalanced braces in {text!r}")


_PLURAL_HEAD = re.compile(r"\{\s*[A-Za-z_][A-Za-z0-9_]*\s*,\s*plural\s*,")
_SELECTOR = re.compile(r"\s*(=\d+|zero|one|two|few|many|other)\s*(?=\{)")


def plural_branches(text: str) -> tuple[str, str]:
    """The texts of the singular and the `other` branch of an ICU plural.

    Args:
        text: the whole ARB text, `{count, plural, =1{...} other{...}}`; branch texts may
            contain nested `{placeholders}`.

    Returns:
        The `=1` branch (or `one` when there is no `=1`) and the `other` branch.

    Raises:
        StringsError: the text is not exactly one plural, its braces do not balance, a
            selector repeats, or the singular or the `other` branch is missing.
    """
    head = _PLURAL_HEAD.match(text)
    if head is None or _block(text, 0) != len(text):
        raise StringsError(f"{text!r} is not an ICU plural")
    branches: dict[str, str] = {}
    position = head.end()
    end = len(text) - 1  # the closing brace of the plural
    while text[position:end].strip():
        selector = _SELECTOR.match(text, position, end)
        if selector is None:
            raise StringsError(f"unexpected text in the plural {text!r}")
        name = selector.group(1)
        if name in branches:
            raise StringsError(f"selector {name} repeats in the plural {text!r}")
        close = _block(text, selector.end())
        if close > end:
            raise StringsError(f"unbalanced braces in {text!r}")
        branches[name] = text[selector.end() + 1 : close - 1]
        position = close
    one = branches.get("=1", branches.get("one"))
    if one is None or "other" not in branches:
        raise StringsError(f"the plural {text!r} needs a =1 or one branch and an other branch")
    return one, branches["other"]


def _order(template: dict[str, object], key: str) -> list[str]:
    meta = template.get(f"@{key}")
    placeholders = meta.get("placeholders", {}) if isinstance(meta, dict) else {}
    return list(placeholders) if isinstance(placeholders, dict) else []


def render(arb: dict[str, object], template: dict[str, object]) -> str:
    """Render one `Localizable.strings`.

    Args:
        arb: the ARB file of the language.
        template: the template ARB file with the placeholder metadata.

    Returns:
        The file content, UTF-8 text with one line per native key.

    Raises:
        StringsError: a key is missing, a placeholder is not declared or a plural key holds
            no valid plural.
    """
    lines = [
        "/* SPDX-License-Identifier: Apache-2.0 */",
        "/* GENERATED by tools/catalog/apple_strings.py from packages/l10n/lib/l10n; do not edit.",
        "   Run `make apple-strings`; make codegen-check holds these files. */",
        "",
    ]
    for native, key in (NATIVE_KEYS | PLURAL_KEYS).items():
        text = arb.get(key)
        if not isinstance(text, str):
            raise StringsError(f"{arb.get('@@locale')}: {key} is missing")
        order = _order(template, key)
        if native in PLURAL_KEYS:
            try:
                one, other = plural_branches(text)
            except StringsError as exc:
                raise StringsError(f"{arb.get('@@locale')}: {key}: {exc}") from exc
            lines.append(f'"{native}_ONE" = "{_native(one, order)}";')
            lines.append(f'"{native}_OTHER" = "{_native(other, order)}";')
        else:
            lines.append(f'"{native}" = "{_native(text, order)}";')
    lines.append("")
    return "\n".join(lines)


def _android(text: str, order: list[str]) -> str:
    def replace(match: re.Match[str]) -> str:
        name = match.group(1)
        if name not in order:
            raise StringsError(f"placeholder {{{name}}} is not declared in the template")
        return f"%{order.index(name) + 1}$s"

    escaped = text.replace("\\", "\\\\")
    if _PLACEHOLDER.search(escaped):
        escaped = escaped.replace("%", "%%")
    escaped = escaped.replace('"', '\\"').replace("'", "\\'").replace("\n", "\\n")
    escaped = escaped.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
    if escaped[:1] in ("@", "?"):
        escaped = "\\" + escaped
    return _PLACEHOLDER.sub(replace, escaped)


def render_android(arb: dict[str, object], template: dict[str, object]) -> str:
    """Render one Android `strings.xml` of the push texts.

    Args:
        arb: the ARB file of the language.
        template: the template ARB file with the placeholder metadata.

    Returns:
        The file content: one `<string>` per key of ANDROID_KEYS, named in lower case.

    Raises:
        StringsError: a key is missing or a placeholder is not declared.
    """
    lines = [
        '<?xml version="1.0" encoding="utf-8"?>',
        "<!-- SPDX-License-Identifier: Apache-2.0 -->",
        "<!-- GENERATED by tools/catalog/apple_strings.py from packages/l10n/lib/l10n;",
        "     do not edit. Run `make apple-strings`; make codegen-check holds these files. -->",
        "<resources>",
    ]
    for native, key in ANDROID_KEYS.items():
        text = arb.get(key)
        if not isinstance(text, str):
            raise StringsError(f"{arb.get('@@locale')}: {key} is missing")
        meta = template.get(f"@{key}")
        placeholders = meta.get("placeholders", {}) if isinstance(meta, dict) else {}
        order = list(placeholders) if isinstance(placeholders, dict) else []
        lines.append(f'    <string name="{native.lower()}">{_android(text, order)}</string>')
    lines.append("</resources>")
    lines.append("")
    return "\n".join(lines)


def outputs(root: Path) -> dict[Path, str]:
    """The generated files by path relative to `root`.

    Args:
        root: the repository root.

    Returns:
        One `Localizable.strings` and one Android `strings.xml` per ARB locale.

    Raises:
        StringsError: an ARB file lacks a key or a placeholder.
        OSError: an ARB file cannot be read.
        ValueError: an ARB file is not JSON.
    """
    arb_dir = root / ARB_DIR
    template = json.loads((arb_dir / TEMPLATE).read_text(encoding="utf-8"))
    files: dict[Path, str] = {}
    for path in sorted(arb_dir.glob("app_*.arb")):
        arb = json.loads(path.read_text(encoding="utf-8"))
        locale = str(arb.get("@@locale") or path.stem.removeprefix("app_"))
        files[OUTPUT_DIR / f"{locale}.lproj" / "Localizable.strings"] = render(arb, template)
        values = "values" if path.name == TEMPLATE else f"values-{locale}"
        files[ANDROID_DIR / values / "strings.xml"] = render_android(arb, template)
    return files


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module docstring for usage and exit codes."""
    parser = argparse.ArgumentParser(prog="apple_strings.py")
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument("--check", action="store_true", help="fail when a file is stale")
    try:
        args = parser.parse_args(argv)
    except SystemExit as exc:  # argparse ends on usage errors (2) and on --help (0)
        return exc.code if isinstance(exc.code, int) else 2
    root: Path = args.root
    try:
        files = outputs(root)
    except (StringsError, OSError, ValueError) as exc:
        print(f"apple-strings: {exc}", file=sys.stderr)
        return 1
    if args.check:
        stale = [
            str(path)
            for path, text in files.items()
            if not (root / path).is_file() or (root / path).read_text(encoding="utf-8") != text
        ]
        if stale:
            print(
                f"apple-strings: stale {', '.join(stale)}; run `make apple-strings` and commit",
                file=sys.stderr,
            )
            return 1
        return 0
    for path, text in files.items():
        (root / path).parent.mkdir(parents=True, exist_ok=True)
        (root / path).write_text(text, encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())

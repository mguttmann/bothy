# SPDX-License-Identifier: Apache-2.0
"""The working name stays at the places of its kinds (docs/spec/02-architektur.md "6. Code-Layout",
ADR 0044 Confirmation, PLAN M1-46).

The test searches every tracked file of code and configuration (not docs/, not Markdown, not
binary assets) for the working name, case-insensitively. Each hit is the whole token around it
(letters, digits and `_./:%@-`); a token is allowed only when it matches one of the kinds below,
each of which names its line in 02 6. A bare word is allowed only as the binary in front of one of
its subcommands. Every other hit fails with file, line and token, so a new place with the name
either fits a kind or enters 02 6 with a reason first.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
NAME = "bothy"

# Files that are no code or configuration: the documentation (it names the product freely),
# binary assets, lock files, code generated from docs/ (it mirrors the documentation) and this
# test, whose fixtures name the forms it checks.
SKIP = re.compile(
    r"^docs/|^tools/ci/tests/test_product_name_places\.py$|\.(md|png|jpe?g|svg|ico|woff2?|ttf|otf|pdf|lock|sum)$|(^|/)package-lock\.json$|\.gen\.(go|ya?ml)$|/generated/",
)

# Characters of a token around a hit.
TOKEN_CHARS = frozenset("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_./:%@-")

# Characters that may stand right before the name inside a token: a path, a prefix list or a
# separator of an identifier (sloth-slo-alerts-bothy-api, .bothy-tmp-x, test_bothy_noise).
BEFORE = frozenset("/-_.:@")

# The kinds of 02 "6. Code-Layout". Each form keeps the name as a whole segment, so a change of
# the name stays a mechanical replace:
#   - module paths and the repository: github.com/OWNER/bothy, OWNER.github.io/bothy;
#   - a path segment: cmd/bothy, /run/bothy, /var/lib/bothy, the image namespace bothy/...;
#   - a prefix: bothy-<x> (images, host service, protocol ids, SDK, plugin, SLO, dashboard and
#     tool ids), bothy_<x> (database roles, Dart packages, metrics, token prefixes), bothy.<x>
#     (telemetry attributes, configuration file), BOTHY_ (environment), Bothy<Name> (alerts);
#   - the binary in front of a subcommand (bothy migrate up);
#   - the display name in the tool configurations listed in DISPLAY (02 6 "Anzeigenamen").
# Two values are fixed bytes and never change with the name: the advisory lock keys "bothyjob"
# and "bothyboo" (a new value would let two server versions migrate at the same time).
FIXED = frozenset({"bothyjob", "bothyboo"})

# Files whose display texts carry the product name (02 6 "Anzeigenamen der Werkzeuge"): the
# copyright line, the documentation site, SonarQube project and gate, the gitleaks title, the
# Grafana dashboards and folder, the design preview, and the default of branding.product_name.
DISPLAY = re.compile(
    r"^(NOTICE|mkdocs\.yml|sonar-project\.properties|\.gitleaks\.toml|quality/sonar/.*|tools/sonarqube/.*"
    r"|services/(server|host)/\.golangci\.yml|infra/observability/(grafana|alertmanager)/.*"
    r"|design/build/preview\.html|tools/design-tokens/tokenkit/emit_preview\.py"
    r"|services/server/internal/instance/domain/settings(_test)?\.go)$",
)

# Files of the binary itself, whose identifiers (not texts) may name it (runBothy, bothyBinary).
BINARY_FILES = re.compile(r"^services/server/cmd/bothy/")
IDENTIFIER = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")

# Client code: user-facing texts live here, so a bare value is never allowed (ADR 0044).
CLIENT_FILES = re.compile(r"^(apps|packages)/")

# A bare word is the binary when one of its subcommands follows (bothy migrate up, `bothy serve`).
BINARY_USE = re.compile(
    r"bothy[`\"']?\s+[`\"']?(serve|migrate|secrets|config|telemetry|healthcheck|setup|import|restore|version|<command>)\b",
)

# A bare word as a whole value: a quoted literal ("bothy", 'bothy', `bothy`) or a configuration
# value (name: bothy, NAME=bothy, ${X:-bothy}). These are the ids of 02 6 written alone: the
# database name and service name of the default configuration, the component and container
# names. Prose in comments never matches, so it names the product without the name.
VALUE = re.compile(r"""(["'`])bothy\1|\[bothy\]|(:\s*|=\s*|:-)bothy(\s|$|["'}\],\\])|'bothy\\_""")

# The comment marker at the start of a line.
COMMENT = re.compile(r"^\s*(//+|#+|\*|--)\s*")

# The alert rules define the CamelCase alert names (alert: BothyVolumeFull); only these names,
# and their lower-case forms (runbook anchors, label values), may use the prefix Bothy.
ALERT_FILES = re.compile(r"^infra/observability/prometheus/rules/[\w.-]+\.ya?ml$")
ALERT = re.compile(r"^\s*-?\s*alert:\s*(Bothy[A-Z]\w*)", re.MULTILINE)

# The alert tooling and its tests use example alert names of the same form (BothyExample).
ALERT_TOOLS = re.compile(r"^(tools/opscheck|infra/observability)/")


def tokens(line: str) -> list[tuple[int, str]]:
    """Returns the start in line and the token of every hit of the name in line."""
    low = line.lower()
    out: list[tuple[int, str]] = []
    pos = 0
    while (i := low.find(NAME, pos)) >= 0:
        a, b = i, i + len(NAME)
        while a > 0 and line[a - 1] in TOKEN_CHARS:
            a -= 1
        # An escape such as \n before the token belongs to the string, not to the token.
        if a > 0 and line[a - 1] == "\\" and a < i:
            a += 1
        while b < len(line) and line[b] in TOKEN_CHARS:
            b += 1
        out.append((a, line[a:b]))
        pos = b
    return out


def _form_ok(token: str, at: int, alerts: frozenset[str]) -> bool:
    """Reports whether the hit at index at of token is one of the forms of the kinds."""
    form, before, after = token[at : at + len(NAME)], token[at - 1 : at], token[at + len(NAME) :]
    if form == "BOTHY" and after.startswith("_"):
        return True
    if before and before not in BEFORE:
        return False
    if (
        after[:1] in ("-", "_", ".")
        and (after[1:2].isalnum() or after[1:] == "")
        and form in ("bothy", "BOTHY")
    ):
        return True
    word = re.match(r"\w*", token[at:])
    if form == "Bothy" and after[:1].isupper():
        return word is not None and word.group(0).lower() in alerts
    if form == "bothy" and (before == "/" or after[:1] in ("/", ":")):
        return True
    if token[at - 2 : at] == ":-" and after == "":
        return True
    plain = token[at:].split("/", maxsplit=1)[0].strip("./:-")
    return (form == "bothy" and plain in alerts) or plain in FIXED


def allowed(
    rel: str,
    line: str,
    start: int,
    token: str,
    alerts: frozenset[str] = frozenset(),
    following: str = "",
) -> bool:
    """Reports whether every hit of the name in token (at start in line of file rel) is a place
    of a kind of 02 6; following is the next line without its comment marker, for a binary name
    at the end of a wrapped comment line."""
    low = token.lower()
    at = low.find(NAME)
    while at >= 0:
        bare = token[at : at + len(NAME) + 1].rstrip("./:-") == token[at : at + len(NAME)] and (
            at == 0 or token[at - 1] not in TOKEN_CHARS
        )
        ok = _form_ok(token, at, alerts)
        if not ok and token.strip("./:-`'\"") == token[at : at + len(NAME)]:
            joined = line + " " + following
            ok = bool(BINARY_USE.match(joined, start + at)) or (
                DISPLAY.search(rel) is not None and token[at] == "B"
            )
            ok = ok or (
                token[at] == "b"
                and not CLIENT_FILES.search(rel)
                and VALUE.search(line[max(0, start + at - 3) : start + at + len(NAME) + 2])
                is not None
            )
        if not ok and BINARY_FILES.search(rel):
            # The binary's own files name it in identifiers and as the lower-case command word,
            # never in a capitalized text.
            ok = (IDENTIFIER.fullmatch(token) is not None and not bare) or (
                token[at : at + len(NAME)] == NAME
            )
        if not ok and ALERT_TOOLS.search(rel) and token[at : at + len(NAME)] == "Bothy":
            ok = re.fullmatch(r"Bothy[A-Z]\w*:?", token[at:]) is not None
        if not ok and bare and DISPLAY.search(rel):
            ok = True
        if not ok:
            return False
        at = low.find(NAME, at + len(NAME))
    return True


def _texts(paths: list[str], root: Path) -> list[tuple[str, str]]:
    """Returns path and text of every scanned file that contains the name."""
    out: list[tuple[str, str]] = []
    for rel in paths:
        if SKIP.search(rel):
            continue
        try:
            text = (root / rel).read_text(encoding="utf-8")
        except (UnicodeDecodeError, FileNotFoundError, IsADirectoryError):
            continue
        if NAME in text.lower():
            out.append((rel, text))
    return out


def offenders(paths: list[str], root: Path) -> list[str]:
    """Returns "file:line: token" for every hit outside the kinds."""
    texts = _texts(paths, root)
    alerts = frozenset(
        m.group(1).lower()
        for rel, text in texts
        if ALERT_FILES.search(rel)
        for m in ALERT.finditer(text)
    )
    out: list[str] = []
    for rel, text in texts:
        lines = text.splitlines()
        for number, line in enumerate(lines, 1):
            if NAME not in line.lower():
                continue
            following = COMMENT.sub("", lines[number]) if number < len(lines) else ""
            out.extend(
                f"{rel}:{number}: {token}"
                for start, token in tokens(line)
                if not allowed(rel, line, start, token, alerts, following)
            )
    return out


def tracked() -> list[str]:
    result = subprocess.run(
        ["git", "-C", str(REPO), "ls-files", "-z"], check=True, capture_output=True
    )
    return [p for p in result.stdout.decode().split("\0") if p]


def test_name_only_at_its_kinds() -> None:
    found = offenders(tracked(), REPO)
    assert found == [], "the working name outside the kinds of 02 6:\n" + "\n".join(found[:200])


def test_offenders_are_reported(tmp_path: Path) -> None:
    (tmp_path / "a.go").write_text(
        'package a\n// bothy migrate up\nconst x = "BOTHY_SERVER__ROLE"\n'
        'var y = "bothycorp"\nvar z = "MyBothyThing"\n'
        '// the Bothy server\nvar w = "bothy-server" + "bothy_app" + "/run/bothy"\n',
        encoding="utf-8",
    )
    (tmp_path / "docs").mkdir()
    (tmp_path / "docs" / "b.go").write_text("bothycorp\n", encoding="utf-8")
    (tmp_path / "c.md").write_text("bothycorp\n", encoding="utf-8")
    assert offenders(["a.go", "docs/b.go", "c.md"], tmp_path) == [
        "a.go:4: bothycorp",
        "a.go:5: MyBothyThing",
        "a.go:6: Bothy",
    ]


def test_bare_word_only_as_binary() -> None:
    assert allowed("a.go", "run `bothy serve`", 5, "bothy")
    assert allowed("a.go", "bothy migrate up", 0, "bothy")
    assert allowed("a.go", 'Default: "bothy",', 10, "bothy")
    assert allowed("compose.yaml", "  POSTGRES_DB: bothy", 15, "bothy")
    assert not allowed("a.go", "Welcome to bothy today", 11, "bothy")
    assert allowed("NOTICE", "Copyright 2026 The Bothy Authors", 19, "Bothy")
    assert allowed("a.go", "// the session settings of bothy", 27, "bothy", following="migrate up")


def test_forms_of_the_kinds() -> None:
    for token in (
        "github.com/OWNER/bothy/services/server",
        "cmd/bothy",
        "bothy-server:1.0",
        "bothy_app",
        "bothy.reason",
        "BOTHY_X",
        "sloth-slo-alerts-bothy-api",
        "file:/run/bothy/secrets/x",
        "bothyjob",
    ):
        assert allowed("a.go", token, 0, token), token
    for token in ("runBothy", "bothyBinary", "nonbothy", "BothyHomePage", "Bothy/1.0", "Bothy:"):
        assert not allowed("a.go", token, 0, token), token
    assert allowed(
        "rules.yaml", "BothyVolumeFull", 0, "BothyVolumeFull", frozenset({"bothyvolumefull"})
    )
    assert allowed("x.go", "bothyvolumefull", 0, "bothyvolumefull", frozenset({"bothyvolumefull"}))


def test_red_team_cases_are_offenders(tmp_path: Path) -> None:
    files = {
        "apps/x/lib/main.dart": (
            "title: 'Welcome to Bothy: sign in',\nText('bothy')\nclass BothyHomePage {}\n"
        ),
        "services/server/internal/x/x.go": 'msg := "Bothy/1.0 says hi"\n',
        "services/server/cmd/bothy/main.go": (
            'fmt.Println("Welcome to Bothy, the chat")\nfunc runBothy() {}\n'
        ),
    }
    for rel, text in files.items():
        (tmp_path / rel).parent.mkdir(parents=True, exist_ok=True)
        (tmp_path / rel).write_text(text, encoding="utf-8")
    assert offenders(list(files), tmp_path) == [
        "apps/x/lib/main.dart:1: Bothy:",
        "apps/x/lib/main.dart:2: bothy",
        "apps/x/lib/main.dart:3: BothyHomePage",
        "services/server/internal/x/x.go:1: Bothy/1.0",
        "services/server/cmd/bothy/main.go:1: Bothy",
    ]

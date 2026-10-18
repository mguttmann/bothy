# SPDX-License-Identifier: Apache-2.0
"""Gate `make signal-check`: dashboards, rules and SLOs use only the signal catalogue (M1-68).

The catalogue docs/observability/signals.md is the only list of metric names, units and
attributes (docs/spec/17-observability.md "Signals", R-OB-06). This check holds the files that
query the metrics to it:

1. Catalogue integrity: every metric row of tables 4.1 to 4.6 parses; the Prometheus column
   equals the translation rule of "Names and units" item 4 (dots to `_`, unit `s` to
   `_seconds`, `By` to `_bytes`, counters `_total`, units in braces dropped); every histogram
   has exactly one row in 4.7, and 4.7 names only histograms; no attribute column names a
   person or an ID attribute.
2. Names: every metric name in the `expr` of the dashboards (Prometheus targets, recursively),
   in the `expr` of the rule files and in `error_query`/`total_query` of the Sloth files is a
   Prometheus name of the catalogue (histograms also with `_bucket`, `_count`, `_sum`), a
   recording rule of the rule files, a series of section 3 (`target_info`), or `up`, `ALERTS`,
   `ALERTS_FOR_STATE`.
3. Labels: inside the selector braces of a catalogue metric every label is one of its
   attributes, a resource attribute of section 3 (both translated), `job`, `instance`, or `le`
   on a `_bucket` series.
4. Thresholds: every `le` matcher on a `_bucket` series is a bound of the histogram's 4.7 row
   (numeric comparison) or `+Inf`.
5. PromQL is lexed without a PromQL library: `{{.window}}` and Grafana variables become a
   duration, strings are skipped except as label values, identifiers before `(` are functions,
   identifiers in `by`, `without`, `on`, `ignoring`, `group_left` and `group_right` lists are
   labels, keywords, numbers and durations are skipped; the rest are metric names.
6. Syntax of the dashboards: every dashboard `expr` becomes a recording rule of a temporary
   rule file, checked by the pinned `promtool check rules` (tools/bin/tools.lock.json); a parse
   error names the dashboard and the panel.

Usage:
  signalcheck.py [--root <repository root>]

Exit 0 with a one-line summary; 1 with one line per finding (file, location, rule).

Example:
  uv run --project tools --locked --quiet python tools/signalcheck/signalcheck.py
"""

from __future__ import annotations

import argparse
import json
import math
import re
import subprocess
import sys
import tempfile
from collections.abc import Iterator, Sequence
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import yaml

# The Makefile runs this file as a script; the pin reader lives in tools/bin (pytest finds it
# through its pythonpath setting).
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "bin"))

import fetch_tools

REPO_ROOT = Path(__file__).resolve().parents[2]
"""Default `--root`: the checkout this file belongs to."""

SIGNALS_DOC = Path("docs/observability/signals.md")
"""The signal catalogue."""
DASHBOARD_DIR = Path("infra/observability/grafana/dashboards")
"""Grafana dashboards (JSON)."""
RULES_DIR = Path("infra/observability/prometheus/rules")
"""Prometheus rule files: hand-written alerts and the rules Sloth generates."""
SLO_DIR = Path("infra/observability/slo")
"""Sloth SLO files."""

PROMTOOL_TIMEOUT_S: float = 120
"""Seconds one promtool run may take."""

BUILTIN_SERIES = frozenset({"up", "ALERTS", "ALERTS_FOR_STATE"})
"""Series Prometheus creates itself."""
TARGET_LABELS = frozenset({"job", "instance"})
"""Labels Prometheus sets on every series of a target."""

KINDS = {
    "Histogram": "Histogram",
    "Histogramm": "Histogram",
    "Counter": "Counter",
    "Zähler": "Counter",
    "UpDownCounter": "UpDownCounter",
    "Gauge": "Gauge",
}
"""Kind column of the catalogue (English or German edition) to the kind."""

NO_ATTRIBUTES = frozenset({"none", "keine"})
"""Words of the attribute column for "no attribute"."""

PERSON_KEYS = frozenset(
    {
        "bothy.principal.pseudo_id",
        "bothy.correlation_id",
        "bothy.event.seq",
        "client.address",
        "session.id",
    }
)
"""Attribute keys that identify a person or carry an ID (the deny list of the Go registry)."""

KEYWORDS = frozenset({"and", "or", "unless", "bool", "offset", "inf", "nan", "atan2"})
"""PromQL keywords that are neither metrics nor functions."""
LABEL_LISTS = frozenset({"by", "without", "on", "ignoring", "group_left", "group_right"})
"""Keywords whose parenthesised list holds label names."""

_BACKTICK = re.compile(r"`([^`]+)`")
_SECTION4 = re.compile(r"^### 4\.(\d)([a-z]?)\b")
_CLIENT_ATTR = re.compile(r"`(bothy\.client\.[a-z_]+)` \(")
_GRAFANA_VAR = re.compile(r"\$\{[A-Za-z0-9_:]+\}|\$[A-Za-z_][A-Za-z0-9_]*")
_WINDOW = re.compile(r"\{\{\s*\.window\s*\}\}")
_TOKEN = re.compile(
    r"""
    (?P<space>\s+)
  | (?P<string>"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|`[^`]*`)
  | (?P<duration>(?:\d+(?:ms|[smhdwy]))+)
  | (?P<number>(?:0[xX][0-9a-fA-F]+|(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?))
  | (?P<ident>[A-Za-z_:][A-Za-z0-9_:]*)
  | (?P<op>=~|!~|!=|==|<=|>=|[-+*/%^<>=(){}\[\],@:])
  """,
    re.VERBOSE,
)


@dataclass
class Metric:
    """One metric of the catalogue as Prometheus sees it."""

    name: str
    kind: str | None
    unit: str
    prom: str
    labels: frozenset[str]
    attributes: tuple[str, ...]


@dataclass
class Catalogue:
    """What the check reads from signals.md."""

    metrics: dict[str, Metric] = field(default_factory=dict)
    """Catalogue metrics by Prometheus name."""
    resource_labels: frozenset[str] = frozenset()
    """Resource attributes of section 3, translated."""
    extra_series: frozenset[str] = frozenset()
    """Series named in section 3 (`target_info`)."""
    buckets: dict[str, list[float]] = field(default_factory=dict)
    """Bucket bounds of 4.7 by catalogue name."""


@dataclass(frozen=True)
class Finding:
    """One problem: file, location, rule number and message."""

    file: str
    location: str
    rule: int
    message: str

    def __str__(self) -> str:
        """Render the finding as one report line."""
        return f"{self.file}: {self.location}: rule {self.rule}: {self.message}"


@dataclass(frozen=True)
class Expression:
    """One PromQL expression of a file."""

    file: str
    location: str
    text: str


def translate(key: str) -> str:
    """Prometheus label or name of an OpenTelemetry key: dots and dashes become `_`."""
    return re.sub(r"[^A-Za-z0-9_:]", "_", key)


def prometheus_name(name: str, kind: str, unit: str) -> str:
    """The Prometheus name of a metric by the translation rule of signals.md item 4."""
    out = translate(name)
    if unit == "s":
        out += "_seconds"
    elif unit == "By":
        out += "_bytes"
    if kind == "Counter":
        out += "_total"
    return out


def person_key(key: str) -> bool:
    """Whether key names a person or an ID and so never appears on a metric."""
    return (
        key in PERSON_KEYS or key.endswith((".id", "_id")) or key.startswith(("user.", "enduser."))
    )


def split_row(line: str) -> list[str]:
    """Cells of a markdown table row; a `|` inside backticks is no separator."""
    line = line.strip().removeprefix("|").removesuffix("|")
    cells: list[str] = []
    cur: list[str] = []
    in_tick = False
    for ch in line:
        if ch == "`":
            in_tick = not in_tick
        if ch == "|" and not in_tick:
            cells.append("".join(cur).strip())
            cur = []
            continue
        cur.append(ch)
    cells.append("".join(cur).strip())
    return cells


def attribute_keys(cell: str) -> tuple[str, ...]:
    """The attribute keys of an attribute cell: backticked words outside parentheses."""
    if cell.strip() in NO_ATTRIBUTES:
        return ()
    keys: list[str] = []
    depth = 0
    i = 0
    while i < len(cell):
        ch = cell[i]
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
        elif ch == "`":
            end = cell.find("`", i + 1)
            if end < 0:
                break
            if depth == 0:
                keys.append(cell[i + 1 : end])
            i = end
        i += 1
    return tuple(keys)


def read_catalogue(text: str, file: str) -> tuple[Catalogue, list[Finding]]:
    """Parse signals.md and apply rule 1."""
    cat = Catalogue()
    findings: list[Finding] = []
    resource: set[str] = set()
    extra: set[str] = set()
    client_attrs: list[str] = []
    histograms: dict[str, str] = {}
    bucket_rows: dict[str, int] = {}
    section = ""
    for number, line in enumerate(text.splitlines(), start=1):
        loc = f"line {number}"
        if line.startswith("## "):
            section = "3" if line.startswith("## 3.") else ""
            continue
        m = _SECTION4.match(line)
        if m:
            section = "4." + m.group(1) + m.group(2)
            continue
        if section == "3":
            extra.update(w for w in _BACKTICK.findall(line) if w == "target_info")
            if line.startswith("| `"):
                resource.add(translate(_BACKTICK.findall(split_row(line)[0])[0]))
            continue
        if section == "4.6a" and not line.startswith("|"):
            client_attrs.extend(_CLIENT_ATTR.findall(line))
        if not line.startswith("| ") or not section:
            continue
        cells = split_row(line)
        if section == "4.7":
            _bucket_row(cells, loc, file, cat, bucket_rows, findings)
        elif section == "4.6a" and len(cells) == 5 and cells[1] in KINDS:
            _client_row(cells, loc, file, cat, client_attrs, histograms, findings)
        elif len(cells) == 6 and cells[0].startswith("`"):
            _metric_row(cells, loc, file, cat, histograms, findings)
    cat.resource_labels = frozenset(resource)
    cat.extra_series = frozenset(extra)
    for name, loc in histograms.items():
        if bucket_rows.get(name, 0) != 1:
            findings.append(
                Finding(
                    file,
                    loc,
                    1,
                    f"histogram {name} has {bucket_rows.get(name, 0)} rows in 4.7, want 1",
                )
            )
    for name in bucket_rows:
        if name not in histograms:
            findings.append(
                Finding(file, "4.7", 1, f"4.7 names {name}, which is no histogram of 4.1 to 4.6")
            )
    return cat, findings


def _bucket_row(
    cells: list[str],
    loc: str,
    file: str,
    cat: Catalogue,
    rows: dict[str, int],
    findings: list[Finding],
) -> None:
    """Read one row of 4.7."""
    if len(cells) != 2:
        return
    try:
        bounds = [float(v) for v in cells[1].split(",")]
    except ValueError:
        return  # the header
    if bounds != sorted(bounds):
        findings.append(Finding(file, loc, 1, "4.7 bounds are not ascending"))
    for name in _BACKTICK.findall(cells[0]):
        rows[name] = rows.get(name, 0) + 1
        cat.buckets[name] = bounds


def _check_attributes(
    name: str, keys: Sequence[str], loc: str, file: str, findings: list[Finding]
) -> None:
    """Rule 1: no attribute of a metric names a person or an ID."""
    for key in keys:
        if person_key(key):
            findings.append(
                Finding(
                    file, loc, 1, f"{name}: attribute {key} identifies a person or carries an ID"
                )
            )


def _metric_row(
    cells: list[str],
    loc: str,
    file: str,
    cat: Catalogue,
    histograms: dict[str, str],
    findings: list[Finding],
) -> None:
    """Read one metric row of 4.1 to 4.6: name, kind, unit, attributes, source, Prometheus."""
    names = _BACKTICK.findall(cells[0])
    proms = _BACKTICK.findall(cells[5])
    keys = list(attribute_keys(cells[3]))
    if not names or len(names) != len(proms):
        findings.append(
            Finding(
                file,
                loc,
                1,
                "metric row does not parse: names and Prometheus names differ in number",
            )
        )
        return
    kind = KINDS.get(cells[1])
    unit = cells[2].strip("`")
    _check_attributes(", ".join(names), keys, loc, file, findings)
    labels = frozenset(translate(k) for k in keys)
    for name, prom in zip(names, proms, strict=True):
        if kind is not None:
            want = prometheus_name(name, kind, unit)
            if prom != want:
                findings.append(
                    Finding(
                        file,
                        loc,
                        1,
                        f"{name}: Prometheus column {prom}, translation rule gives {want}",
                    )
                )
            if kind == "Histogram":
                histograms[name] = loc
        cat.metrics[prom] = Metric(name, kind, unit, prom, labels, tuple(keys))


def _client_row(
    cells: list[str],
    loc: str,
    file: str,
    cat: Catalogue,
    common: list[str],
    histograms: dict[str, str],
    findings: list[Finding],
) -> None:
    """Read one row of 4.6a: app name, kind, unit, further attributes, server name, Prometheus."""
    server = _BACKTICK.findall(cells[4])
    if len(server) != 2:
        findings.append(Finding(file, loc, 1, "client metric row does not parse"))
        return
    name, prom = server
    kind = KINDS[cells[1]]
    unit = cells[2].strip("`")
    keys = [*attribute_keys(cells[3]), *common]
    _check_attributes(name, keys, loc, file, findings)
    want = prometheus_name(name, kind, unit)
    if prom != want:
        findings.append(
            Finding(
                file, loc, 1, f"{name}: Prometheus column {prom}, translation rule gives {want}"
            )
        )
    if kind == "Histogram":
        histograms[name] = loc
    cat.metrics[prom] = Metric(
        name, kind, unit, prom, frozenset(translate(k) for k in keys), tuple(keys)
    )


# ---- PromQL ---------------------------------------------------------------------------------


@dataclass(frozen=True)
class Token:
    """One PromQL token."""

    kind: str
    text: str


@dataclass
class Selector:
    """A metric name with the label matchers of its braces."""

    name: str
    matchers: list[tuple[str, str, str]]


def normalise(expr: str) -> str:
    """Replace `{{.window}}` and Grafana variables by a duration (rule 5)."""
    return _GRAFANA_VAR.sub("5m", _WINDOW.sub("5m", expr))


def tokens(expr: str) -> list[Token]:
    """Lex expr into tokens without spaces; an unknown character is an `op` token of its own."""
    out: list[Token] = []
    pos = 0
    while pos < len(expr):
        m = _TOKEN.match(expr, pos)
        if m is None:
            out.append(Token("op", expr[pos]))
            pos += 1
            continue
        kind = m.lastgroup or "op"
        if kind != "space":
            out.append(Token(kind, m.group()))
        pos = m.end()
    return out


def selectors(expr: str) -> list[Selector]:
    """The metric names of expr with their matchers (rule 5)."""
    toks = tokens(normalise(expr))
    out: list[Selector] = []
    i = 0
    while i < len(toks):
        t = toks[i]
        nxt = toks[i + 1] if i + 1 < len(toks) else None
        if t.kind == "op" and t.text == "[":
            i = _skip_to(toks, i, "]")
            continue
        if t.kind == "op" and t.text == "{":
            _, i = _matchers(toks, i)  # a selector without a metric name
            continue
        if t.kind != "ident":
            i += 1
            continue
        low = t.text.lower()
        if low in LABEL_LISTS:
            i = _skip_to(toks, i + 1, ")") if nxt is not None and nxt.text == "(" else i + 1
            continue
        # A function or aggregation: followed by its parenthesis, or by a by/without list.
        if low in KEYWORDS or (
            nxt is not None and (nxt.text == "(" or nxt.text.lower() in ("by", "without"))
        ):
            i += 1
            continue
        sel = Selector(t.text, [])
        i += 1
        if i < len(toks) and toks[i].text == "{":
            sel.matchers, i = _matchers(toks, i)
        out.append(sel)
    return out


def _skip_to(toks: list[Token], i: int, close: str) -> int:
    """Index after the token close that ends the bracket at i."""
    while i < len(toks) and toks[i].text != close:
        i += 1
    return i + 1


def _matchers(toks: list[Token], i: int) -> tuple[list[tuple[str, str, str]], int]:
    """The label matchers of the braces at i and the index after them."""
    out: list[tuple[str, str, str]] = []
    i += 1
    while i < len(toks) and toks[i].text != "}":
        if (
            toks[i].kind in ("ident", "string")
            and i + 2 < len(toks)
            and toks[i + 1].text in ("=", "!=", "=~", "!~")
        ):
            label = toks[i].text.strip("\"'`")
            value = toks[i + 2].text
            if toks[i + 2].kind == "string":
                value = value[1:-1]
            out.append((label, toks[i + 1].text, value))
            i += 3
            continue
        i += 1
    return out, i + 1


# ---- sources -------------------------------------------------------------------------------


def dashboard_expressions(root: Path) -> Iterator[Expression]:
    """Yield the `expr` of every Prometheus target of the dashboards, recursively.

    A target whose nearest datasource is not Prometheus (Loki) is skipped.
    """
    for path in sorted((root / DASHBOARD_DIR).glob("*.json")):
        rel = str(path.relative_to(root))
        doc = json.loads(path.read_text(encoding="utf-8"))
        yield from _walk_dashboard(doc, rel, "prometheus", "")


def _walk_dashboard(node: Any, file: str, ds: str, panel: str) -> Iterator[Expression]:
    """Walk one JSON node with the inherited datasource type and panel title."""
    if isinstance(node, list):
        for item in node:
            yield from _walk_dashboard(item, file, ds, panel)
        return
    if not isinstance(node, dict):
        return
    source = node.get("datasource")
    if isinstance(source, dict) and isinstance(source.get("type"), str):
        ds = source["type"]
    if isinstance(node.get("title"), str) and ("targets" in node or "panels" in node):
        panel = node["title"]
    for key, value in node.items():
        if key == "expr" and isinstance(value, str):
            if ds == "prometheus":
                yield Expression(file, f"panel {panel!r}", value)
        else:
            yield from _walk_dashboard(value, file, ds, panel)


def rule_files(root: Path) -> list[Path]:
    """The Prometheus rule files."""
    return sorted((root / RULES_DIR).glob("*.y*ml"))


def rule_expressions(root: Path) -> tuple[list[Expression], set[str]]:
    """The `expr` of every rule and the names of the recording rules."""
    exprs: list[Expression] = []
    records: set[str] = set()
    for path in rule_files(root):
        rel = str(path.relative_to(root))
        doc = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
        for group in doc.get("groups", []) or []:
            for rule in group.get("rules", []) or []:
                name = rule.get("record") or rule.get("alert") or "?"
                if rule.get("record"):
                    records.add(str(rule["record"]))
                if isinstance(rule.get("expr"), str):
                    exprs.append(
                        Expression(rel, f"group {group.get('name')!r} rule {name!r}", rule["expr"])
                    )
    return exprs, records


def slo_expressions(root: Path) -> list[Expression]:
    """`error_query` and `total_query` of every SLO of the Sloth files."""
    out: list[Expression] = []
    for path in sorted((root / SLO_DIR).glob("*.y*ml")):
        rel = str(path.relative_to(root))
        doc = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
        for slo in doc.get("slos", []) or []:
            events = ((slo.get("sli") or {}).get("events")) or {}
            for key in ("error_query", "total_query"):
                if isinstance(events.get(key), str):
                    out.append(Expression(rel, f"slo {slo.get('name')!r} {key}", events[key]))
    return out


# ---- rules 2 to 4 --------------------------------------------------------------------------


def check_expression(e: Expression, cat: Catalogue, records: set[str]) -> list[Finding]:
    """Rules 2, 3 and 4 for one expression."""
    findings: list[Finding] = []
    for sel in selectors(e.text):
        metric, suffix = resolve(sel.name, cat)
        if metric is None:
            if (
                sel.name not in records
                and sel.name not in BUILTIN_SERIES
                and sel.name not in cat.extra_series
            ):
                findings.append(
                    Finding(
                        e.file,
                        e.location,
                        2,
                        f"{sel.name} is no catalogue metric, recording rule or built-in series",
                    )
                )
            continue
        for label, op, value in sel.matchers:
            if label == "le" and suffix == "_bucket":
                if op == "=":
                    findings.extend(_check_bound(e, metric, value, cat))
                continue
            if (
                label not in metric.labels
                and label not in cat.resource_labels
                and label not in TARGET_LABELS
            ):
                findings.append(
                    Finding(
                        e.file, e.location, 3, f"label {label} is no attribute of {metric.name}"
                    )
                )
    return findings


def resolve(name: str, cat: Catalogue) -> tuple[Metric | None, str]:
    """The catalogue metric of a series name and the histogram suffix it carries ("" for none)."""
    if name in cat.metrics:
        return cat.metrics[name], ""
    for suffix in ("_bucket", "_count", "_sum"):
        base = name.removesuffix(suffix)
        if base != name and base in cat.metrics and cat.metrics[base].kind == "Histogram":
            return cat.metrics[base], suffix
    return None, ""


def _check_bound(e: Expression, metric: Metric, value: str, cat: Catalogue) -> list[Finding]:
    """Rule 4: an `le` value is a bound of the histogram's 4.7 row or +Inf."""
    try:
        number = float(value)
    except ValueError:
        return [Finding(e.file, e.location, 4, f"le={value!r} of {metric.name} is no number")]
    if math.isinf(number) and number > 0:
        return []
    bounds = cat.buckets.get(metric.name, [])
    if not any(math.isclose(number, b, rel_tol=0, abs_tol=1e-12) for b in bounds):
        return [
            Finding(
                e.file, e.location, 4, f"le={value} is no bound of {metric.name} in 4.7 {bounds}"
            )
        ]
    return []


# ---- rule 6 --------------------------------------------------------------------------------


def promtool_path() -> Path:
    """The pinned promtool, fetched and sha256-checked on first use (tools/bin/tools.lock.json)."""
    lock = fetch_tools.load_lock(fetch_tools.DEFAULT_LOCK)
    return fetch_tools.ensure(
        lock, "promtool", fetch_tools.platform_key(), fetch_tools.DEFAULT_CACHE
    )


def check_syntax(exprs: Sequence[Expression], promtool: Path) -> list[Finding]:
    """Rule 6: check the expressions of each dashboard file with `promtool check rules`.

    On a failure each expression of that file runs alone, so the finding names its panel.
    """
    findings: list[Finding] = []
    by_file: dict[str, list[Expression]] = {}
    for e in exprs:
        by_file.setdefault(e.file, []).append(e)
    with tempfile.TemporaryDirectory(prefix="signalcheck-") as tmp:
        for file, group in by_file.items():
            if _promtool_ok(group, promtool, Path(tmp)):
                continue
            for e in group:
                if not _promtool_ok([e], promtool, Path(tmp)):
                    findings.append(
                        Finding(file, e.location, 6, "PromQL does not parse (promtool check rules)")
                    )
    return findings


def _promtool_ok(group: Sequence[Expression], promtool: Path, tmp: Path) -> bool:
    """Whether promtool accepts the expressions of group as recording rules."""
    rules = [
        {"record": f"signalcheck:panel_{i}", "expr": normalise(e.text)} for i, e in enumerate(group)
    ]
    path = tmp / "rules.yaml"
    path.write_text(
        yaml.safe_dump({"groups": [{"name": "signalcheck", "rules": rules}]}), encoding="utf-8"
    )
    env = {"PATH": "/usr/bin:/bin", "HOME": str(tmp), "TMPDIR": str(tmp)}
    done = subprocess.run(
        [str(promtool), "check", "rules", str(path)],
        capture_output=True,
        check=False,
        env=env,
        timeout=PROMTOOL_TIMEOUT_S,
    )
    return done.returncode == 0


# ---- main ----------------------------------------------------------------------------------


def run(root: Path, promtool: Path | None) -> tuple[list[Finding], str]:
    """Run every rule on the repository at root; promtool None skips rule 6."""
    cat_text = (root / SIGNALS_DOC).read_text(encoding="utf-8")
    cat, findings = read_catalogue(cat_text, str(SIGNALS_DOC))
    rules, records = rule_expressions(root)
    dashboards = list(dashboard_expressions(root))
    exprs = [*dashboards, *rules, *slo_expressions(root)]
    for e in exprs:
        findings.extend(check_expression(e, cat, records))
    if promtool is not None:
        findings.extend(check_syntax(dashboards, promtool))
    summary = f"signal-check: ok ({len(cat.metrics)} catalogue metrics, {len(exprs)} expressions)"
    return findings, summary


def main(argv: Sequence[str] | None = None) -> int:
    """Command line entry; see the module docstring."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0] if __doc__ else None)
    parser.add_argument("--root", type=Path, default=REPO_ROOT, help="repository root")
    args = parser.parse_args(argv)
    try:
        promtool = promtool_path()
    except (fetch_tools.LockError, fetch_tools.FetchError) as err:
        print(f"signal-check: promtool unavailable: {err}", file=sys.stderr)
        return 1
    findings, summary = run(args.root, promtool)
    for f in findings:
        print(f)
    if findings:
        return 1
    print(summary)
    return 0


if __name__ == "__main__":
    sys.exit(main())

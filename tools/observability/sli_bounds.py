# SPDX-License-Identifier: Apache-2.0
"""SLI bounds from the Sloth files for the server and the apps (25 3.3, R-VZ-03, VZ-T03).

Targets and thresholds of the SLOs live only in `infra/observability/slo/*.yaml`. This
generator reads, per SLO, `service/name`, the label `bothy_sli_metric`, the `objective`, the
threshold (the value of `le` in `error_query`, else in `total_query`, in seconds) and the period
(the file's `slo_period`, else the Sloth default of 30 days) and writes two files:

- `services/server/internal/stats/domain/slibounds.gen.go`: a map from the `sli.*` metric to
  SLO, objective, threshold and period; the own measurement counts `good` with the threshold,
  the traffic light uses the objective and the period;
- `packages/bothy_viz/lib/src/slo/slo_catalog.g.dart`: SLO, metric and l10n key per SLO, no
  thresholds (the server delivers the values in `getStatsSlo`).

Label rules (also the check `sli-labels` of `make ops-check`): every SLO carries exactly one
label `bothy_sli_metric`; its value is one of the `sli.*` values of the `CHECK` of
`stats_minutes.metric` in docs/spec/03; every such value belongs to exactly one SLO.

Usage:
  sli_bounds.py --write [--root <repo>]   regenerate both files
  sli_bounds.py --check [--root <repo>]   compare with a fresh run, print a diff; exit 1 on drift

Example:
  uv run --project tools --locked --quiet python tools/observability/sli_bounds.py --check
"""

from __future__ import annotations

import argparse
import difflib
import re
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml

REPO_ROOT = Path(__file__).resolve().parents[2]
"""Default `--root`: the checkout this file belongs to."""

SLO_DIR = Path("infra/observability/slo")
"""Sloth SLO files (format `prometheus/v1`)."""
DATA_MODEL_GLOB = "03-*.md"
"""Chapter 03 under docs/spec/ (core contract); the CHECK of `stats_minutes.metric`."""
GO_OUT = Path("services/server/internal/stats/domain/slibounds.gen.go")
"""Generated Go file of the server."""
DART_OUT = Path("packages/bothy_viz/lib/src/slo/slo_catalog.g.dart")
"""Generated Dart file of the apps."""
LABEL = "bothy_sli_metric"
"""SLO label that names the metric of the own measurement."""
SLOTH_DEFAULT_PERIOD_DAYS = 30
"""Sloth's default SLO period (docs/observability/slos.md 1 item 3)."""
GENERATOR = "tools/observability/sli_bounds.py"
"""Name the DO NOT EDIT headers give."""

_LE = re.compile(r"""\ble\s*=\s*(?:"([^"]*)"|'([^']*)'|`([^`]*)`)""")
_PERIOD = re.compile(r"^([1-9][0-9]*)d$")


class SliBoundsError(Exception):
    """The inputs cannot be read or break a label rule; `findings` holds one text per problem."""

    def __init__(self, findings: list[str]) -> None:
        """Keep the findings; the message joins them line by line."""
        super().__init__("\n".join(findings))
        self.findings = findings


@dataclass(frozen=True)
class Slo:
    """One SLO of a Sloth file.

    Attributes:
        file: path of the Sloth file relative to the root.
        slo: `service/name`.
        metric: value of the label `bothy_sli_metric`; `None` when absent.
        objective: target in percent.
        threshold_s: threshold in seconds from `le`; `None` for a ratio SLO.
        period_days: SLO period in days.
        label_problem: a duplicate label key, reported by `label_findings`.
    """

    file: str
    slo: str
    metric: str | None
    objective: float
    threshold_s: float | None
    period_days: int
    label_problem: str | None = None


class _DuplicateKeyLoader(yaml.SafeLoader):
    """A safe loader that records duplicate mapping keys instead of keeping the last one."""

    duplicates: list[str]


def _construct_mapping(loader: _DuplicateKeyLoader, node: yaml.MappingNode) -> dict[Any, Any]:
    seen: set[Any] = set()
    for key_node, _ in node.value:
        key = loader.construct_object(key_node)
        if key in seen:
            loader.duplicates.append(f"duplicate key {key} (line {key_node.start_mark.line + 1})")
        seen.add(key)
    return loader.construct_mapping(node)


_DuplicateKeyLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG,
    _construct_mapping,
)


def _load(path: Path) -> tuple[Any, list[str]]:
    loader = _DuplicateKeyLoader(path.read_text(encoding="utf-8"))
    loader.duplicates = []
    try:
        return loader.get_single_data(), loader.duplicates
    finally:
        loader.dispose()


def read_sli_metrics(root: Path) -> list[str]:
    """Return the `sli.*` values of the `CHECK` of `stats_minutes.metric`, sorted.

    A narrow parser: the table row of the field `metric` in the section `stats_minutes` of
    docs/spec/03-*.md and its `CHECK (metric IN (...))` list.

    Raises:
        SliBoundsError: the chapter, the section, the row or the list is missing.
    """
    chapters = sorted((root / "docs" / "spec").glob(DATA_MODEL_GLOB))
    if len(chapters) != 1:
        raise SliBoundsError([f"docs/spec/{DATA_MODEL_GLOB}: expected one chapter 03"])
    text = chapters[0].read_text(encoding="utf-8")
    section = re.search(r"^#### `stats_minutes`\n(.*?)(?=^#{1,4} )", text, re.M | re.S)
    if section is None:
        raise SliBoundsError([f"{chapters[0].name}: no section `stats_minutes`"])
    row = re.search(r"^\| `metric` \|.*?CHECK \(metric IN \(([^)]*)\)\)", section.group(1), re.M)
    if row is None:
        raise SliBoundsError([f"{chapters[0].name}: stats_minutes has no metric CHECK list"])
    values = re.findall(r"'([^']+)'", row.group(1))
    return sorted(v for v in values if v.startswith("sli."))


def threshold_seconds(error_query: str, total_query: str) -> float | None:
    """The threshold in seconds: `le` of `error_query`, else of `total_query`, else `None`.

    The value may stand in any PromQL quotes. Example: `le="0.5"` gives 0.5.
    """
    for query in (error_query, total_query):
        match = _LE.search(query)
        if match is not None:
            return float(next(g for g in match.groups() if g is not None))
    return None


def _period_days(spec: dict[str, Any], rel: str) -> int:
    raw = spec.get("slo_period")
    if raw is None:
        return SLOTH_DEFAULT_PERIOD_DAYS
    match = _PERIOD.match(str(raw))
    if match is None:
        raise SliBoundsError([f"{rel}: slo_period {raw} is not a number of days like 30d"])
    return int(match.group(1))


def read_slos(root: Path) -> list[Slo]:
    """Every SLO of every Sloth file under `root`, in file and list order.

    Raises:
        SliBoundsError: a file has no `slos` list or a bad `slo_period`.
    """
    slos: list[Slo] = []
    for path in sorted((root / SLO_DIR).glob("*.yaml")):
        rel = (SLO_DIR / path.name).as_posix()
        spec, duplicates = _load(path)
        if not isinstance(spec, dict) or not isinstance(spec.get("slos"), list):
            raise SliBoundsError([f"{rel}: no slos list"])
        period = _period_days(spec, rel)
        problem = "; ".join(d for d in duplicates if LABEL in d) or None
        for item in spec["slos"]:
            events = item.get("sli", {}).get("events", {})
            labels = item.get("labels") or {}
            metric = labels.get(LABEL)
            slos.append(
                Slo(
                    file=rel,
                    slo=f"{spec['service']}/{item['name']}",
                    metric=str(metric) if metric is not None else None,
                    objective=float(item["objective"]),
                    threshold_s=threshold_seconds(
                        str(events.get("error_query", "")), str(events.get("total_query", ""))
                    ),
                    period_days=period,
                    label_problem=problem,
                )
            )
    return slos


def label_findings(root: Path) -> list[str]:
    """The label rules of 25 3.3 against the real metric list; one text per broken rule."""
    metrics = read_sli_metrics(root)
    slos = read_slos(root)
    found: list[str] = []
    owners: dict[str, list[str]] = {}
    for slo in slos:
        where = f"{slo.file}: {slo.slo}"
        if slo.label_problem is not None:
            found.append(f"{where}: {slo.label_problem}")
        elif slo.metric is None:
            found.append(f"{where}: no label {LABEL}")
        elif slo.metric not in metrics:
            found.append(
                f"{where}: label {LABEL} is {slo.metric}, not an sli.* metric of"
                " stats_minutes.metric"
            )
        else:
            owners.setdefault(slo.metric, []).append(slo.slo)
    for metric in metrics:
        names = sorted(owners.get(metric, []))
        if not names:
            found.append(f"{metric}: no SLO carries it")
        elif len(names) > 1:
            found.append(f"{metric}: label of {len(names)} SLOs: {', '.join(names)}")
    return found


def _l10n_key(metric: str) -> str:
    words = metric.removeprefix("sli.").split("_")
    return "vizSli" + "".join(w.capitalize() for w in words)


def _go_duration_ms(seconds: float | None) -> str:
    if seconds is None:
        return "0"
    millis = round(seconds * 1000)
    return f"{millis} * time.Millisecond"


def _go_float(value: float) -> str:
    text = repr(value)
    return text[:-2] if text.endswith(".0") else text


def render_go(slos: list[Slo]) -> str:
    """The Go file: header, package doc, `SLIBound` and the sorted map `SLIBounds`."""
    lines = [
        f"// Code generated by {GENERATOR} from {SLO_DIR.as_posix()}/*.yaml. DO NOT EDIT.",
        "",
        "// Package domain holds the pure rules of the stats module. slibounds.gen.go carries",
        "// the SLO bounds of the Sloth files (docs/spec/25-visualization.md 3.3): the own",
        "// measurement counts an SLI event as good up to Threshold, and the traffic light",
        "// compares the good share over Period with Objective. Change the Sloth file and run",
        "// the generator; never edit this file.",
        "package domain",
        "",
        'import "time"',
        "",
        "// SLIBound is the bound of one SLO for its sli.* metric of stats_minutes.",
        "type SLIBound struct {",
        "\t// SLO is service/name of the Sloth file.",
        "\tSLO string",
        "\t// Objective is the target in percent.",
        "\tObjective float64",
        "\t// Threshold is the latency threshold; zero for a ratio SLO without one.",
        "\tThreshold time.Duration",
        "\t// Period is the SLO period.",
        "\tPeriod time.Duration",
        "}",
        "",
        "// SLIBounds maps each sli.* metric of stats_minutes to the bound of its SLO.",
        "var SLIBounds = map[string]SLIBound{",
    ]
    for slo in sorted(slos, key=lambda s: str(s.metric)):
        lines += [
            f'\t"{slo.metric}": {{',
            f'\t\tSLO:       "{slo.slo}",',
            f"\t\tObjective: {_go_float(slo.objective)},",
            f"\t\tThreshold: {_go_duration_ms(slo.threshold_s)},",
            f"\t\tPeriod:    {slo.period_days * 24} * time.Hour,",
            "\t},",
        ]
    lines.append("}")
    return "\n".join(lines) + "\n"


def render_dart(slos: list[Slo]) -> str:
    """The Dart file: header, library doc, `SloCatalogEntry` and the sorted list `sloCatalog`."""
    lines = [
        f"// Code generated by {GENERATOR} from {SLO_DIR.as_posix()}/*.yaml. DO NOT EDIT.",
        "",
        "/// The SLOs of the Sloth files with their metric of the own measurement and the l10n key",
        "/// of their display name (docs/spec/25-visualization.md 3.3). No thresholds: the server",
        "/// delivers the values in `getStatsSlo`.",
        "library;",
        "",
        "/// One SLO: `service/name`, its `sli.*` metric and the key of its display name.",
        "final class SloCatalogEntry {",
        "  /// Creates an entry; only the generated [sloCatalog] does.",
        "  const SloCatalogEntry({",
        "    required this.slo,",
        "    required this.metric,",
        "    required this.l10nKey,",
        "  });",
        "",
        "  /// `service/name` of the Sloth file, as `StatsSloEntry.slo` names it.",
        "  final String slo;",
        "",
        "  /// The `sli.*` metric of `stats_minutes` (label `bothy_sli_metric`).",
        "  final String metric;",
        "",
        "  /// Key of the display name in `AppLocalizations`.",
        "  final String l10nKey;",
        "}",
        "",
        "/// Every SLO, sorted by metric.",
        "const List<SloCatalogEntry> sloCatalog = <SloCatalogEntry>[",
    ]
    for slo in sorted(slos, key=lambda s: str(s.metric)):
        lines += [
            "  SloCatalogEntry(",
            f"    slo: '{slo.slo}',",
            f"    metric: '{slo.metric}',",
            f"    l10nKey: '{_l10n_key(str(slo.metric))}',",
            "  ),",
        ]
    lines.append("];")
    return "\n".join(lines) + "\n"


def render(root: Path) -> dict[Path, str]:
    """Both outputs from the files under `root`, keyed by their relative path.

    Raises:
        SliBoundsError: a label rule is broken (the findings of `label_findings`).
    """
    found = label_findings(root)
    if found:
        raise SliBoundsError(found)
    slos = read_slos(root)
    return {GO_OUT: render_go(slos), DART_OUT: render_dart(slos)}


def check(root: Path) -> list[str]:
    """Compare the checked-in outputs with a fresh run in a temporary folder.

    Returns:
        One finding per broken label rule, missing output or differing output (with a
        unified diff, checked-in file first); empty when everything matches.
    """
    try:
        fresh = render(root)
    except SliBoundsError as err:
        return err.findings
    found: list[str] = []
    with tempfile.TemporaryDirectory(prefix="sli-bounds-") as work:
        for rel, text in fresh.items():
            target = Path(work) / rel.name
            target.write_text(text, encoding="utf-8")
            committed = root / rel
            if not committed.is_file():
                found.append(f"{rel}: missing; run sli_bounds.py --write")
                continue
            have = committed.read_text(encoding="utf-8")
            want = target.read_text(encoding="utf-8")
            if have != want:
                diff = difflib.unified_diff(
                    have.splitlines(), want.splitlines(), str(rel), "fresh", lineterm="", n=1
                )
                found.append(f"{rel}: differs from a fresh run\n" + "\n".join(diff))
    return found


def main(argv: list[str] | None = None) -> int:
    """Command line; see the module docstring. Exit 0 clean or written, 1 with findings."""
    parser = argparse.ArgumentParser(description=(__doc__ or "").splitlines()[0])
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--write", action="store_true", help="regenerate both files")
    mode.add_argument("--check", action="store_true", help="compare with a fresh run")
    parser.add_argument("--root", type=Path, default=REPO_ROOT, help="repository root")
    args = parser.parse_args(argv)
    root: Path = args.root
    if args.check:
        found = check(root)
        for finding in found:
            print(finding, file=sys.stderr)
        return 1 if found else 0
    try:
        outputs = render(root)
    except SliBoundsError as err:
        for finding in err.findings:
            print(finding, file=sys.stderr)
        return 1
    for rel, text in outputs.items():
        (root / rel).parent.mkdir(parents=True, exist_ok=True)
        (root / rel).write_text(text, encoding="utf-8")
        print(f"  wrote {rel}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

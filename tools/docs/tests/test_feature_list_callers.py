# SPDX-License-Identifier: Apache-2.0
"""The feature list of the product chapter names for the apps only what an app can do.

docs/spec/01-product.md section 3 lists every feature with its milestone, the place where a
person finds it (column "Where") and its actions. The server and the API can be ahead of the
apps: an operation of the contract that no app calls is no feature of the desktop or the phone
(docs/decisions.d/AUD-docs-spec-01.md). So the list is held against the sources of the apps:

- a row of milestone `core` whose place is an app (D, M, W, macOS) names only actions of which
  at least one REST operation is called from the app sources, and every operation it names in
  parentheses is called;
- a row whose place is `API` names its operations in parentheses, and none of them is called
  from the app sources; the change that builds the view moves the row to the app.

An action belongs to its operations through `x-authz-action` of docs/api/openapi.yaml. The app
sources are the tracked Dart files under `lib/` of `apps/` and `packages/`, without the generated
client `packages/api_client` (it holds every operation) and without the fake server and the test
worlds under `lib/src/testing/`. A call is `.<operation>(`, the operation as a string, which is
how an outbox entry names it, or the path of the operation as a string when no other operation
shares the path, which is how a streamed download is requested; a mention in a comment is none.

The test lives with the tools tests because they run on every change, also on one that touches
only the chapter.
"""

from __future__ import annotations

import re
import subprocess
from functools import cache
from pathlib import Path
from typing import NamedTuple

import pytest

REPO = Path(__file__).resolve().parents[3]
PRODUCT = REPO / "docs" / "spec" / "01-product.md"
OPENAPI = REPO / "docs" / "api" / "openapi.yaml"
# The places of column "Where" that are an app, and the place of a function no app offers.
APP_PLACES = frozenset({"D", "M", "W", "macOS"})
API_PLACE = "API"
CORE = "core"
# Dart sources that are no app code: the generated client and the fake server of the tests.
NOT_APP_CODE = re.compile(r"^packages/api_client/|/lib/src/testing/")

_HEADER = "| Feature | Milestone | Where | Action(s) | Remark |"
_PATH = re.compile(r"^ {2}(?P<path>/[^\s:]*):\s*$")
_OPERATION = re.compile(r"^ {4,}operationId:\s*(?P<id>\w+)\s*$")
_ACTION = re.compile(r"^ {4,}x-authz-action:\s*(?P<action>[\w.]+)\s*$")
_PARAMETER = re.compile(r"\\\{\w+\\\}")
# A path parameter in a Dart string: `$id` or `${id.value}`.
_INTERPOLATION = r"\$(?:\{[^}]*\}|\w+)"
# A method call, a name as a string, and a string that holds a path.
_METHOD = re.compile(r"\.(\w+)\s*\(")
_NAME = re.compile(r"['\"](\w+)['\"]")
_PATH_STRING = re.compile(r"['\"]([^'\"\s]*/[^'\"\s]*)['\"]")
_CODE = re.compile(r"`(?P<code>[^`]+)`")
_GROUP = re.compile(r"\((?P<inner>[^()]*)\)")


class Row(NamedTuple):
    """A row of the feature list."""

    feature: str
    milestone: str
    places: frozenset[str]
    actions: tuple[str, ...]
    operations: tuple[str, ...]


def rows(chapter: str) -> list[Row]:
    """Return the rows of the feature tables of section 3 of the product chapter.

    The actions are the code spans of the column "Action(s)" outside parentheses, the operations
    those inside. Example: the cell `` `agent.invoke` (`retryRun`) `` gives the action
    `agent.invoke` and the operation `retryRun`.
    """
    section = chapter.split("\n## 3. Feature list", 1)[1].split("\n## 4. ", 1)[0]
    found, inside = [], False
    for line in section.splitlines():
        if line.strip() == _HEADER:
            inside = True
        elif not line.startswith("|"):
            inside = False
        elif inside and not line.startswith("|---"):
            cells = [c.strip() for c in re.split(r"(?<!\\)\|", line.strip().strip("|"))]
            if len(cells) != 5:
                raise ValueError(f"a row of the feature list has {len(cells)} cells: {line}")
            named = [
                c.group("code")
                for g in _GROUP.finditer(cells[3])
                for c in _CODE.finditer(g.group("inner"))
            ]
            actions = [c.group("code") for c in _CODE.finditer(_GROUP.sub("", cells[3]))]
            places = frozenset(p.strip() for p in cells[2].split(",") if p.strip())
            found.append(Row(cells[0], cells[1], places, tuple(actions), tuple(named)))
    return found


class Operation(NamedTuple):
    """A REST operation of the API contract."""

    id: str
    path: str
    action: str


class Contract(NamedTuple):
    """The REST operations of the API contract."""

    operations: tuple[Operation, ...]

    def by_action(self, action: str) -> list[Operation]:
        """Return the operations of an action; `group.*` stands for every action it starts."""
        if action.endswith(".*"):
            return [op for op in self.operations if op.action.startswith(action[:-1])]
        return [op for op in self.operations if op.action == action]

    def by_id(self, operation: str) -> Operation | None:
        """Return the operation of an ID, or None."""
        return next((op for op in self.operations if op.id == operation), None)

    def alone_on_its_path(self, operation: Operation) -> bool:
        """Return whether no other operation shares the path of the operation."""
        return [op.path for op in self.operations].count(operation.path) == 1


def read_contract(contract: str) -> Contract:
    """Return the operations of docs/api/openapi.yaml with path and `x-authz-action`."""
    found, path, operation = [], "", ""
    for line in contract.splitlines():
        if m := _PATH.match(line):
            path = m.group("path")
        elif m := _OPERATION.match(line):
            operation = m.group("id")
        elif (m := _ACTION.match(line)) and operation:
            found.append(Operation(operation, path, m.group("action")))
            operation = ""
    return Contract(tuple(found))


class AppCode(NamedTuple):
    """What the app sources call: method names and names as strings, and strings with a path."""

    names: frozenset[str]
    paths: tuple[str, ...]


def read_calls(code: str) -> AppCode:
    """Return the calls of a piece of Dart code.

    Example: `api.setThreadTags(id)` and `const tags = 'setThreadTags';` give the name
    `setThreadTags`, `dio.get('/threads/${id.value}/tags')` gives the path string; the comment
    ``/// see `setThreadTags` `` gives nothing.
    """
    names = set(_METHOD.findall(code)) | set(_NAME.findall(code))
    return AppCode(frozenset(names), tuple(sorted(set(_PATH_STRING.findall(code)))))


def is_called(operation: Operation, contract: Contract, code: AppCode) -> bool:
    """Return whether the app code calls the operation.

    A call is its name as a method or as a string, or, for the only operation of its path, the
    path as a string with `$name` or `${...}` for every parameter.
    """
    if operation.id in code.names:
        return True
    if not contract.alone_on_its_path(operation):
        return False
    path = _PARAMETER.sub(lambda _: _INTERPOLATION, re.escape(operation.path))
    request = re.compile(rf"{path}(?:\?.*)?$")
    return any(request.search(text) for text in code.paths)


@cache
def app_sources() -> tuple[str, ...]:
    """Return the repository paths of the app sources."""
    out = subprocess.run(
        ["git", "ls-files", "-z", "--", "apps/*/lib/*.dart", "packages/*/lib/*.dart"],
        cwd=REPO,
        capture_output=True,
        check=True,
        text=True,
    ).stdout
    return tuple(p for p in out.split("\0") if p and not NOT_APP_CODE.search(p))


@cache
def app_code() -> AppCode:
    """Return the calls of every app source."""
    return read_calls("\n".join((REPO / p).read_text(encoding="utf-8") for p in app_sources()))


def app_row_problems(row: Row, contract: Contract, code: AppCode) -> list[str]:
    """Return what an app row promises that no app calls."""
    found = []
    for action in row.actions:
        operations = contract.by_action(action)
        if operations and not any(is_called(op, contract, code) for op in operations):
            names = [op.id for op in operations]
            found.append(f"{row.feature!r}: no app calls an operation of `{action}` {names}")
    for name in row.operations:
        operation = contract.by_id(name)
        if operation is None:
            found.append(f"{row.feature!r}: `{name}` is no operation of the API")
        elif not is_called(operation, contract, code):
            found.append(f"{row.feature!r}: no app calls `{name}`")
    return found


def api_row_problems(row: Row, contract: Contract, code: AppCode) -> list[str]:
    """Return what is wrong with a row whose place is the API alone."""
    if not row.operations:
        return [f"{row.feature!r}: a row of the place API names its operations in parentheses"]
    found = []
    for name in row.operations:
        operation = contract.by_id(name)
        if operation is None:
            found.append(f"{row.feature!r}: `{name}` is no operation of the API")
        elif is_called(operation, contract, code):
            found.append(f"{row.feature!r}: an app calls `{name}`, so the place is the app")
    return found


CHAPTER = (
    "# Product\n\n## 3. Feature list\n\nLegend.\n\n### 3.1 One\n\n"
    f"{_HEADER}\n|---|---|---|---|---|\n"
    "| Posts with tags | core | D, M | `thread.create`, `thread.tags.set` | list |\n"
    "| Retry | core | D, M | `agent.invoke` (`retryRun`) | 6.2 |\n"
    "| Change tags | core | API | `thread.tags.set` (`setThreadTags`) | no app |\n"
    "| Extra keys (`\\|`, `~`) | core | M | | M8 |\n"
    "| Polls | never | | | no |\n\n## 4. Navigation\n"
)
CONTRACT = read_contract(
    "paths:\n  /threads:\n    post:\n      operationId: createThread\n"
    "      x-authz-action: thread.create\n"
    "  /threads/{thread_id}/tags:\n    put:\n      operationId: setThreadTags\n"
    "      summary: Set tags\n      x-authz-action: thread.tags.set\n"
    "  /runs/{run_id}/retry:\n    post:\n      operationId: retryRun\n"
    "      x-authz-action: agent.invoke\n"
    "  /channels/{channel_id}/export:\n    get:\n      operationId: exportChannel\n"
    "      x-authz-action: export.channel\n"
    "  /channels/{channel_id}/templates:\n    get:\n      operationId: listTemplates\n"
    "      x-authz-action: template.read\n"
    "    post:\n      operationId: createChannelTemplate\n"
    "      x-authz-action: template.manage\n"
    "  /groups:\n    post:\n      operationId: createGroup\n      x-authz-action: group.create\n"
)
SOURCES = read_calls(
    "await api.createThread(body);\n/// see `setThreadTags`\n"
    "static const String retryRun = 'retryRun';\n"
    "dio.get('/channels/${channel.value}/export');\n"
    "dio.get('/api/v1/channels/$id/templates?limit=5');\n"
)


def _operation(name: str) -> Operation:
    found = CONTRACT.by_id(name)
    assert found is not None, name
    return found


def test_rows_are_read_with_actions_and_operations() -> None:
    found = rows(CHAPTER)
    assert [r.feature for r in found] == [
        "Posts with tags",
        "Retry",
        "Change tags",
        "Extra keys (`\\|`, `~`)",
        "Polls",
    ]
    assert found[0] == Row(
        "Posts with tags", "core", frozenset({"D", "M"}), ("thread.create", "thread.tags.set"), ()
    )
    assert found[1].actions == ("agent.invoke",)
    assert found[1].operations == ("retryRun",)
    assert found[2].places == frozenset({"API"})
    assert found[4].places == frozenset()
    with pytest.raises(ValueError, match="cells"):
        rows(CHAPTER.replace("| Polls | never | | | no |", "| Polls | never | no |"))


def test_operations_are_read_with_path_and_action() -> None:
    assert _operation("setThreadTags") == Operation(
        "setThreadTags", "/threads/{thread_id}/tags", "thread.tags.set"
    )
    assert [op.id for op in CONTRACT.by_action("group.*")] == ["createGroup"]
    assert CONTRACT.by_action("typing.start") == []
    assert CONTRACT.by_id("setTags") is None
    assert CONTRACT.alone_on_its_path(_operation("exportChannel"))
    assert not CONTRACT.alone_on_its_path(_operation("listTemplates"))


def test_calls_are_read_from_the_code() -> None:
    assert {"createThread", "retryRun"} <= SOURCES.names
    assert "setThreadTags" not in SOURCES.names
    assert SOURCES.paths == (
        "/api/v1/channels/$id/templates?limit=5",
        "/channels/${channel.value}/export",
    )


def test_a_call_counts_and_a_comment_does_not() -> None:
    assert is_called(_operation("createThread"), CONTRACT, SOURCES)
    assert is_called(_operation("retryRun"), CONTRACT, SOURCES)
    assert not is_called(_operation("setThreadTags"), CONTRACT, SOURCES)


def test_a_request_by_path_counts_when_the_path_has_one_operation() -> None:
    assert is_called(_operation("exportChannel"), CONTRACT, SOURCES)
    assert not is_called(_operation("createChannelTemplate"), CONTRACT, SOURCES)
    assert not is_called(_operation("listTemplates"), CONTRACT, SOURCES)


def test_an_app_row_without_a_caller_is_reported() -> None:
    posts, retry, _, keys, _ = rows(CHAPTER)
    assert app_row_problems(posts, CONTRACT, SOURCES) == [
        "'Posts with tags': no app calls an operation of `thread.tags.set` ['setThreadTags']"
    ]
    assert app_row_problems(retry, CONTRACT, SOURCES) == []
    assert app_row_problems(keys, CONTRACT, SOURCES) == []
    renamed = retry._replace(operations=("retryRuns",))
    assert "no operation of the API" in app_row_problems(renamed, CONTRACT, SOURCES)[0]
    uncalled = retry._replace(operations=("setThreadTags",))
    assert app_row_problems(uncalled, CONTRACT, SOURCES) == [
        "'Retry': no app calls `setThreadTags`"
    ]


def test_an_api_row_with_a_caller_is_reported() -> None:
    change = rows(CHAPTER)[2]
    assert api_row_problems(change, CONTRACT, SOURCES) == []
    unnamed = change._replace(operations=())
    assert "in parentheses" in api_row_problems(unnamed, CONTRACT, SOURCES)[0]
    called = change._replace(operations=("createThread",))
    assert "so the place is the app" in api_row_problems(called, CONTRACT, SOURCES)[0]
    unknown = change._replace(operations=("setTags",))
    assert "no operation of the API" in api_row_problems(unknown, CONTRACT, SOURCES)[0]


def test_the_app_sources_leave_out_the_generated_client_and_the_fake_server() -> None:
    sources = app_sources()
    assert len(sources) > 500
    assert any(p.startswith("apps/client/lib/") for p in sources)
    assert any(p.startswith("packages/admin_ui/lib/") for p in sources)
    assert not any(NOT_APP_CODE.search(p) for p in sources)
    assert "createThread" in app_code().names
    assert NOT_APP_CODE.search("packages/api_client/lib/src/generated/api/threads_api.dart")
    assert NOT_APP_CODE.search("packages/sync_engine/lib/src/testing/world.dart")


def test_the_legend_explains_the_place_api() -> None:
    legend = PRODUCT.read_text(encoding="utf-8").split("\n## 3. Feature list", 1)[1]
    legend = " ".join(legend.split("\n### 3.1 ", 1)[0].split())
    assert "API = " in legend, "the legend of 01 section 3 does not explain the place API"


def test_every_app_row_of_the_feature_list_has_a_caller() -> None:
    contract = read_contract(OPENAPI.read_text(encoding="utf-8"))
    assert len(contract.operations) > 300
    found = [
        problem
        for row in rows(PRODUCT.read_text(encoding="utf-8"))
        if row.milestone == CORE and row.places & APP_PLACES
        for problem in app_row_problems(row, contract, app_code())
    ]
    assert found == [], "\n".join(found)


def test_every_api_row_of_the_feature_list_is_without_a_caller() -> None:
    contract = read_contract(OPENAPI.read_text(encoding="utf-8"))
    api_rows = [r for r in rows(PRODUCT.read_text(encoding="utf-8")) if API_PLACE in r.places]
    found = [p for row in api_rows for p in api_row_problems(row, contract, app_code())]
    assert found == [], "\n".join(found)
    mixed = [r.feature for r in api_rows if r.places != {API_PLACE}]
    assert mixed == [], f"a row is either of the API or of an app: {mixed}"

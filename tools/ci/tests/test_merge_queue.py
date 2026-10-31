# SPDX-License-Identifier: Apache-2.0
"""The merge queue of main as a repository ruleset (tools/ci/merge_queue.py).

docs/quality/gates.md, "How a pull request lands"; docs/decisions.d/CI-SPLIT-MERGE-QUEUE.md. The
tests replace `gh api` with a fake that keeps the state of the repository, so no call leaves the
machine.
"""

from __future__ import annotations

import json
from typing import Any

import merge_queue
import pytest

REPO = "octo/bothy"


class FakeGitHub:
    """The few endpoints of the REST API that merge_queue.py calls, with their state."""

    def __init__(self, *, owner_type: str = "Organization", queue_allowed: bool = True) -> None:
        self.owner_type = owner_type
        self.queue_allowed = queue_allowed
        self.rulesets: dict[int, dict[str, Any]] = {}
        self.variables: dict[str, str] = {}
        self.calls: list[tuple[str, str]] = []
        self.next_id = 7

    def __call__(self, method: str, path: str, body: dict[str, Any] | None = None) -> Any:
        self.calls.append((method, path))
        base = f"repos/{REPO}"
        if (method, path) == ("GET", base):
            return {"owner": {"type": self.owner_type}, "full_name": REPO}
        if (method, path) == ("GET", f"{base}/rulesets"):
            return [{"id": i, "name": r["name"]} for i, r in self.rulesets.items()]
        if method == "POST" and path == f"{base}/rulesets":
            assert body is not None
            self._check(body)
            self.rulesets[self.next_id] = {**body, "id": self.next_id}
            self.next_id += 1
            return self.rulesets[self.next_id - 1]
        if path.startswith(f"{base}/rulesets/"):
            ruleset_id = int(path.rsplit("/", 1)[1])
            if method == "GET":
                return self.rulesets[ruleset_id]
            if method == "PUT":
                assert body is not None
                self._check(body)
                self.rulesets[ruleset_id] = {**body, "id": ruleset_id}
                return self.rulesets[ruleset_id]
            if method == "DELETE":
                del self.rulesets[ruleset_id]
                return None
        if (method, path) == ("GET", f"{base}/branches/main/protection"):
            return {
                "required_status_checks": {
                    "strict": False,
                    "contexts": ["ci-ok"],
                    "checks": [{"context": "ci-ok", "app_id": 15368}],
                },
                "enforce_admins": {"enabled": True},
                "required_linear_history": {"enabled": True},
                "allow_force_pushes": {"enabled": False},
                "allow_deletions": {"enabled": False},
                "required_conversation_resolution": {"enabled": True},
            }
        if path.startswith(f"{base}/actions/variables/"):
            name = path.rsplit("/", 1)[1]
            if method == "GET":
                if name not in self.variables:
                    raise merge_queue.GitHubError(f"HTTP 404: {path}")
                return {"name": name, "value": self.variables[name]}
            if method == "PATCH":
                assert body is not None
                self.variables[name] = body["value"]
                return None
        if (method, path) == ("POST", f"{base}/actions/variables"):
            assert body is not None
            self.variables[body["name"]] = body["value"]
            return None
        raise AssertionError(f"unexpected call {method} {path}")

    def _check(self, body: dict[str, Any]) -> None:
        if not self.queue_allowed and any(r["type"] == "merge_queue" for r in body["rules"]):
            raise merge_queue.GitHubError("HTTP 422: Invalid rule 'merge_queue': ")


def test_the_ruleset_requires_a_batching_squash_queue_on_main() -> None:
    ruleset = merge_queue.ruleset()
    assert ruleset["target"] == "branch"
    assert ruleset["enforcement"] == "active"
    assert ruleset["conditions"] == {"ref_name": {"include": ["~DEFAULT_BRANCH"], "exclude": []}}
    (rule,) = ruleset["rules"]
    assert rule["type"] == "merge_queue"
    params = rule["parameters"]
    assert params["merge_method"] == "SQUASH"
    # Up to five pull requests land together; no wait for a minimum.
    assert params["max_entries_to_merge"] == 5
    assert params["min_entries_to_merge"] == 1
    # Only the head of a group must pass: one full run covers the whole batch.
    assert params["grouping_strategy"] == "HEADGREEN"
    # Two full runs at a time: each takes two macOS jobs of the five GitHub gives a free account,
    # and the run of the push to main needs two more.
    assert params["max_entries_to_build"] == 2
    # The longest job (test (server contract)) may take 240 minutes, plus waiting for a runner.
    assert params["check_response_timeout_minutes"] == 360


def test_apply_creates_the_ruleset_then_switches_the_pull_requests_to_the_fast_plan() -> None:
    gh = FakeGitHub()
    merge_queue.apply(REPO, gh)
    (stored,) = gh.rulesets.values()
    assert stored["name"] == merge_queue.RULESET_NAME
    assert stored["rules"] == merge_queue.ruleset()["rules"]
    assert gh.variables == {"BOTHY_MERGE_QUEUE": "true"}
    # The variable follows the ruleset: a pull request runs the fast plan only with a queue.
    order = [c for c in gh.calls if c[0] in ("POST", "PUT", "PATCH")]
    assert order[0] == ("POST", f"repos/{REPO}/rulesets")
    assert order[-1] == ("POST", f"repos/{REPO}/actions/variables")


def test_apply_twice_updates_the_one_ruleset() -> None:
    gh = FakeGitHub()
    merge_queue.apply(REPO, gh)
    gh.variables["BOTHY_MERGE_QUEUE"] = "false"
    merge_queue.apply(REPO, gh)
    assert len(gh.rulesets) == 1
    assert ("PUT", f"repos/{REPO}/rulesets/7") in gh.calls
    assert gh.variables == {"BOTHY_MERGE_QUEUE": "true"}


def test_apply_refuses_a_repository_of_a_personal_account() -> None:
    # GitHub offers merge queues only to repositories of organizations; nothing is changed.
    gh = FakeGitHub(owner_type="User")
    with pytest.raises(merge_queue.GitHubError, match="owned by an organization"):
        merge_queue.apply(REPO, gh)
    assert not gh.rulesets
    assert not gh.variables


def test_a_refused_ruleset_leaves_the_pull_requests_on_the_full_plan() -> None:
    gh = FakeGitHub(queue_allowed=False)
    with pytest.raises(merge_queue.GitHubError, match="merge_queue"):
        merge_queue.apply(REPO, gh)
    assert not gh.variables


def test_disable_switches_the_plan_back_before_it_removes_the_queue() -> None:
    gh = FakeGitHub()
    merge_queue.apply(REPO, gh)
    gh.calls.clear()
    merge_queue.disable(REPO, gh)
    assert gh.variables == {"BOTHY_MERGE_QUEUE": "false"}
    assert not gh.rulesets
    writes = [c for c in gh.calls if c[0] in ("PATCH", "DELETE")]
    assert writes == [
        ("PATCH", f"repos/{REPO}/actions/variables/BOTHY_MERGE_QUEUE"),
        ("DELETE", f"repos/{REPO}/rulesets/7"),
    ]


def test_show_reads_back_ruleset_protection_and_variable() -> None:
    gh = FakeGitHub()
    merge_queue.apply(REPO, gh)
    state = merge_queue.show(REPO, gh)
    assert state["ruleset"]["rules"] == merge_queue.ruleset()["rules"]
    assert state["ruleset"]["enforcement"] == "active"
    assert state["branch_protection"]["required_status_checks"] == ["ci-ok"]
    assert state["branch_protection"]["enforce_admins"] is True
    assert state["variable"] == {"BOTHY_MERGE_QUEUE": "true"}
    json.dumps(state)


def test_show_without_a_queue() -> None:
    state = merge_queue.show(REPO, FakeGitHub())
    assert state["ruleset"] is None
    assert state["variable"] == {"BOTHY_MERGE_QUEUE": None}


def test_cli_prints_the_ruleset(capsys: pytest.CaptureFixture[str]) -> None:
    assert merge_queue.main(["ruleset"]) == 0
    assert json.loads(capsys.readouterr().out) == merge_queue.ruleset()


def test_cli_reports_a_refusal(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setattr(merge_queue, "gh_api", FakeGitHub(owner_type="User"))
    assert merge_queue.main(["apply", "--repo", REPO]) == 1
    assert "owned by an organization" in capsys.readouterr().err


def test_gh_api_sends_the_body_on_standard_input(monkeypatch: pytest.MonkeyPatch) -> None:
    seen: dict[str, Any] = {}

    class Done:
        returncode = 0
        stdout = '{"id": 3}'
        stderr = ""

    def run(args: list[str], **kwargs: Any) -> Done:
        seen["args"] = args
        seen["input"] = kwargs.get("input")
        return Done()

    monkeypatch.setattr(merge_queue.subprocess, "run", run)
    assert merge_queue.gh_api("POST", "repos/x/y/rulesets", {"a": 1}) == {"id": 3}
    assert seen["args"] == ["gh", "api", "-X", "POST", "repos/x/y/rulesets", "--input", "-"]
    assert json.loads(seen["input"]) == {"a": 1}


def test_gh_api_raises_on_an_error(monkeypatch: pytest.MonkeyPatch) -> None:
    class Failed:
        returncode = 1
        stdout = ""
        stderr = "gh: Validation Failed (HTTP 422)"

    monkeypatch.setattr(merge_queue.subprocess, "run", lambda *a, **k: Failed())
    with pytest.raises(merge_queue.GitHubError, match="HTTP 422"):
        merge_queue.gh_api("GET", "repos/x/y")

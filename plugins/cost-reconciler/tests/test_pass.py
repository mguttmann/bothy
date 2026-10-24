# SPDX-License-Identifier: Apache-2.0
"""One pass end to end against the fakes: submissions, log lines and exit codes."""

from __future__ import annotations

import json
from pathlib import Path

import pytest
from conftest import A1, A2, A4, DAY1, DAY2, R1, R3, R4, R5, W1, W2, W3, World
from fakes import ESTIMATES_PATH, FakeStop, Reply, load_fixture

PROBLEM = "application/problem+json"


def test_world_of_the_brief(world: World) -> None:
    assert world.run("--once") == 0
    submissions = world.server.submissions()
    assert len(submissions) == 1
    body = submissions[0].json()
    assert body == {
        "provider": "anthropic",
        "period_start": DAY1,
        "period_end": "2026-10-21T00:00:00.000Z",
        "entries": [
            {"run_id": R1, "amount_micros": 6666734, "currency": "USD"},
            {"run_id": R3, "amount_micros": 3333366, "currency": "USD"},
            {"run_id": R5, "amount_micros": 2505000, "currency": "USD"},
        ],
    }
    estimates = [r for r in world.server.requests if r.path == ESTIMATES_PATH]
    assert [r.query["from"] for r in estimates] == [[DAY2], [DAY1]]
    for request in estimates:
        assert request.query["agent_id"] == [A1, A2, A4]
    not_assigned = world.events("not_assigned")
    assert {
        "bucket_start": DAY1,
        "workspace_id": W3,
        "currency": "USD",
        "amount_micros": 500000,
        "reason": "no_mapping",
    }.items() <= next(e for e in not_assigned if e["workspace_id"] == W3).items()
    no_runs = [e for e in not_assigned if e["reason"] == "no_runs"]
    assert len(no_runs) == 1
    assert no_runs[0]["bucket_start"] == DAY2
    assert no_runs[0]["workspace_id"] == W1
    assert no_runs[0]["amount_micros"] == 1000000
    assert len(not_assigned) == 2
    mismatch = world.events("currency_mismatch")
    assert len(mismatch) == 1
    assert mismatch[0]["run_ids"] == [R4]
    assert mismatch[0]["workspace_id"] == W1
    assert mismatch[0]["currency"] == "USD"
    submitted = world.events("submitted")
    assert len(submitted) == 1
    assert submitted[0]["bucket_start"] == DAY1
    assert submitted[0]["entry_count"] == 3
    assert submitted[0]["reconciled"] == 3
    assert world.events("unmatched_runs") == []
    assert world.logs()[0]["event"] == "pass_started"
    assert [e["event"] for e in world.logs()][-1] == "pass_finished"
    for line in world.logs():
        assert set(line) >= {"ts", "level", "event"}
        assert line["ts"].endswith("Z")


def test_log_order_follows_buckets(world: World) -> None:
    world.run("--once")
    starts = [e["bucket_start"] for e in world.logs() if "bucket_start" in e]
    assert starts == sorted(starts)


def test_unmatched_runs_are_logged(world: World) -> None:
    world.server.unmatched = {R5}
    assert world.run("--once") == 0
    unmatched = world.events("unmatched_runs")
    assert len(unmatched) == 1
    assert unmatched[0]["run_ids"] == [R5]
    assert unmatched[0]["bucket_start"] == DAY1
    assert world.events("submitted")[0]["reconciled"] == 2


def test_bucket_without_mapped_workspace_needs_no_estimates(world: World) -> None:
    world.workspace_map = {"wrkspc_other": [A1]}
    assert world.run("--once") == 0
    assert world.server.requests == []
    reasons = [(e["workspace_id"], e["reason"]) for e in world.events("not_assigned")]
    assert sorted(reasons) == sorted(
        [(W1, "no_mapping"), (W1, "no_mapping"), (W2, "no_mapping"), (W3, "no_mapping")]
    )


def test_zero_estimates_reason(world: World) -> None:
    world.server.estimates[DAY1] = [
        {**e, "estimate_micros": 0} for e in world.server.estimates[DAY1] if e["currency"] == "USD"
    ]
    assert world.run("--once") == 0
    reasons = {
        (e["bucket_start"], e["workspace_id"]): e["reason"] for e in world.events("not_assigned")
    }
    assert reasons[(DAY1, W1)] == "zero_estimates"
    assert reasons[(DAY1, W2)] == "zero_estimates"
    assert world.server.submissions() == []


def test_other_provider_runs_are_left_out(world: World) -> None:
    world.server.estimates[DAY1].append(
        {
            **world.server.estimates[DAY1][0],
            "run_id": "00000000-0000-7000-8000-000000000099",
            "provider": "other",
        }
    )
    assert world.run("--once") == 0
    entries = world.server.submissions()[0].json()["entries"]
    assert [e["run_id"] for e in entries] == [R1, R3, R5]


def test_null_workspace_is_not_assigned(world: World) -> None:
    page = load_fixture("cost_report_page2.json")
    page["data"][0]["results"][0]["workspace_id"] = None
    world.cost_api.pages = {None: page}
    assert world.run("--once") == 0
    (line,) = world.events("not_assigned")
    assert line["workspace_id"] is None
    assert line["reason"] == "no_mapping"


def test_cost_api_failure_submits_nothing(world: World) -> None:
    world.cost_api.override = Reply(500, b"oops", {})
    assert world.run("--once") == 1
    assert world.server.requests == []
    (failed,) = world.events("pass_failed")
    assert failed["stage"] == "cost_api"
    assert failed["status"] == 500
    assert "oops" not in world.stderr.getvalue()


def test_malformed_second_page_submits_nothing(world: World) -> None:
    world.cost_api.pages["page_2"] = load_fixture("cost_report_missing_amount.json")
    assert world.run("--once") == 1
    assert world.server.requests == []
    assert world.events("pass_failed")[0]["error"] == "malformed"


def test_invalid_amount_in_a_later_bucket_submits_nothing(world: World) -> None:
    page = load_fixture("cost_report_page2.json")
    page["data"][0]["starting_at"] = "2026-10-20T00:00:00Z"
    page["data"][0]["ending_at"] = "2026-10-21T00:00:00Z"
    page["data"][0]["results"][0]["amount"] = "1e5"
    world.cost_api.pages["page_2"] = page
    assert world.run("--once") == 1
    assert world.server.requests == []


def test_validation_failure_is_logged_with_pointers(world: World) -> None:
    world.server.submit_reply = Reply.of_json(
        400,
        {
            "code": "validation_failed",
            "errors": [{"pointer": "/entries/1/run_id", "code": "invalid"}],
        },
        PROBLEM,
    )
    assert world.run("--once") == 1
    (failed,) = world.events("pass_failed")
    assert failed["stage"] == "submit"
    assert failed["status"] == 400
    assert failed["code"] == "validation_failed"
    assert failed["errors"] == [{"pointer": "/entries/1/run_id", "code": "invalid"}]
    assert failed["bucket_start"] == DAY1


@pytest.mark.parametrize(
    ("status", "body"),
    [
        (401, {"code": "unauthenticated", "reason": "app_not_installed"}),
        (403, {"code": "forbidden", "reason": "scope_missing"}),
        (409, {"code": "request_in_progress"}),
        (422, {"code": "idempotency_key_reuse"}),
        (429, {"code": "rate_limited"}),
        (500, {"code": "internal"}),
    ],
)
def test_submission_errors_fail_the_pass(world: World, status: int, body: dict[str, str]) -> None:
    world.server.submit_reply = Reply.of_json(status, body, PROBLEM)
    assert world.run("--once") == 1
    (failed,) = world.events("pass_failed")
    assert failed["status"] == status
    assert failed["code"] == body["code"]
    assert failed.get("reason") == body.get("reason")
    assert len(world.server.submissions()) == 1
    assert world.events("pass_finished") == []


def test_estimates_error_fails_the_pass(world: World) -> None:
    world.server.estimates_reply = Reply.of_json(
        403, {"code": "forbidden", "reason": "scope_missing"}, PROBLEM
    )
    assert world.run("--once") == 1
    (failed,) = world.events("pass_failed")
    assert failed["stage"] == "estimates"
    assert world.server.submissions() == []


def test_network_error_of_the_server_fails_the_pass(world: World) -> None:
    world.server.close()
    assert world.run("--once") == 1
    assert world.events("pass_failed")[0]["error"] == "network"


def test_missing_config_file_is_exit_2(world: World, tmp_path: Path) -> None:
    from cost_reconciler.__main__ import run

    code = run(["--config", str(tmp_path / "absent.toml"), "--once"], world.runtime())
    assert code == 2
    (line,) = world.events("config_error")
    assert line["key"] == "--config"
    assert world.cost_api.requests == []


def test_invalid_config_is_exit_2(world: World) -> None:
    world.extra = 'lookback = "36h"\n'
    assert world.run("--once") == 2
    assert world.events("config_error")[0]["key"] == "reconciler.lookback"


def test_secret_with_open_mode_is_exit_2(world: World) -> None:
    world.write()
    (world.tmp / "admin_key").chmod(0o644)
    assert world.run("--once") == 2
    (line,) = world.events("config_error")
    assert line["key"] == "reconciler.admin_key_ref"
    assert line["rule"] == "insecure_file"
    assert world.cost_api.requests == []
    assert world.admin_key not in world.stderr.getvalue()


def test_missing_token_file_is_exit_2(world: World) -> None:
    world.write()
    (world.tmp / "token").unlink()
    code = world.run("--once")
    assert code == 2
    assert world.events("config_error")[0]["key"] == "reconciler.token_ref"


def test_loop_runs_passes_until_stopped(world: World) -> None:
    stop = FakeStop(world.clock, stop_after_waits=2)
    assert world.run(stop=stop) == 0
    assert stop.waits == [60.0, 60.0]
    assert len(world.events("pass_started")) == 2
    assert len(world.server.submissions()) == 2
    assert len(world.cost_api.requests) == 4
    # Every cost API request after the first started at least the interval after the previous.
    assert all(s <= 60.0 for s in world.clock.sleeps)


def test_loop_keeps_going_after_a_failed_pass(world: World) -> None:
    world.server.submit_reply = Reply.of_json(500, {"code": "internal"}, PROBLEM)
    stop = FakeStop(world.clock, stop_after_waits=2)
    assert world.run(stop=stop) == 0
    assert len(world.events("pass_failed")) == 2


def test_loop_keeps_going_after_a_secret_error(world: World) -> None:
    world.write()
    (world.tmp / "token").unlink()
    stop = FakeStop(world.clock, stop_after_waits=2)
    assert world.run(stop=stop) == 0
    assert len(world.events("config_error")) == 2
    assert world.cost_api.requests == []


def test_stop_before_the_first_pass(world: World) -> None:
    stop = FakeStop(world.clock)
    stop.set()
    assert world.run(stop=stop) == 0
    assert world.cost_api.requests == []


def test_stop_during_a_pass_ends_after_the_current_request(world: World) -> None:
    stop = FakeStop(world.clock)
    original = world.cost_api.responder

    def respond_then_stop(request):  # type: ignore[no-untyped-def]
        stop.set()
        return original(request)

    world.cost_api.responder = respond_then_stop
    assert world.run("--once", stop=stop) == 1
    assert len(world.cost_api.requests) == 1
    assert world.server.requests == []
    assert world.events("pass_failed")[0]["error"] == "interrupted"


def test_secrets_are_read_again_each_pass(world: World) -> None:
    stop = FakeStop(world.clock, stop_after_waits=2)
    world.write()
    first_token = world.token
    calls: list[str] = []
    original = world.server.responder

    def respond(request):  # type: ignore[no-untyped-def]
        calls.append(request.headers["authorization"])
        if len(calls) == 1:
            new = "canary" + "rotated" * 3
            path = world.tmp / "token"
            path.write_text(new, encoding="utf-8")
        return original(request)

    world.server.responder = respond
    world.run(stop=stop)
    assert calls[0] == f"Bearer {first_token}"
    assert calls[-1] == "Bearer canary" + "rotated" * 3


def test_log_lines_are_json(world: World) -> None:
    world.run("--once")
    for raw in world.stderr.getvalue().splitlines():
        line = json.loads(raw)
        assert line["level"] in {"info", "warning", "error"}

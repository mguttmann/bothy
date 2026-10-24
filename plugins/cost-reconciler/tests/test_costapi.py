# SPDX-License-Identifier: Apache-2.0
"""Cost API client: request shape, window, paging, interval, no redirects, errors."""

from __future__ import annotations

import copy
import secrets
import socket
from collections.abc import Iterator
from datetime import UTC, datetime, timedelta, timezone

import pytest
from fakes import COST_REPORT_PATH, FakeClock, FakeCostApi, FakeHTTP, Reply, load_fixture

from cost_reconciler.costapi import Bucket, CostApiClient, CostApiError, CostLine, report_window
from cost_reconciler.secretref import Secret
from cost_reconciler.transport import Pacer, StopRequestedError, build_opener

START = datetime(2026, 10, 18, tzinfo=UTC)
END = datetime(2026, 10, 21, tzinfo=UTC)


@pytest.fixture
def cost_api() -> Iterator[FakeCostApi]:
    fake = FakeCostApi()
    yield fake
    fake.close()


def client(url: str, clock: FakeClock, interval: float = 60.0) -> CostApiClient:
    return CostApiClient(
        base_url=url,
        timeout_s=5.0,
        pacer=Pacer(interval, clock.monotonic, clock.sleep),
        opener=build_opener(),
    )


def key() -> Secret:
    return Secret("canary" + secrets.token_hex(12))


def test_window_is_complete_utc_days() -> None:
    assert report_window(datetime(2026, 10, 21, 5, 30, tzinfo=UTC), 3) == (START, END)
    assert report_window(datetime(2026, 10, 21, tzinfo=UTC), 1) == (
        datetime(2026, 10, 20, tzinfo=UTC),
        END,
    )
    later_zone = timezone(timedelta(hours=5))
    assert report_window(datetime(2026, 10, 21, 3, 0, tzinfo=later_zone), 1) == (
        datetime(2026, 10, 19, tzinfo=UTC),
        datetime(2026, 10, 20, tzinfo=UTC),
    )


def test_window_needs_an_aware_time() -> None:
    with pytest.raises(ValueError, match="time zone"):
        report_window(datetime(2026, 10, 21), 1)


def test_request_shape_and_paging(cost_api: FakeCostApi) -> None:
    admin_key = key()
    buckets = client(cost_api.url, FakeClock()).fetch_report(admin_key, START, END)
    assert len(cost_api.requests) == 2
    first, second = cost_api.requests
    for request in (first, second):
        assert request.method == "GET"
        assert request.path == COST_REPORT_PATH
        assert request.headers["x-api-key"] == admin_key.reveal()
        assert request.headers["anthropic-version"] == "2023-06-01"
        assert "authorization" not in request.headers
        assert request.body == b""
        assert request.query["starting_at"] == ["2026-10-18T00:00:00Z"]
        assert request.query["ending_at"] == ["2026-10-21T00:00:00Z"]
        assert request.query["bucket_width"] == ["1d"]
        assert request.query["group_by[]"] == ["workspace_id", "description"]
    assert "page" not in first.query
    assert second.query["page"] == ["page_2"]
    assert buckets == [
        Bucket(
            starting_at=datetime(2026, 10, 19, tzinfo=UTC),
            ending_at=datetime(2026, 10, 20, tzinfo=UTC),
            lines=(CostLine("wrkspc_01e2ew1", "USD", "100"),),
        ),
        Bucket(
            starting_at=datetime(2026, 10, 20, tzinfo=UTC),
            ending_at=datetime(2026, 10, 21, tzinfo=UTC),
            lines=(
                CostLine("wrkspc_01e2ew1", "USD", "600.005"),
                CostLine("wrkspc_01e2ew1", "USD", "400.005"),
                CostLine("wrkspc_01e2ew2", "USD", "250.5"),
                CostLine("wrkspc_01e2ew3", "USD", "50"),
            ),
        ),
    ]


def test_interval_between_every_request(cost_api: FakeCostApi) -> None:
    clock = FakeClock()
    api = client(cost_api.url, clock, interval=60.0)
    api.fetch_report(key(), START, END)
    assert clock.sleeps == [60.0]
    clock.now += 45.0
    api.fetch_report(key(), START, END)
    assert clock.sleeps == [60.0, 15.0, 60.0]
    clock.now += 600.0
    api.fetch_report(key(), START, END)
    assert clock.sleeps == [60.0, 15.0, 60.0, 60.0]


def test_zero_interval_never_sleeps(cost_api: FakeCostApi) -> None:
    clock = FakeClock()
    client(cost_api.url, clock, interval=0.0).fetch_report(key(), START, END)
    assert clock.sleeps == []


def test_stop_during_the_wait_interrupts() -> None:
    clock = FakeClock()
    stopped = False

    def sleep(seconds: float) -> None:
        nonlocal stopped
        clock.sleep(seconds)
        stopped = True

    pacer = Pacer(60.0, clock.monotonic, sleep, lambda: stopped)
    pacer.wait()
    with pytest.raises(StopRequestedError):
        pacer.wait()


def test_buckets_split_over_pages_are_joined(cost_api: FakeCostApi) -> None:
    page1 = load_fixture("cost_report_page1.json")
    page2 = copy.deepcopy(page1)
    page1["data"][0]["results"] = page1["data"][0]["results"][:1]
    page2["data"][0]["results"] = page2["data"][0]["results"][1:]
    page2["has_more"] = False
    page2["next_page"] = None
    cost_api.pages = {None: page1, "page_2": page2}
    buckets = client(cost_api.url, FakeClock()).fetch_report(key(), START, END)
    assert len(buckets) == 1
    assert len(buckets[0].lines) == 4


def test_null_workspace_is_kept(cost_api: FakeCostApi) -> None:
    page = load_fixture("cost_report_page2.json")
    page["data"][0]["results"][0]["workspace_id"] = None
    cost_api.pages = {None: page}
    buckets = client(cost_api.url, FakeClock()).fetch_report(key(), START, END)
    assert buckets[0].lines == (CostLine(None, "USD", "100"),)


def test_redirect_is_not_followed(cost_api: FakeCostApi) -> None:
    target = FakeHTTP(lambda _: Reply.of_json(200, load_fixture("cost_report_page2.json")))
    try:
        for status in (301, 302, 303, 307, 308):
            cost_api.override = Reply(status, b"", {"Location": target.url + COST_REPORT_PATH})
            with pytest.raises(CostApiError) as info:
                client(cost_api.url, FakeClock()).fetch_report(key(), START, END)
            assert info.value.status == status
        assert target.requests == []
    finally:
        target.close()


@pytest.mark.parametrize("status", [400, 401, 403, 404, 429, 500, 503])
def test_error_status_carries_the_status_only(cost_api: FakeCostApi, status: int) -> None:
    admin_key = key()
    body = f"error detail {admin_key.reveal()} body-marker-5"
    cost_api.override = Reply(status, body.encode(), {"Content-Type": "text/plain"})
    with pytest.raises(CostApiError) as info:
        client(cost_api.url, FakeClock()).fetch_report(admin_key, START, END)
    assert info.value.status == status
    assert info.value.kind == "status"
    for text in (str(info.value), repr(info.value), str(info.value.args)):
        assert "body-marker-5" not in text
        assert admin_key.reveal() not in text
    assert str(status) in str(info.value)


def _malformed_pages() -> list[tuple[str, object]]:
    def page(mutate: object) -> object:
        value = load_fixture("cost_report_page2.json")
        assert callable(mutate)
        mutate(value)
        return value

    def drop(path: list[object]) -> object:
        def mutate(value: dict[str, object]) -> None:
            target: object = value
            for step in path[:-1]:
                target = target[step]  # type: ignore[index]
            del target[path[-1]]  # type: ignore[attr-defined]

        return page(mutate)

    def setv(path: list[object], new: object) -> object:
        def mutate(value: dict[str, object]) -> None:
            target: object = value
            for step in path[:-1]:
                target = target[step]  # type: ignore[index]
            target[path[-1]] = new  # type: ignore[index]

        return page(mutate)

    line = ["data", 0, "results", 0]
    return [
        ("no data", drop(["data"])),
        ("data not a list", setv(["data"], {})),
        ("no has_more", drop(["has_more"])),
        ("has_more not bool", setv(["has_more"], "false")),
        ("has_more without next_page", setv(["has_more"], True)),
        ("bucket not an object", setv(["data", 0], [])),
        ("no starting_at", drop(["data", 0, "starting_at"])),
        ("no ending_at", drop(["data", 0, "ending_at"])),
        ("bad starting_at", setv(["data", 0, "starting_at"], "yesterday")),
        ("naive starting_at", setv(["data", 0, "starting_at"], "2026-10-19T00:00:00")),
        ("end before start", setv(["data", 0, "ending_at"], "2026-10-18T00:00:00Z")),
        ("no results", drop(["data", 0, "results"])),
        ("results not a list", setv(["data", 0, "results"], {})),
        ("line not an object", setv(line, "x")),
        ("no amount", drop([*line, "amount"])),
        ("amount not a string", setv([*line, "amount"], 100)),
        ("amount invalid", setv([*line, "amount"], "1e2")),
        ("no currency", drop([*line, "currency"])),
        ("currency invalid", setv([*line, "currency"], "usd")),
        ("no workspace_id", drop([*line, "workspace_id"])),
        ("workspace_id not a string", setv([*line, "workspace_id"], 5)),
        ("page not an object", []),
    ]


@pytest.mark.parametrize(("case", "page"), _malformed_pages())
def test_malformed_page_fails(cost_api: FakeCostApi, case: str, page: object) -> None:
    cost_api.pages = {None: page}
    with pytest.raises(CostApiError) as info:
        client(cost_api.url, FakeClock()).fetch_report(key(), START, END)
    assert info.value.kind == "malformed", case


def test_malformed_second_page_fails_the_whole_report(cost_api: FakeCostApi) -> None:
    cost_api.pages["page_2"] = load_fixture("cost_report_missing_amount.json")
    with pytest.raises(CostApiError) as info:
        client(cost_api.url, FakeClock()).fetch_report(key(), START, END)
    assert info.value.kind == "malformed"
    assert len(cost_api.requests) == 2


def test_invalid_json_fails(cost_api: FakeCostApi) -> None:
    cost_api.override = Reply(200, b"{not json", {"Content-Type": "application/json"})
    with pytest.raises(CostApiError) as info:
        client(cost_api.url, FakeClock()).fetch_report(key(), START, END)
    assert info.value.kind == "malformed"


def test_network_error_fails() -> None:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    with pytest.raises(CostApiError) as info:
        client(f"http://127.0.0.1:{port}", FakeClock()).fetch_report(key(), START, END)
    assert info.value.kind == "network"
    assert info.value.status is None


def test_base_path_is_kept() -> None:
    fake = FakeHTTP(lambda _: Reply.of_json(200, load_fixture("cost_report_page2.json")))
    try:
        client(fake.url + "/prefix", FakeClock()).fetch_report(key(), START, END)
        assert fake.requests[0].path == "/prefix" + COST_REPORT_PATH
    finally:
        fake.close()


@pytest.mark.parametrize(
    "body",
    [
        b"[" * 200_000,
        b'{"data":[{"starting_at":"0001-01-01T00:00:00+01:00","ending_at":"2026-10-20T00:00:00Z",'
        b'"results":[]}],"has_more":false,"next_page":null}',
    ],
    ids=["deep-nesting", "time-overflow"],
)
def test_hostile_page_is_malformed_not_a_crash(cost_api: FakeCostApi, body: bytes) -> None:
    cost_api.override = Reply(200, body, {"Content-Type": "application/json"})
    with pytest.raises(CostApiError) as info:
        client(cost_api.url, FakeClock()).fetch_report(key(), START, END)
    assert info.value.kind == "malformed"

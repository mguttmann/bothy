# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/distribution/storeapi.py with a recording HTTP seam instead of the stores.

The real services (Google Play, App Store Connect) need the publisher's accounts (OQ-08); these
tests hold the requests each step sends, the tokens' signatures and every refusal.
"""

from __future__ import annotations

import base64
import json
import urllib.parse
from collections.abc import Callable, Mapping
from typing import Any

import common
import pytest
import storeapi
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec, padding, rsa
from cryptography.hazmat.primitives.asymmetric.utils import encode_dss_signature

NOW = 1_800_000_000.0


class FakeHttp:
    """Answers by `METHOD url-without-query`; records every request."""

    def __init__(self, routes: dict[str, Callable[[bytes | None], storeapi.Response]]) -> None:
        self.routes = routes
        self.calls: list[tuple[str, str, dict[str, str], bytes | None]] = []

    def __call__(
        self, method: str, url: str, headers: Mapping[str, str], body: bytes | None
    ) -> storeapi.Response:
        self.calls.append((method, url, dict(headers), body))
        key = f"{method} {url.split('?', maxsplit=1)[0]}"
        if key not in self.routes:
            return storeapi.Response(404, b'{"error": "no route"}')
        return self.routes[key](body)


def answer(data: Any, status: int = 200) -> Callable[[bytes | None], storeapi.Response]:
    return lambda _: storeapi.Response(status, json.dumps(data).encode())


def _b64url_decode(text: str) -> bytes:
    return base64.urlsafe_b64decode(text + "=" * (-len(text) % 4))


# ---- Google Play ----------------------------------------------------------------------------


@pytest.fixture
def rsa_key() -> rsa.RSAPrivateKey:
    return rsa.generate_private_key(public_exponent=65537, key_size=2048)


@pytest.fixture
def account(rsa_key: rsa.RSAPrivateKey) -> str:
    pem = rsa_key.private_bytes(
        serialization.Encoding.PEM,
        serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption(),
    ).decode()
    return json.dumps(
        {
            "type": "service_account",
            "client_email": "release@acme.iam.example.com",
            "private_key": pem,
            "token_uri": "https://oauth2.googleapis.com/token",
        }
    )


PKG = "org.acme.chat"
EDITS = f"{storeapi.PLAY_API}/{PKG}/edits"


def play_routes(version_code: int = 1002003) -> dict[str, Any]:
    return {
        "POST https://oauth2.googleapis.com/token": answer({"access_token": "ya29.token"}),
        f"POST {EDITS}": answer({"id": "e1"}),
        f"POST {storeapi.PLAY_UPLOAD}/{PKG}/edits/e1/bundles": answer(
            {"versionCode": version_code, "sha256": "x"}
        ),
        f"PUT {EDITS}/e1/tracks/internal": answer({"track": "internal"}),
        f"PUT {EDITS}/e1/tracks/stable": answer({"track": "stable"}),
        f"POST {EDITS}/e1:commit": answer({"id": "e1"}),
    }


def test_play_upload_runs_one_edit(account: str, rsa_key: rsa.RSAPrivateKey) -> None:
    http = FakeHttp(play_routes())
    play = storeapi.PlayPublisher(account, PKG, http=http, clock=lambda: NOW)
    play.upload(b"AAB", version_code=1002003, name="1.2.3", track="internal")
    assert [(m, u.split("?")[0]) for m, u, _, _ in http.calls] == [
        ("POST", "https://oauth2.googleapis.com/token"),
        ("POST", EDITS),
        ("POST", f"{storeapi.PLAY_UPLOAD}/{PKG}/edits/e1/bundles"),
        ("PUT", f"{EDITS}/e1/tracks/internal"),
        ("POST", f"{EDITS}/e1:commit"),
    ]
    # The sign-in: a JWT bearer assertion signed with the service account's key.
    form = urllib.parse.parse_qs(http.calls[0][3].decode())  # type: ignore[union-attr]
    assert form["grant_type"] == ["urn:ietf:params:oauth:grant-type:jwt-bearer"]
    head, claims, signature = form["assertion"][0].split(".")
    assert json.loads(_b64url_decode(head)) == {"alg": "RS256", "typ": "JWT"}
    body = json.loads(_b64url_decode(claims))
    assert body["iss"] == "release@acme.iam.example.com"
    assert body["scope"] == storeapi.PLAY_SCOPE
    assert body["exp"] - body["iat"] == 3600
    rsa_key.public_key().verify(
        _b64url_decode(signature),
        f"{head}.{claims}".encode(),
        padding.PKCS1v15(),
        hashes.SHA256(),
    )
    # The bundle goes as bytes, the track names the version code; the token is reused.
    _, url, headers, data = http.calls[2]
    assert url.endswith("?uploadType=media")
    assert headers["Content-Type"] == "application/octet-stream"
    assert headers["Authorization"] == "Bearer ya29.token"
    assert data == b"AAB"
    track = json.loads(http.calls[3][3])  # type: ignore[arg-type]
    assert track == {
        "track": "internal",
        "releases": [{"name": "1.2.3", "versionCodes": ["1002003"], "status": "completed"}],
    }


def test_play_refuses_a_bundle_with_another_version_code(account: str) -> None:
    http = FakeHttp(play_routes(version_code=7))
    play = storeapi.PlayPublisher(account, PKG, http=http, clock=lambda: NOW)
    with pytest.raises(common.StepError, match="version code 7, the build has 1002003"):
        play.upload(b"AAB", version_code=1002003, name="1.2.3", track="internal")
    assert not any(u.endswith(":commit") for _, u, _, _ in http.calls)


def test_play_promotion_sets_the_stable_track(account: str) -> None:
    http = FakeHttp(play_routes())
    storeapi.PlayPublisher(account, PKG, http=http, clock=lambda: NOW).promote(
        version_code=1002003, name="1.2.3", track="stable"
    )
    assert [u for _, u, _, _ in http.calls][-2:] == [
        f"{EDITS}/e1/tracks/stable",
        f"{EDITS}/e1:commit",
    ]


def test_a_refusal_names_the_step_and_the_status(account: str) -> None:
    routes = play_routes()
    routes[f"POST {EDITS}"] = answer({"error": {"message": "app not found"}}, status=404)
    play = storeapi.PlayPublisher(account, PKG, http=FakeHttp(routes), clock=lambda: NOW)
    with pytest.raises(common.StepError, match=r"a Play edit refused with 404: .*app not found"):
        play.upload(b"AAB", version_code=1002003, name="1.2.3", track="internal")


def test_a_bad_service_account_is_refused() -> None:
    with pytest.raises(common.StepError, match="no service account key"):
        storeapi.PlayPublisher("{}", PKG)
    with pytest.raises(common.StepError, match="no service account key"):
        storeapi.PlayPublisher("not json", PKG)
    ec_pem = (
        ec.generate_private_key(ec.SECP256R1())
        .private_bytes(
            serialization.Encoding.PEM,
            serialization.PrivateFormat.PKCS8,
            serialization.NoEncryption(),
        )
        .decode()
    )
    with pytest.raises(common.StepError, match="no RSA key"):
        storeapi.PlayPublisher(json.dumps({"client_email": "a@b", "private_key": ec_pem}), PKG)


def test_an_answer_that_is_no_json_is_a_failure(account: str) -> None:
    routes = play_routes()
    routes["POST https://oauth2.googleapis.com/token"] = lambda _: storeapi.Response(200, b"<html>")
    play = storeapi.PlayPublisher(account, PKG, http=FakeHttp(routes), clock=lambda: NOW)
    with pytest.raises(common.StepError, match="no JSON"):
        play.promote(version_code=1, name="0.0.1", track="stable")


@pytest.mark.parametrize(
    ("route", "body", "message"),
    [
        ("POST https://oauth2.googleapis.com/token", {"token_type": "Bearer"}, "the answer has no"),
        (f"POST {EDITS}", ["e1"], "the answer has no id"),
        (
            f"POST {storeapi.PLAY_UPLOAD}/{PKG}/edits/e1/bundles",
            {},
            "the answer has no versionCode",
        ),
        (
            f"POST {storeapi.PLAY_UPLOAD}/{PKG}/edits/e1/bundles",
            {"versionCode": "seven"},
            "version code seven",
        ),
    ],
)
def test_a_malformed_play_answer_is_a_named_failure(
    account: str, route: str, body: Any, message: str
) -> None:
    routes = play_routes()
    routes[route] = answer(body)
    play = storeapi.PlayPublisher(account, PKG, http=FakeHttp(routes), clock=lambda: NOW)
    with pytest.raises(common.StepError, match=message):
        play.upload(b"AAB", version_code=1002003, name="1.2.3", track="internal")


def test_urllib_http_talks_only_to_the_stores() -> None:
    with pytest.raises(common.StepError, match="refusing a request"):
        storeapi.urllib_http("GET", "https://example.com/x", {}, None)


# ---- App Store Connect ----------------------------------------------------------------------


@pytest.fixture
def ec_key() -> ec.EllipticCurvePrivateKey:
    return ec.generate_private_key(ec.SECP256R1())


@pytest.fixture
def p8(ec_key: ec.EllipticCurvePrivateKey) -> str:
    return ec_key.private_bytes(
        serialization.Encoding.PEM,
        serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption(),
    ).decode()


class Clock:
    """A monotonic clock that the fake sleep moves."""

    def __init__(self) -> None:
        self.now = 0.0
        self.sleeps: list[float] = []

    def sleep(self, seconds: float) -> None:
        self.sleeps.append(seconds)
        self.now += seconds


def asc(p8: str, http: FakeHttp, clock: Clock | None = None) -> storeapi.AppStoreConnect:
    c = clock or Clock()
    return storeapi.AppStoreConnect(
        p8,
        "KEY123",
        "issuer-1",
        http=http,
        clock=lambda: NOW,
        monotonic=lambda: c.now,
        sleep=c.sleep,
    )


API = storeapi.ASC_API


def test_asc_token_is_es256_with_the_key_id(p8: str, ec_key: ec.EllipticCurvePrivateKey) -> None:
    http = FakeHttp(
        {
            f"GET {API}/apps": answer(
                {"data": [{"id": "6400", "attributes": {"bundleId": "org.acme.chat"}}]}
            )
        }
    )
    assert asc(p8, http).app_id("org.acme.chat") == "6400"
    _, url, headers, _ = http.calls[0]
    assert urllib.parse.parse_qs(urllib.parse.urlparse(url).query) == {
        "filter[bundleId]": ["org.acme.chat"]
    }
    head, claims, signature = headers["Authorization"].removeprefix("Bearer ").split(".")
    assert json.loads(_b64url_decode(head)) == {"alg": "ES256", "kid": "KEY123", "typ": "JWT"}
    body = json.loads(_b64url_decode(claims))
    assert body == {
        "iss": "issuer-1",
        "iat": int(NOW),
        "exp": int(NOW) + 1200,
        "aud": "appstoreconnect-v1",
    }
    raw = _b64url_decode(signature)
    der = encode_dss_signature(int.from_bytes(raw[:32], "big"), int.from_bytes(raw[32:], "big"))
    ec_key.public_key().verify(der, f"{head}.{claims}".encode(), ec.ECDSA(hashes.SHA256()))


def test_asc_without_the_app_names_the_owner_step(p8: str) -> None:
    http = FakeHttp({f"GET {API}/apps": answer({"data": []})})
    with pytest.raises(common.StepError, match=r"has no app org\.acme\.chat; create its record"):
        asc(p8, http).app_id("org.acme.chat")


def test_asc_waits_until_the_build_is_processed(p8: str) -> None:
    states = iter(["", "PROCESSING", "VALID"])

    def builds(_: bytes | None) -> storeapi.Response:
        state = next(states)
        data = [] if not state else [{"id": "b1", "attributes": {"processingState": state}}]
        return storeapi.Response(200, json.dumps({"data": data}).encode())

    clock = Clock()
    http = FakeHttp({f"GET {API}/builds": builds})
    assert asc(p8, http, clock).processed_build("6400", 1002003) == "b1"
    assert len(http.calls) == 3
    assert clock.sleeps == [30.0, 30.0]
    query = urllib.parse.parse_qs(urllib.parse.urlparse(http.calls[0][1]).query)
    assert query == {"filter[app]": ["6400"], "filter[version]": ["1002003"]}


def test_asc_stops_on_a_refused_build_and_after_its_patience(p8: str) -> None:
    http = FakeHttp(
        {
            f"GET {API}/builds": answer(
                {"data": [{"id": "b1", "attributes": {"processingState": "INVALID"}}]}
            )
        }
    )
    with pytest.raises(common.StepError, match="refused build 7: INVALID"):
        asc(p8, http).processed_build("6400", 7)
    clock = Clock()
    http = FakeHttp({f"GET {API}/builds": answer({"data": []})})
    with pytest.raises(common.StepError, match="App Store Connect processes build 7"):
        asc(p8, http, clock).processed_build("6400", 7)
    assert clock.now >= storeapi.PROCESSING_PATIENCE_SECONDS


def test_asc_adds_the_build_to_the_named_group(p8: str) -> None:
    http = FakeHttp(
        {
            f"GET {API}/apps/6400/betaGroups": answer(
                {
                    "data": [
                        {"id": "g1", "attributes": {"name": "stable"}},
                        {"id": "g2", "attributes": {"name": "beta"}},
                    ]
                }
            ),
            f"POST {API}/betaGroups/g2/relationships/builds": lambda _: storeapi.Response(204, b""),
        }
    )
    asc(p8, http).add_to_group("6400", "b1", "beta")
    method, url, _, body = http.calls[-1]
    assert (method, url) == ("POST", f"{API}/betaGroups/g2/relationships/builds")
    assert json.loads(body) == {"data": [{"type": "builds", "id": "b1"}]}  # type: ignore[arg-type]
    with pytest.raises(common.StepError, match="no TestFlight group gamma; create the groups"):
        asc(p8, http).add_to_group("6400", "b1", "gamma")


@pytest.mark.parametrize(
    ("route", "body", "call"),
    [
        (f"GET {API}/apps", {"data": "x"}, lambda c: c.app_id("org.acme.chat")),
        (f"GET {API}/apps", ["x"], lambda c: c.app_id("org.acme.chat")),
        (
            f"GET {API}/builds",
            {"data": [{"attributes": {"processingState": "VALID"}}]},
            lambda c: c.processed_build("6400", 7),
        ),
        (
            f"GET {API}/apps/6400/betaGroups",
            {"data": [{"attributes": {"name": "beta"}}]},
            lambda c: c.add_to_group("6400", "b1", "beta"),
        ),
    ],
)
def test_a_malformed_asc_answer_is_a_named_failure(
    p8: str, route: str, body: Any, call: Callable[[storeapi.AppStoreConnect], object]
) -> None:
    with pytest.raises(common.StepError, match="the answer"):
        call(asc(p8, FakeHttp({route: answer(body)})))


def test_asc_refuses_a_key_that_is_no_ec_key(rsa_key: rsa.RSAPrivateKey) -> None:
    with pytest.raises(common.StepError, match="no PEM private key"):
        storeapi.AppStoreConnect("nonsense", "K", "I")
    pem = rsa_key.private_bytes(
        serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption()
    ).decode()
    with pytest.raises(common.StepError, match="no EC key"):
        storeapi.AppStoreConnect(pem, "K", "I")

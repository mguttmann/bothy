# SPDX-License-Identifier: Apache-2.0
"""The Jira signature check (brief M2-50 Design note 5; 11 10.2; RK-83; PLAN row: "signature
with ``sha256`` valid, an unknown method rejected instead of assuming SHA-256 (11, 10.2), a wrong
and a missing signature rejected").

The test values are the example of GitHub's documentation, which Jira's webhook signature
follows (``X-Hub-Signature: <method>=<hex signature>``):

- 11 10.1: https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries
  (accessed 2026-10-10)
- 11 10.2: https://developer.atlassian.com/cloud/jira/platform/webhooks/ (accessed 2026-10-10)
"""

from __future__ import annotations

import hmac

import pytest

from jira_app.jira import signature
from jira_app.jira.signature import verify_signature

# Secret, payload and digest of the documented example (11 10.1, URL above, 2026-10-10).
SECRET = b"It's a Secret to Everybody"
BODY = b"Hello, World!"
DIGITS = "757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17"
VALID = f"sha256={DIGITS}"


def _flip_last(digits: str) -> str:
    last = "0" if digits[-1] != "0" else "1"
    return digits[:-1] + last


def test_sha256_valid() -> None:
    assert verify_signature(SECRET, BODY, [VALID]) is None
    assert verify_signature(SECRET, BODY, [f"sha256={DIGITS.upper()}"]) is None
    # The module's own signer produces exactly the documented value.
    assert signature.sign_body(SECRET, BODY) == VALID


UNSUPPORTED = [
    pytest.param("sha1=01dc10d0c83e72ed246219cdd91669667fe2ca59", id="sha1"),
    pytest.param(
        "sha512=11ed355a617e98134e842012a7944ccf59c10256cb182357bd7e3a42013ff07c"
        "376f8c14cf5cc1923da20b51d64256b2fb8ebbf100aa67a61326f61fea8111bc",
        id="sha512",
    ),
    pytest.param(f"md5={DIGITS}", id="md5-with-sha256-digits"),
    pytest.param(f"SHA256={DIGITS}", id="upper-SHA256-with-sha256-digits"),
    pytest.param("sha384=" + "ab" * 48, id="sha384-96-hex"),
]


@pytest.mark.parametrize("value", UNSUPPORTED)
def test_unknown_method_refused_instead_of_sha256(
    value: str, monkeypatch: pytest.MonkeyPatch
) -> None:
    calls: list[tuple[bytes, bytes]] = []
    real = signature.compute_hmac

    def spy(secret: bytes, body: bytes) -> bytes:
        calls.append((secret, body))
        return real(secret, body)

    monkeypatch.setattr(signature, "compute_hmac", spy)

    assert verify_signature(SECRET, BODY, [value]) == "unsupported_method"
    assert calls == []

    # The spy is effective: a supported method does reach the HMAC.
    assert verify_signature(SECRET, BODY, [VALID]) is None
    assert len(calls) == 1


def test_wrong_signature_refused() -> None:
    assert verify_signature(SECRET, BODY, [f"sha256={_flip_last(DIGITS)}"]) == "mismatch"
    assert verify_signature(SECRET, b"Hello, World?", [VALID]) == "mismatch"
    assert verify_signature(b"another secret", BODY, [VALID]) == "mismatch"


def test_missing_signature_refused() -> None:
    assert verify_signature(SECRET, BODY, []) == "missing"


MALFORMED = [
    pytest.param(["sha256"], id="no-equals"),
    pytest.param([f"={DIGITS}"], id="empty-method"),
    pytest.param(["sha256="], id="empty-signature"),
    pytest.param(["sha256=757107ea"], id="short"),
    pytest.param([f"sha256={DIGITS[:-1]}"], id="63-hex"),
    pytest.param([f"sha256={DIGITS}0"], id="65-hex"),
    pytest.param([f"sha256={DIGITS[:-1]}g"], id="non-hex"),
    pytest.param([VALID, VALID], id="two-headers"),
]


@pytest.mark.parametrize("values", MALFORMED)
def test_malformed_signature_refused(values: list[str]) -> None:
    # None raises: the reason is returned for every value.
    assert verify_signature(SECRET, BODY, values) == "malformed"


def test_constant_time_comparison(monkeypatch: pytest.MonkeyPatch) -> None:
    real = hmac.compare_digest
    wrong = f"sha256={_flip_last(DIGITS)}"
    seen: list[tuple[object, object]] = []

    def recording(a: bytes, b: bytes) -> bool:
        seen.append((a, b))
        return real(a, b)

    monkeypatch.setattr(hmac, "compare_digest", recording)
    for value, expected in ((VALID, None), (wrong, "mismatch")):
        seen.clear()
        assert verify_signature(SECRET, BODY, [value]) == expected
        assert len(seen) == 1
        first, second = seen[0]
        assert isinstance(first, bytes)
        assert isinstance(second, bytes)
        assert len(first) == 32
        assert len(second) == 32

    # The decision goes only through compare_digest.
    monkeypatch.setattr(hmac, "compare_digest", lambda a, b: True)
    assert verify_signature(SECRET, BODY, [wrong]) is None

    monkeypatch.setattr(hmac, "compare_digest", lambda a, b: False)
    assert verify_signature(SECRET, BODY, [VALID]) == "mismatch"

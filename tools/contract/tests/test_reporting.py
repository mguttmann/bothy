# SPDX-License-Identifier: Apache-2.0
"""Tests of the reproduction command of G7."""

from __future__ import annotations

import shlex

import reporting


def test_curl_masks_credentials_and_keeps_the_request() -> None:
    cmd = reporting.curl_command(
        "post",
        "http://127.0.0.1:1/api/v1/spaces?x=1",
        {
            "Authorization": "DPoP secret-token",
            "DPoP": "eyJ.proof.sig",
            "Content-Type": "application/json",
            "Idempotency-Key": "0192f4a0-7b1c-7f89-8d6b-6960453241bc",
            "Content-Length": "13",
        },
        b'{"name":"it\'s"}',
        key_file="/tmp/key.json",
    )
    assert "secret-token" not in cmd
    assert "eyJ.proof.sig" not in cmd
    assert '"Authorization: ""$BOTHY_CONTRACT_AUTH"' in cmd
    assert (
        "python -m dpop --key /tmp/key.json --htm POST --htu 'http://127.0.0.1:1/api/v1/spaces?x=1'"
        in cmd
    )
    assert "-X POST 'http://127.0.0.1:1/api/v1/spaces?x=1'" in cmd
    assert "'Content-Type: application/json'" in cmd
    assert "'Idempotency-Key: 0192f4a0-7b1c-7f89-8d6b-6960453241bc'" in cmd
    assert "Content-Length" not in cmd
    tokens = shlex.split(cmd)
    assert tokens[-2:] == ["--data-binary", '{"name":"it\'s"}']


def test_curl_without_body() -> None:
    cmd = reporting.curl_command("GET", "http://127.0.0.1:1/api/v1/me", {}, None, key_file="k")
    assert cmd == "curl -sS -X GET http://127.0.0.1:1/api/v1/me"


def test_failure_report_names_the_seed() -> None:
    text = reporting.failure_report("bad", "curl x", 42)
    assert text.splitlines()[0] == "bad"
    assert "curl x" in text
    assert text.endswith("CONTRACT_SEED=42")

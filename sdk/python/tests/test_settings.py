# SPDX-License-Identifier: Apache-2.0
"""SdkSettings: one field per key, defaults equal to the configuration reference."""

from __future__ import annotations

import dataclasses
import re

import pytest
from conftest import config_defaults

from bothy_sdk.settings import SdkSettings, parse_duration


def test_defaults_match_configuration_reference() -> None:
    defaults = config_defaults()
    settings = SdkSettings()
    for f in dataclasses.fields(SdkSettings):
        key = f.metadata["key"]
        assert key in defaults, f"{key} is not in docs/operations/configuration.md"
        text = defaults[key]
        expected: float = int(text) if f.type in ("int", int) else parse_duration(text)
        assert getattr(settings, f.name) == expected, key


def test_from_mapping_parses_durations_and_rejects_unknown_keys() -> None:
    s = SdkSettings.from_mapping(
        {
            "sdk.retry_backoff_base": "250ms",
            "sync.reconnect_backoff_cap": "2m",
            "sdk.poll_interval": "1.5s",
        }
    )
    assert (s.retry_backoff_base, s.reconnect_backoff_cap, s.poll_interval) == (0.25, 120.0, 1.5)
    with pytest.raises(ValueError, match=r"unknown SDK setting: sdk\.nope"):
        SdkSettings.from_mapping({"sdk.nope": "1s"})
    with pytest.raises(ValueError, match=r"sdk\.retry_max_attempts"):
        SdkSettings.from_mapping({"sdk.retry_max_attempts": "many"})
    for key, text in (("sdk.poll_interval", "0s"), ("sdk.retry_max_attempts", "0")):
        with pytest.raises(ValueError, match=rf"invalid value for {re.escape(key)}"):
            SdkSettings.from_mapping({key: text})
    with pytest.raises(ValueError, match="not a duration"):
        parse_duration("10 parsecs")

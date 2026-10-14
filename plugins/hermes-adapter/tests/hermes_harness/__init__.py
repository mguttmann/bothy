# SPDX-License-Identifier: Apache-2.0
"""Recording harness for Hermes Agent (M2-32), reused by the adapter tests of M2-33.

The harness runs a pinned public Hermes Agent version in a throwaway directory with a fake
model and a probe platform plugin, and records how Hermes calls the plugin surfaces of
docs/spec/09-agenten.md section 17. Nothing it starts can reach a real model, a real
credential or another Hermes installation (docs/PLAN.md section 1 point 2).

Modules:
    pin: the pinned version (``tests/hermes_pin.toml``).
    provision: read-only checkout, install from an export into a throwaway venv.
    integrity: digests of the checkout and of the installed files.
    environment: throwaway homes and the allowlisted process environment.
    sandbox: the macOS sandbox profile of the Hermes process tree.
    egress: the refusing HTTP proxy that records attempted destinations.
    fake_model: the local fake model endpoint and the fake credentials.
    control: the control channel between the harness and the probe plugin.
    gateway: start and stop of the Hermes gateway process group.
    calls: normalization of recorded calls and the pattern comparison.
    observations: the facts and verdicts of the six points of 09 17.10.
    limits: the named deadlines of the harness.
    scenarios: loading and running the recording scenarios.
"""

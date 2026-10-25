# SPDX-License-Identifier: Apache-2.0
"""Reference app for ticket systems: Jira issues in a forum (docs/spec/11-extensions.md 10.2).

The app is its own process outside the server and uses only the public API through the Python
SDK (`bothy_sdk`). It receives Jira webhooks at ``POST /jira/webhook`` (checked with
``X-Hub-Signature`` before anything is parsed), posts the events ``jira:issue_created``,
``jira:issue_updated`` and ``comment_created`` into a forum, one thread per issue, creates
issues from the chat with ``/issue`` and a form, and, with the optional scopes, mirrors people's
replies to Jira as comments with loop and echo protection.

Modules: `jira_app.config` (configuration and secret files), `jira_app.app` (start and stop),
`jira_app.web` (HTTP surface), `jira_app.jira` (signature, events, ADF, REST client),
`jira_app.chat` (port to the server, SDK adapter, the three endpoints), `jira_app.domain`
(rendering, mirroring rules), `jira_app.store` (state and queue), `jira_app.worker` (jobs).

Examples:
    >>> from jira_app.jira.signature import SUPPORTED_METHODS
    >>> sorted(SUPPORTED_METHODS)
    ['sha256']
"""

from pathlib import Path

__all__ = ["APP_VERSION"]

APP_VERSION = (Path(__file__).resolve().parents[1] / "VERSION").read_text(encoding="utf-8").strip()
"""The version of the app, the ``VERSION`` file of `plugins/jira-app`."""

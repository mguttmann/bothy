# SPDX-License-Identifier: Apache-2.0
"""Rules of the app: rendering of Jira text in the chat and the mirroring decisions.

Examples:
    >>> from jira_app.domain.render import code_span
    >>> code_span("a`b")
    '``a`b``'
"""

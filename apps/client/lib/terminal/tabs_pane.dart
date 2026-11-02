// SPDX-License-Identifier: Apache-2.0

/// The terminal view of a host's tabs, the same on the desktop page and the phone screen M8
/// (FIX-terminal-session-lifecycle): the selected tab, the tab labels, "End" and "Close" of the
/// selected tab and the list of the own sessions.
library;

import 'dart:async';

import 'package:api_client/api_client.dart' show Host;
import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'providers.dart';

/// Builds the [TerminalPane] of [current], the selected tab of [tabs] on [host].
///
/// ```dart
/// tabsPane(key: const ValueKey('pane'), tabs: tabs, current: c, host: host, l: l)
/// ```
TerminalPane tabsPane({
  required Key key,
  required TerminalTabs tabs,
  required TerminalController current,
  required Host host,
  required AppLocalizations l,
  DateTime Function()? now,
  bool phone = false,
  HardwareKeyboardWatch? hardwareKeyboard,
  Map<ShortcutActivator, Intent> shortcuts = const {},
}) => TerminalPane(
  key: key,
  controller: current,
  fontSize: tabs.fontSize,
  tabs: [
    for (final (profile, n) in tabs.labels)
      SessionTabData(label: l.terminalTabLabel(profile, n)),
  ],
  selected: tabs.selected,
  onSelect: tabs.select,
  onEnd: () => unawaited(tabs.end(current)),
  onClose: () => tabs.close(current),
  sessions: tabs.entries(l.terminalTabLabel),
  maxSessionDuration: tabs.maxSessionDuration(host),
  now: now,
  phone: phone,
  showExtraKeys: phone,
  hardwareKeyboard: hardwareKeyboard,
  shortcuts: shortcuts,
  autofocus: true,
);

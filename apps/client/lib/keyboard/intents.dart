// SPDX-License-Identifier: Apache-2.0

/// The actions of the first shortcut table of docs/spec/01-product.md 7 and one [Intent] per
/// action (brief M4-24 Scope).
///
/// The set is closed on purpose: it holds no approval and no other component action, so no
/// binding can ever reach a component (R-PR-04). A component reacts only to Enter or Space on
/// its focused button, which Flutter's default activation does, not this layer.
library;

import 'package:flutter/widgets.dart';

/// One action of the shortcut table of 01 7.
enum ShortcutAction {
  /// Opens the quick switcher (01 D6).
  quickSwitcher,

  /// Opens the search (01 D7).
  search,

  /// Searches in the open thread.
  searchInThread,

  /// A new post or a new chat in the current forum.
  newPost,

  /// Puts the focus into the input of the open history.
  focusInput,

  /// Opens the next channel or thread row of the sidebar.
  nextChannel,

  /// Opens the previous channel or thread row of the sidebar.
  previousChannel,

  /// Opens the next channel or thread row with unread items.
  nextUnreadChannel,

  /// Opens the previous channel or thread row with unread items.
  previousUnreadChannel,

  /// Pins or unpins the forum overview as a column (01 D15, M4-40).
  pinOverview,

  /// Moves the focus to the next column (F6).
  nextColumn,

  /// Moves the focus to the previous column (Shift+F6).
  previousColumn,

  /// Marks the open thread read (Esc in the history).
  markRead,

  /// Marks a thread unread from a message (a pointer gesture on the message).
  markUnread,

  /// Marks the open thread as done or reopens it.
  toggleDone,

  /// Stops the run of the open thread.
  stopRun,

  /// Expands or collapses the live box of the open thread.
  toggleLiveBox,

  /// Shows or hides the session column.
  toggleSessionColumn,

  /// Hides or shows the channel column next to an open thread (01 D1, D2; M4-42).
  toggleChannelColumn,

  /// Edits the last own message (arrow up in the empty input; the composer handles it).
  editLast,

  /// Sends the input (the composer handles it).
  send,

  /// A new line in the input (the composer handles it).
  newLine,

  /// Opens "Running now" (01 D10).
  runningNow,

  /// Opens the mentions.
  mentions,

  /// Opens the settings.
  settings,

  /// Shows the list of shortcuts.
  shortcutList,

  /// Opens hosts and the terminal (`/hosts`, M4-36).
  hostsTerminal,
}

/// The intent of one [ShortcutAction]; every subclass names exactly one action.
///
/// Example: `Actions.invoke(context, const QuickSwitcherIntent())`.
sealed class AppIntent extends Intent {
  const AppIntent();

  /// The action this intent stands for.
  ShortcutAction get action;
}

/// Intent of [ShortcutAction.quickSwitcher].
final class QuickSwitcherIntent extends AppIntent {
  /// Creates the intent.
  const QuickSwitcherIntent();

  @override
  ShortcutAction get action => ShortcutAction.quickSwitcher;
}

/// Intent of [ShortcutAction.search].
final class SearchIntent extends AppIntent {
  /// Creates the intent.
  const SearchIntent();

  @override
  ShortcutAction get action => ShortcutAction.search;
}

/// Intent of [ShortcutAction.searchInThread].
final class SearchInThreadIntent extends AppIntent {
  /// Creates the intent.
  const SearchInThreadIntent();

  @override
  ShortcutAction get action => ShortcutAction.searchInThread;
}

/// Intent of [ShortcutAction.newPost].
final class NewPostIntent extends AppIntent {
  /// Creates the intent.
  const NewPostIntent();

  @override
  ShortcutAction get action => ShortcutAction.newPost;
}

/// Intent of [ShortcutAction.focusInput].
final class FocusInputIntent extends AppIntent {
  /// Creates the intent.
  const FocusInputIntent();

  @override
  ShortcutAction get action => ShortcutAction.focusInput;
}

/// Intent of [ShortcutAction.nextChannel].
final class NextChannelIntent extends AppIntent {
  /// Creates the intent.
  const NextChannelIntent();

  @override
  ShortcutAction get action => ShortcutAction.nextChannel;
}

/// Intent of [ShortcutAction.previousChannel].
final class PreviousChannelIntent extends AppIntent {
  /// Creates the intent.
  const PreviousChannelIntent();

  @override
  ShortcutAction get action => ShortcutAction.previousChannel;
}

/// Intent of [ShortcutAction.nextUnreadChannel].
final class NextUnreadChannelIntent extends AppIntent {
  /// Creates the intent.
  const NextUnreadChannelIntent();

  @override
  ShortcutAction get action => ShortcutAction.nextUnreadChannel;
}

/// Intent of [ShortcutAction.previousUnreadChannel].
final class PreviousUnreadChannelIntent extends AppIntent {
  /// Creates the intent.
  const PreviousUnreadChannelIntent();

  @override
  ShortcutAction get action => ShortcutAction.previousUnreadChannel;
}

/// Intent of [ShortcutAction.pinOverview].
final class PinOverviewIntent extends AppIntent {
  /// Creates the intent.
  const PinOverviewIntent();

  @override
  ShortcutAction get action => ShortcutAction.pinOverview;
}

/// Intent of [ShortcutAction.nextColumn].
final class NextColumnIntent extends AppIntent {
  /// Creates the intent.
  const NextColumnIntent();

  @override
  ShortcutAction get action => ShortcutAction.nextColumn;
}

/// Intent of [ShortcutAction.previousColumn].
final class PreviousColumnIntent extends AppIntent {
  /// Creates the intent.
  const PreviousColumnIntent();

  @override
  ShortcutAction get action => ShortcutAction.previousColumn;
}

/// Intent of [ShortcutAction.markRead].
final class MarkReadIntent extends AppIntent {
  /// Creates the intent.
  const MarkReadIntent();

  @override
  ShortcutAction get action => ShortcutAction.markRead;
}

/// Intent of [ShortcutAction.markUnread].
final class MarkUnreadIntent extends AppIntent {
  /// Creates the intent.
  const MarkUnreadIntent();

  @override
  ShortcutAction get action => ShortcutAction.markUnread;
}

/// Intent of [ShortcutAction.toggleDone].
final class ToggleDoneIntent extends AppIntent {
  /// Creates the intent.
  const ToggleDoneIntent();

  @override
  ShortcutAction get action => ShortcutAction.toggleDone;
}

/// Intent of [ShortcutAction.stopRun].
final class StopRunIntent extends AppIntent {
  /// Creates the intent.
  const StopRunIntent();

  @override
  ShortcutAction get action => ShortcutAction.stopRun;
}

/// Intent of [ShortcutAction.toggleLiveBox].
final class ToggleLiveBoxIntent extends AppIntent {
  /// Creates the intent.
  const ToggleLiveBoxIntent();

  @override
  ShortcutAction get action => ShortcutAction.toggleLiveBox;
}

/// Intent of [ShortcutAction.toggleSessionColumn].
final class ToggleSessionColumnIntent extends AppIntent {
  /// Creates the intent.
  const ToggleSessionColumnIntent();

  @override
  ShortcutAction get action => ShortcutAction.toggleSessionColumn;
}

/// Intent of [ShortcutAction.toggleChannelColumn].
final class ToggleChannelColumnIntent extends AppIntent {
  /// Creates the intent.
  const ToggleChannelColumnIntent();

  @override
  ShortcutAction get action => ShortcutAction.toggleChannelColumn;
}

/// Intent of [ShortcutAction.editLast].
final class EditLastIntent extends AppIntent {
  /// Creates the intent.
  const EditLastIntent();

  @override
  ShortcutAction get action => ShortcutAction.editLast;
}

/// Intent of [ShortcutAction.send].
final class SendIntent extends AppIntent {
  /// Creates the intent.
  const SendIntent();

  @override
  ShortcutAction get action => ShortcutAction.send;
}

/// Intent of [ShortcutAction.newLine].
final class NewLineIntent extends AppIntent {
  /// Creates the intent.
  const NewLineIntent();

  @override
  ShortcutAction get action => ShortcutAction.newLine;
}

/// Intent of [ShortcutAction.runningNow].
final class RunningNowIntent extends AppIntent {
  /// Creates the intent.
  const RunningNowIntent();

  @override
  ShortcutAction get action => ShortcutAction.runningNow;
}

/// Intent of [ShortcutAction.mentions].
final class MentionsIntent extends AppIntent {
  /// Creates the intent.
  const MentionsIntent();

  @override
  ShortcutAction get action => ShortcutAction.mentions;
}

/// Intent of [ShortcutAction.settings].
final class SettingsIntent extends AppIntent {
  /// Creates the intent.
  const SettingsIntent();

  @override
  ShortcutAction get action => ShortcutAction.settings;
}

/// Intent of [ShortcutAction.shortcutList].
final class ShortcutListIntent extends AppIntent {
  /// Creates the intent.
  const ShortcutListIntent();

  @override
  ShortcutAction get action => ShortcutAction.shortcutList;
}

/// Intent of [ShortcutAction.hostsTerminal].
final class HostsTerminalIntent extends AppIntent {
  /// Creates the intent.
  const HostsTerminalIntent();

  @override
  ShortcutAction get action => ShortcutAction.hostsTerminal;
}

/// The intent of [action].
///
/// Example: `intentOf(ShortcutAction.search)` is a [SearchIntent].
AppIntent intentOf(ShortcutAction action) => switch (action) {
  ShortcutAction.quickSwitcher => const QuickSwitcherIntent(),
  ShortcutAction.search => const SearchIntent(),
  ShortcutAction.searchInThread => const SearchInThreadIntent(),
  ShortcutAction.newPost => const NewPostIntent(),
  ShortcutAction.focusInput => const FocusInputIntent(),
  ShortcutAction.nextChannel => const NextChannelIntent(),
  ShortcutAction.previousChannel => const PreviousChannelIntent(),
  ShortcutAction.nextUnreadChannel => const NextUnreadChannelIntent(),
  ShortcutAction.previousUnreadChannel => const PreviousUnreadChannelIntent(),
  ShortcutAction.pinOverview => const PinOverviewIntent(),
  ShortcutAction.nextColumn => const NextColumnIntent(),
  ShortcutAction.previousColumn => const PreviousColumnIntent(),
  ShortcutAction.markRead => const MarkReadIntent(),
  ShortcutAction.markUnread => const MarkUnreadIntent(),
  ShortcutAction.toggleDone => const ToggleDoneIntent(),
  ShortcutAction.stopRun => const StopRunIntent(),
  ShortcutAction.toggleLiveBox => const ToggleLiveBoxIntent(),
  ShortcutAction.toggleSessionColumn => const ToggleSessionColumnIntent(),
  ShortcutAction.toggleChannelColumn => const ToggleChannelColumnIntent(),
  ShortcutAction.editLast => const EditLastIntent(),
  ShortcutAction.send => const SendIntent(),
  ShortcutAction.newLine => const NewLineIntent(),
  ShortcutAction.runningNow => const RunningNowIntent(),
  ShortcutAction.mentions => const MentionsIntent(),
  ShortcutAction.settings => const SettingsIntent(),
  ShortcutAction.shortcutList => const ShortcutListIntent(),
  ShortcutAction.hostsTerminal => const HostsTerminalIntent(),
};

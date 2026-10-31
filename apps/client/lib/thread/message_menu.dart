// SPDX-License-Identifier: Apache-2.0

/// The actions of a message (brief M4-20 Design note 4; 01 3.3, 6.7, 7): which the server lets
/// the person use, and the menu that offers them on hover, right click and by keyboard
/// (focus a message, then the menu key or Shift+F10).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'providers.dart';

/// An action of a message.
enum MessageAction {
  /// Reply with a quote.
  reply,

  /// Start a thread from a message of the main history of a text channel.
  startThread,

  /// Add or remove a reaction.
  react,

  /// Pin the message.
  pin,

  /// Unpin the message.
  unpin,

  /// Edit the own message.
  edit,

  /// Show the earlier versions of an edited message.
  versions,

  /// Delete the message (one confirmation).
  delete,

  /// Move the read position before the message.
  markUnread,

  /// Copy the text.
  copyText,

  /// Copy the link `<scheme>://t/<thread>?m=<message>`.
  copyLink,

  /// Copy the text selected in the message (the desktop, 01 6.15 item 2); offered by the tile
  /// only while text is selected, and run by the tile itself.
  copySelection,

  /// Open the message in a view with selectable text (the phone, 01 6.15 item 5); offered and
  /// run by the tile itself.
  selectText,
}

/// The actions of [message] for the reader [me] with [permissions] (05 action table:
/// `message.create`, `reaction.add` and `thread.create` need write, `message.update` write and
/// the author, `message.delete` write and the author or `manage`, `pin.*` `manage`,
/// `message.revisions.read` read; docs/DECISIONS.md D-M4-20-4). [mainHistory] offers "Start
/// thread" (01 3.1); [startedThread] hides it when the message started one already. A deleted
/// message and a system line have no actions.
List<MessageAction> messageActions(
  Message message, {
  required PrincipalId? me,
  required ThreadPermissions permissions,
  required bool mainHistory,
  bool startedThread = false,
}) {
  if (message.deletedAt != null || message.kind == MessageKind.system) {
    return const [];
  }
  final own = me != null && message.authorId == me;
  final write = permissions.canWrite;
  return [
    if (write) MessageAction.reply,
    if (write && mainHistory && !startedThread) MessageAction.startThread,
    if (write) MessageAction.react,
    if (permissions.canManage && !message.pinned) MessageAction.pin,
    if (permissions.canManage && message.pinned) MessageAction.unpin,
    if (own && write && message.kind == MessageKind.text) MessageAction.edit,
    if (message.editedAt != null) MessageAction.versions,
    if ((own && write) || permissions.canManage) MessageAction.delete,
    MessageAction.markUnread,
    MessageAction.copyText,
    MessageAction.copyLink,
  ];
}

/// The label of [action].
String messageActionLabel(AppLocalizations l, MessageAction action) =>
    switch (action) {
      MessageAction.reply => l.messageActionReply,
      MessageAction.startThread => l.messageActionStartThread,
      MessageAction.react => l.messageActionReact,
      MessageAction.pin => l.messageActionPin,
      MessageAction.unpin => l.messageActionUnpin,
      MessageAction.edit => l.messageEdit,
      MessageAction.versions => l.messageActionVersions,
      MessageAction.delete => l.messageActionDelete,
      MessageAction.markUnread => l.messageActionMarkUnread,
      MessageAction.copyText => l.messageActionCopyText,
      MessageAction.copyLink => l.messageActionCopyLink,
      MessageAction.copySelection => l.messageActionCopySelection,
      MessageAction.selectText => l.messageActionSelectText,
    };

/// The icon of [action] on the hover bar, or null for an action only in the menu.
DsGlyph? messageActionGlyph(MessageAction action) => switch (action) {
  MessageAction.reply => DsIcons.actionReply,
  MessageAction.react => DsIcons.actionReact,
  _ => null,
};

/// Opens the menu with [actions] at [position] (global) over [context]; returns the chosen
/// action, or null.
///
/// Example: `final action = await showMessageMenu(context, at, actions);`
Future<MessageAction?> showMessageMenu(
  BuildContext context,
  Offset position,
  List<MessageAction> actions,
) {
  final l = AppLocalizations.of(context);
  final overlay = Overlay.of(context).context.findRenderObject()! as RenderBox;
  return showMenu<MessageAction>(
    context: context,
    position: RelativeRect.fromRect(
      position & const Size(1, 1),
      Offset.zero & overlay.size,
    ),
    semanticLabel: l.messageMenuLabel,
    items: [
      for (final a in actions)
        PopupMenuItem<MessageAction>(
          key: ValueKey('action.${a.name}'),
          value: a,
          child: Text(messageActionLabel(l, a)),
        ),
    ],
  );
}

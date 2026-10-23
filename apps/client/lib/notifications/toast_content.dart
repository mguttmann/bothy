// SPDX-License-Identifier: Apache-2.0

/// The content of a local notification, mapped from the payload of `notification.created`
/// (docs/spec/01-product.md 10.2; docs/spec/13-push-notifications.md 6.3 and 6.4; brief
/// M4-30).
library;

import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

/// Title and text of one local notification.
///
/// [toString] names neither, so a log line never carries content (13 10).
///
/// Example: `presenter.show(toastContentFor(n, texts: l, appName: name), ...)`.
final class ToastContent {
  /// Creates the content from [title] and [body].
  const ToastContent({required this.title, required this.body});

  /// The first line, for example `general · Release notes`.
  final String title;

  /// The text below the title, for example `Sam: Build is green`; a bundled notification has
  /// the unread count as its first line.
  final String body;

  @override
  String toString() => 'ToastContent(<content>)';
}

/// Maps [n] to the content of its local notification (01 10.2).
///
/// With a thread: the title is `<Channel> · <Thread title>` from the preview fields, only the
/// channel when the thread title is missing, and [appName] when no channel name is known; the
/// channel name comes from the payload or, without preview fields, from [localChannelName] (the
/// name the local database holds). The text is `<Author>: <excerpt>`; without an excerpt
/// (`notify.preview_mode = names`) the excerpt is the placeholder, without an author the text
/// is the placeholder alone. A `count` above 1 (13 4.5) puts the unread count as the first line.
/// `security` and `admin` carry no thread and get their own texts (13 2). [texts] are the
/// interface texts of the current language.
///
/// Example:
/// ```dart
/// final c = toastContentFor(n, texts: l, appName: 'ExampleChat', localChannelName: 'general');
/// ```
ToastContent toastContentFor(
  NotificationCreated n, {
  required AppLocalizations texts,
  required String appName,
  String? localChannelName,
}) {
  switch (n.kind) {
    case HintKind.security:
      return ToastContent(
        title: texts.notificationSecurityTitle,
        body: texts.notificationSecurityText,
      );
    case HintKind.admin:
      return ToastContent(
        title: texts.notificationAdminTitle,
        body: texts.notificationAdminText,
      );
    case _:
      break;
  }
  final channel = _present(n.channelName) ?? _present(localChannelName);
  final thread = _present(n.threadTitle);
  final title = switch ((channel, thread)) {
    (final String c, final String t) => texts.notificationTitle(c, t),
    (final String c, null) => c,
    _ => appName,
  };
  final author = _present(n.senderDisplayName);
  final excerpt = _present(n.excerpt);
  final text = author == null
      ? texts.notificationPlaceholder
      : texts.notificationText(
          author,
          excerpt ?? texts.notificationPlaceholder,
        );
  final body = n.count > 1
      ? '${texts.notificationBundled(n.count)}\n$text'
      : text;
  return ToastContent(title: title, body: body);
}

String? _present(String? value) =>
    value == null || value.trim().isEmpty ? null : value;

// SPDX-License-Identifier: Apache-2.0

// The content of a local notification (brief M4-30 Tests; 01 10.2, 13 6.3 and 6.4): title and
// text from the preview fields, the placeholder with the local channel name without them, the
// unread count of a bundled notification, the kinds without a thread, and no content in logs.

import 'package:client/notifications/toast_content.dart';
import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

final AppLocalizations en = lookupAppLocalizations(const Locale('en'));

const String thread = '01890a5d-ac96-774b-bcce-b302099a8057';
const String message = '01890a5d-ac96-774b-bcce-b302099a8058';

NotificationCreated hint({
  HintKind kind = HintKind.message,
  int count = 1,
  String? channelName,
  String? threadTitle,
  String? sender,
  String? excerpt,
  bool withThread = true,
}) => NotificationCreated(
  nid: '01890a5d-ac96-774b-bcce-b302099a8059',
  kind: kind,
  threadId: withThread ? ThreadId.parse(thread) : null,
  messageId: withThread ? MessageId.parse(message) : null,
  count: count,
  channelName: channelName,
  threadTitle: threadTitle,
  senderDisplayName: sender,
  excerpt: excerpt,
);

void main() {
  test('title and text from the preview fields (mode full)', () {
    final c = toastContentFor(
      hint(
        channelName: 'general',
        threadTitle: 'Release notes',
        sender: 'Sam',
        excerpt: 'Build is green',
      ),
      texts: en,
      appName: 'ExampleChat',
      localChannelName: 'ignored',
    );
    expect(c.title, 'general · Release notes');
    expect(c.body, 'Sam: Build is green');
  });

  test('names without excerpt (mode names) keep the author', () {
    final c = toastContentFor(
      hint(channelName: 'general', threadTitle: 'Release notes', sender: 'Sam'),
      texts: en,
      appName: 'ExampleChat',
    );
    expect(c.title, 'general · Release notes');
    expect(c.body, 'Sam: New message');
  });

  test('without preview fields: the placeholder with the local channel', () {
    final c = toastContentFor(
      hint(),
      texts: en,
      appName: 'ExampleChat',
      localChannelName: 'general',
    );
    expect(c.title, 'general');
    expect(c.body, 'New message');
  });

  test('without preview fields and without a local channel: the app name', () {
    final c = toastContentFor(hint(), texts: en, appName: 'ExampleChat');
    expect(c.title, 'ExampleChat');
    expect(c.body, 'New message');
  });

  test('a bundled notification names the unread count', () {
    final c = toastContentFor(
      hint(count: 3, sender: 'Sam', excerpt: 'Third', channelName: 'general'),
      texts: en,
      appName: 'ExampleChat',
    );
    expect(c.body, '3 new messages\nSam: Third');
  });

  test('security and admin have their own texts and no thread', () {
    final s = toastContentFor(
      hint(kind: HintKind.security, withThread: false),
      texts: en,
      appName: 'ExampleChat',
    );
    expect(s.title, en.notificationSecurityTitle);
    expect(s.body, en.notificationSecurityText);
    final a = toastContentFor(
      hint(kind: HintKind.admin, withThread: false),
      texts: en,
      appName: 'ExampleChat',
    );
    expect(a.title, en.notificationAdminTitle);
    expect(a.body, en.notificationAdminText);
  });

  test('no content in logs: toString names no title and no text', () {
    final c = toastContentFor(
      hint(sender: 'Sam', excerpt: 'Build is green', channelName: 'general'),
      texts: en,
      appName: 'ExampleChat',
    );
    expect(c.toString(), isNot(contains('Sam')));
    expect(c.toString(), isNot(contains('Build')));
    expect(c.toString(), isNot(contains('general')));
  });
}

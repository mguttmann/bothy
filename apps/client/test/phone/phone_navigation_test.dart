// SPDX-License-Identifier: Apache-2.0

// The tabs and stacks of the phone (brief M5-02 Design note 2; 01 5.3; 15 11): which tab a path
// belongs to, and which pages a tab shows below it.

import 'package:client/app/router.dart';
import 'package:client/phone/phone_navigation.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

const String space = '0193f2a7-0000-7000-8000-0000000000a1';
const String forum = '0193f2a7-0000-7000-8000-0000000000c1';
const String thread = '0193f2a7-0000-7000-8000-0000000000d1';
const String mainThread = '0193f2a7-0000-7000-8000-0000000000d2';
const String message = '0193f2a7-0000-7000-8000-0000000000e1';

AppLocation at(String path) => parseLocation(Uri.parse(path));

void main() {
  test('a root path selects its tab', () {
    for (final current in PhoneTab.values) {
      expect(tabOf(at('/'), current), PhoneTab.spaces);
      expect(tabOf(at('/mentions'), current), PhoneTab.mentions);
      expect(tabOf(at('/running'), current), PhoneTab.running);
      // The run graph of a card (M12) belongs to Running.
      expect(
        tabOf(at('/running/$message?t=$thread'), current),
        PhoneTab.running,
      );
      expect(tabOf(at('/settings'), current), PhoneTab.me);
    }
  });

  test('a thread stays in Mentions and Running, else it opens in Spaces', () {
    expect(tabOf(at('/t/$thread'), PhoneTab.mentions), PhoneTab.mentions);
    expect(tabOf(at('/t/$thread'), PhoneTab.running), PhoneTab.running);
    expect(tabOf(at('/t/$thread'), PhoneTab.spaces), PhoneTab.spaces);
    expect(tabOf(at('/t/$thread'), PhoneTab.me), PhoneTab.spaces);
  });

  test('a link from outside always opens in Spaces', () {
    for (final current in PhoneTab.values) {
      expect(
        tabOf(at('/t/$thread?m=$message'), current, external: true),
        PhoneTab.spaces,
      );
    }
  });

  test('channels, spaces and search belong to Spaces, settings to Me', () {
    expect(tabOf(at('/c/$forum'), PhoneTab.mentions), PhoneTab.spaces);
    expect(tabOf(at('/s/$space'), PhoneTab.running), PhoneTab.spaces);
    expect(tabOf(at('/search?q=x'), PhoneTab.me), PhoneTab.spaces);
    expect(tabOf(at('/settings/devices'), PhoneTab.spaces), PhoneTab.me);
    expect(tabOf(at('/admin/people'), PhoneTab.spaces), PhoneTab.me);
    expect(tabOf(at('/nothing'), PhoneTab.running), PhoneTab.running);
  });

  test('a thread in Spaces stacks spaces, channel and thread', () {
    final pages = phoneStack(
      PhoneTab.spaces,
      Uri.parse('/t/$thread?m=$message'),
      channelOf: (channel: ChannelId.parse(forum), mainThread: null),
    );
    expect(pages, const [
      PhonePage(PhonePageKind.spaces, '/'),
      PhonePage(PhonePageKind.channel, '/c/$forum'),
      PhonePage(PhonePageKind.thread, '/t/$thread?m=$message'),
    ]);
  });

  test('before its channel is stored a thread stacks on Spaces alone', () {
    expect(phoneStack(PhoneTab.spaces, Uri.parse('/t/$thread')), const [
      PhonePage(PhonePageKind.spaces, '/'),
      PhonePage(PhonePageKind.thread, '/t/$thread'),
    ]);
  });

  test('the main thread of a text channel is not shown twice', () {
    final pages = phoneStack(
      PhoneTab.spaces,
      Uri.parse('/t/$mainThread'),
      channelOf: (
        channel: ChannelId.parse(forum),
        mainThread: ThreadId.parse(mainThread),
      ),
    );
    expect(pages.map((p) => p.kind), [
      PhonePageKind.spaces,
      PhonePageKind.thread,
    ]);
  });

  test('a thread in Mentions or Running stacks on the tab root', () {
    expect(phoneStack(PhoneTab.mentions, Uri.parse('/t/$thread')), const [
      PhonePage(PhonePageKind.mentions, '/mentions'),
      PhonePage(PhonePageKind.thread, '/t/$thread'),
    ]);
    expect(phoneStack(PhoneTab.running, Uri.parse('/t/$thread')), const [
      PhonePage(PhonePageKind.running, '/running'),
      PhonePage(PhonePageKind.thread, '/t/$thread'),
    ]);
  });

  test('channels, search, settings and unknown paths', () {
    expect(phoneStack(PhoneTab.spaces, Uri.parse('/')), const [
      PhonePage(PhonePageKind.spaces, '/'),
    ]);
    expect(phoneStack(PhoneTab.spaces, Uri.parse('/s/$space')), const [
      PhonePage(PhonePageKind.spaces, '/'),
    ]);
    expect(phoneStack(PhoneTab.spaces, Uri.parse('/c/$forum')), const [
      PhonePage(PhonePageKind.spaces, '/'),
      PhonePage(PhonePageKind.channel, '/c/$forum'),
    ]);
    expect(phoneStack(PhoneTab.spaces, Uri.parse('/search?q=x')), const [
      PhonePage(PhonePageKind.spaces, '/'),
      PhonePage(PhonePageKind.search, '/search?q=x'),
    ]);
    expect(phoneStack(PhoneTab.me, Uri.parse('/settings')), const [
      PhonePage(PhonePageKind.me, '/settings'),
    ]);
    expect(phoneStack(PhoneTab.me, Uri.parse('/settings/devices')), const [
      PhonePage(PhonePageKind.me, '/settings'),
      PhonePage(PhonePageKind.settingsSection, '/settings/devices'),
    ]);
    // The administration is desktop and web console only (01 4).
    expect(phoneStack(PhoneTab.me, Uri.parse('/admin/people')), const [
      PhonePage(PhonePageKind.me, '/settings'),
      PhonePage(PhonePageKind.notFound, '/admin/people'),
    ]);
    expect(phoneStack(PhoneTab.spaces, Uri.parse('/t/not-an-id')), const [
      PhonePage(PhonePageKind.spaces, '/'),
      PhonePage(PhonePageKind.notFound, '/t/not-an-id'),
    ]);
  });

  test('the run graph M12: on Running from a card, on top of the thread '
      'from the live box', () {
    final graph = '/running/$message?t=$thread';
    final fromThread = '$graph&from=thread';
    expect(phoneStack(PhoneTab.running, Uri.parse(graph)), [
      const PhonePage(PhonePageKind.running, '/running'),
      PhonePage(PhonePageKind.runGraph, graph),
    ]);
    for (final current in [PhoneTab.spaces, PhoneTab.me]) {
      expect(tabOf(at(fromThread), current), PhoneTab.spaces);
    }
    for (final current in [PhoneTab.mentions, PhoneTab.running]) {
      expect(tabOf(at(fromThread), current), current);
    }
    expect(
      tabOf(at(fromThread), PhoneTab.running, external: true),
      PhoneTab.spaces,
    );
    final pages = phoneStack(
      PhoneTab.spaces,
      Uri.parse(fromThread),
      channelOf: (channel: ChannelId.parse(forum), mainThread: null),
    );
    expect(pages, [
      const PhonePage(PhonePageKind.spaces, '/'),
      const PhonePage(PhonePageKind.channel, '/c/$forum'),
      const PhonePage(PhonePageKind.thread, '/t/$thread'),
      PhonePage(PhonePageKind.runGraph, fromThread),
    ]);
    expect(phoneStack(PhoneTab.mentions, Uri.parse(fromThread)), [
      const PhonePage(PhonePageKind.mentions, '/mentions'),
      const PhonePage(PhonePageKind.thread, '/t/$thread'),
      PhonePage(PhonePageKind.runGraph, fromThread),
    ]);
  });

  test('Live M10 sits under Me (brief M6-16)', () {
    for (final current in PhoneTab.values) {
      expect(tabOf(at('/live'), current), PhoneTab.me);
    }
    expect(phoneStack(PhoneTab.me, Uri.parse('/live')), const [
      PhonePage(PhonePageKind.me, '/settings'),
      PhonePage(PhonePageKind.live, '/live'),
    ]);
  });

  test('the status page belongs to Me, on top of the Me page (M7-21)', () {
    final status = Uri.parse('/status');
    expect(tabOf(parseLocation(status), PhoneTab.spaces), PhoneTab.me);
    expect(phoneStack(PhoneTab.me, status), [
      const PhonePage(PhonePageKind.me, '/settings'),
      const PhonePage(PhonePageKind.status, '/status'),
    ]);
    // A path below it names nothing.
    expect(parseLocation(Uri.parse('/status/x')), isA<UnknownLocation>());
  });
}

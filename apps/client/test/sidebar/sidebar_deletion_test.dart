// SPDX-License-Identifier: Apache-2.0

// The sidebar follows deletions and their undo from the live stream (01 6.18 item 8, D-DEL-5,
// D-DEL-8): the fake server deletes a chat (`thread.updated` with `deleted_at`), a channel
// (`channel.archived` with it) and a space (`space.archived` with it) through the engine's real
// appliers, and each row leaves the sidebar tree at once; the undo brings it back in its
// place. The second test deletes from this device through the app's own port (`deleteThread`,
// `deleteChannel` with the step-up, their restores) against the same fake. Headless against
// the fake server through the app's providers.

import 'dart:io';

import 'package:auth/auth.dart' show StepUpCredentials, StepUpPrompt;
import 'package:client/app/providers/ports.dart';
import 'package:client/delete/delete_port.dart';
import 'package:client/delete/providers.dart';
import 'package:client/sidebar/providers.dart';
import 'package:client/sidebar/sidebar_model.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import '../support/app_scenario_client.dart';
import '../support/fake_instance.dart';
import '../support/fakes.dart';

/// Confirms every step-up with the device key and records the purposes.
class _Prompt implements StepUpPrompt {
  final List<String> asked = [];

  @override
  Future<bool> ask(String purposeText) async {
    asked.add(purposeText);
    return true;
  }

  @override
  Future<StepUpCredentials?> askPassword(
    String purposeText, {
    bool rejected = false,
  }) async => null;
}

/// The sidebar tree of [c] with lookups by ID.
final class _Tree {
  _Tree(ProviderContainer c, this.spaceId)
    : _sub = c.listen(sidebarTreeProvider, (_, _) {});

  final ProviderSubscription<List<SidebarSpaceNode>> _sub;
  final String spaceId;

  SidebarSpaceNode? get space =>
      _sub.read().where((n) => n.entry.space.id.value == spaceId).firstOrNull;

  List<String> get channels => [
    for (final ch in space?.channels ?? const <SidebarChannel>[])
      ch.channel.id.value,
  ];

  SidebarChannel? channel(String id) =>
      space?.channels.where((ch) => ch.channel.id.value == id).firstOrNull;

  bool chatShown(String channelId, String threadId) =>
      (channel(channelId)?.group?.rows ?? const []).any(
        (r) => r.thread.id.value == threadId,
      );

  void close() => _sub.close();
}

void main() {
  test('deleted chat, channel and space rows leave the sidebar and come back '
      'on undo', () async {
    final saved = HttpOverrides.current;
    HttpOverrides.global = null;
    final f = await FakeInstance.start();
    final c = ProviderContainer(
      overrides: [appPortsProvider.overrideWithValue(f.ports)],
    );
    final client = AppScenarioClient(
      c,
      f.server,
      f.ports.network as SwitchNetwork,
    );
    final w = f.seed;
    final world = f.server.world;
    try {
      await client.start();
      final t = _Tree(c, w.space);
      SidebarSpaceNode? team() => t.space;
      List<String> channels() => t.channels;
      bool chatShown(String channelId, String threadId) =>
          t.chatShown(channelId, threadId);

      await eventually(
        'the tree with both channels and the printer chat',
        () =>
            channels().contains(w.general) &&
            channels().contains(w.support) &&
            chatShown(w.support, w.printer),
      );
      final order = channels();

      // A chat: thread.updated with deleted_at, then without it.
      world.deleteThread(w.sam.id, w.printer);
      await eventually(
        'the chat row gone',
        () => !chatShown(w.support, w.printer),
      );
      expect(channels(), order, reason: 'the channels stay');
      world.restoreThread(w.sam.id, w.printer);
      await eventually(
        'the chat row back',
        () => chatShown(w.support, w.printer),
      );

      // A channel: channel.archived with deleted_at, then channel.updated without it.
      world.grantCapability(w.alex.id, w.support, 'manage');
      world.deleteChannel(w.alex.id, w.support);
      await eventually(
        'the channel row gone',
        () => !channels().contains(w.support),
      );
      expect(channels(), contains(w.general));
      world.restoreChannel(w.alex.id, w.support);
      await eventually(
        'the channel row back in its place with its chat',
        () => channels().contains(w.support) && chatShown(w.support, w.printer),
      );
      expect(channels(), order);

      // A space: space.archived with deleted_at removes it with its channels.
      world.deleteSpace(w.alex.id, w.space);
      await eventually('the space gone', () => team() == null);
      world.restoreSpace(w.alex.id, w.space);
      await eventually(
        'the space back with its channels',
        () => channels().contains(w.general) && channels().contains(w.support),
      );
      expect(channels(), order);
      expect(f.server.violations, isEmpty);
      t.close();
    } finally {
      await client.stop();
      c.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    }
  });

  test('a delete from this device through the port removes the rows and its '
      'undo brings them back', () async {
    final saved = HttpOverrides.current;
    HttpOverrides.global = null;
    final prompt = _Prompt();
    final f = await FakeInstance.start(stepUpPrompt: prompt);
    final c = ProviderContainer(
      overrides: [appPortsProvider.overrideWithValue(f.ports)],
    );
    final client = AppScenarioClient(
      c,
      f.server,
      f.ports.network as SwitchNetwork,
    );
    final w = f.seed;
    try {
      await client.start();
      final t = _Tree(c, w.space);
      await eventually(
        'the tree with the release notes chat',
        () => t.chatShown(w.general, w.releaseNotes),
      );
      final order = t.channels;
      final port = c.read(deletePortProvider)!;

      // Alex created the chat: no step-up (D-DEL-4).
      final chat = DeleteTarget(
        kind: DeleteKind.chat,
        id: w.releaseNotes,
        name: 'Release notes',
      );
      final before = await port.info(chat);
      expect(before.undoUntil, isNull, reason: 'a live item has no window');
      final deleted = await port.delete(
        chat,
        trashFolder: false,
        idempotencyKey: port.newKey(),
        purpose: 'Delete Release notes',
      );
      expect(deleted.undoUntil, isNotNull);
      await eventually(
        'the chat row gone',
        () => !t.chatShown(w.general, w.releaseNotes),
      );
      await port.restore(chat, idempotencyKey: port.newKey(), purpose: 'Undo');
      await eventually(
        'the chat row back',
        () => t.chatShown(w.general, w.releaseNotes),
      );
      expect(prompt.asked, isEmpty);

      // A channel with `manage`: the step-up, `trash_project_folder` false.
      f.server.world.grantCapability(w.alex.id, w.support, 'manage');
      final support = DeleteTarget(
        kind: DeleteKind.channel,
        id: w.support,
        name: 'support',
      );
      final gone = await port.delete(
        support,
        trashFolder: false,
        idempotencyKey: port.newKey(),
        purpose: 'Delete support',
      );
      expect(gone.undoUntil, isNotNull);
      await eventually(
        'the channel row gone',
        () => !t.channels.contains(w.support),
      );
      await port.restore(
        support,
        idempotencyKey: port.newKey(),
        purpose: 'Undo support',
      );
      await eventually(
        'the channel row back in its place',
        () => t.channels.contains(w.support),
      );
      expect(t.channels, order);
      // The undo right after the delete reuses the recent step-up of the session (05 4.5).
      expect(prompt.asked, ['Delete support']);
      expect(f.server.violations, isEmpty);
      t.close();
    } finally {
      await client.stop();
      c.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    }
  });
}

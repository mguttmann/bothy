// SPDX-License-Identifier: Apache-2.0

// The example of 07 9.3 headless, through the app's REST source against the fake server
// (brief M4-27 Tests and Design note 3): the preview answers Sam's request with the text of
// the specification, read at run time (D-W-94), in `de` with hint H1; after the grant "Who
// has access?" lists Sam with the direct source on the channel and via the agent on the
// channel the agent reads. Every request and response matches docs/api/.

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:auth/auth.dart';
import 'package:client/admin/admin_route.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';

/// Answers every step-up with yes.
class _Yes implements StepUpPrompt {
  @override
  Future<bool> ask(String purposeText) async => true;

  /// These tests confirm with the step-up key; a password prompt cancels.
  @override
  Future<StepUpCredentials?> askPassword(
    String purposeText, {
    bool rejected = false,
  }) async => null;
}

void main() {
  HttpOverrides? saved;
  late FakeInstance fake;
  late ProviderContainer c;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    fake = await FakeInstance.start(stepUpPrompt: _Yes());
    c = appContainer(fake);
  });

  tearDown(() async {
    c.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  test('the example of 07 9.3 end to end over REST', () async {
    final control = await connected(c, fake);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    final world = fake.server.world;
    final alex = fake.seed.alex.id;
    final sam = fake.seed.sam.id;
    final texter = world.addPerson(
      handle: 'texter',
      displayName: 'Texter',
      kind: 'agent',
    );
    final marketing = world.addSpace('Marketing');
    final campaigns = world.addChannel(marketing, 'Kampagnen', {
      alex: 'trigger',
      texter.id: 'write',
    });
    final internal = world.addChannel(marketing, 'Intern', {
      alex: 'trigger',
      texter.id: 'read',
    });
    final admin = c.read(adminRepositoryProvider)!;

    final targets = await admin.targets();
    final channel = targets.firstWhere((t) => t.id == campaigns);
    expect(channel.path, 'Marketing/Kampagnen');
    expect(
      (await admin.agents()).map((a) => a.displayName),
      contains('Texter'),
    );

    final request = GrantRequest(
      subjectKind: AdminSubjectKind.user,
      subjectId: sam,
      target: channel,
      level: AccessLevel.trigger,
      expiresAt: DateTime.utc(2026, 12, 31, 23),
      reason: 'Campaign work',
    );
    final preview = await admin.previewGrant(request);
    expect(preview.allowed, isTrue);
    expect(preview.text, grantPreviewExample());
    expect(preview.textLocale, 'de');
    expect(preview.effects.single.after, AccessLevel.trigger);
    final h1 = preview.hints.singleWhere((h) => h.code == 'read_horizon');
    expect(h1.agentId, texter.id);
    expect(h1.channelId, internal);
    expect(preview.hints.where((h) => h.code == 'expiry_missing'), isEmpty);

    final grant = await admin.createGrant(request, purpose: 'createGrant');
    expect((await admin.grantsOn(channel)).single.id, grant.id);

    final onChannel = await admin.explain(channel);
    final samEntry = onChannel.entries.singleWhere(
      (e) => e.principal.id == sam,
    );
    expect(samEntry.level, AccessLevel.trigger);
    expect(samEntry.sources.single.kind, AccessSourceKind.direct);
    expect(samEntry.sources.single.grantId, grant.id);

    final onInternal = await admin.explain(
      targets.firstWhere((t) => t.id == internal),
    );
    final viaAgent = onInternal.entries.singleWhere(
      (e) => e.principal.id == sam,
    );
    expect(viaAgent.level, AccessLevel.none);
    expect(viaAgent.sources.single.kind, AccessSourceKind.viaAgent);
    expect(viaAgent.sources.single.agentId, texter.id);

    final snapshot = await admin.viewAs(sam, purpose: 'viewAsUser');
    expect(snapshot.channels.map((v) => v.id), contains(campaigns));
    expect(snapshot.channels.map((v) => v.id), isNot(contains(internal)));

    await admin.revokeGrant(grant.id, reason: 'Campaign over', purpose: 'x');
    expect(await admin.grantsOn(channel), isEmpty);
    expect(fake.server.violations, isEmpty);
  });
}

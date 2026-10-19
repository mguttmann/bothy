// SPDX-License-Identifier: Apache-2.0

// The audit log of the administration against the fake server, headless (brief M4-29): the
// filters reach `listAuditEvents` as query parameters, the chain check reports the broken
// entry, the export returns the server's NDJSON bytes unchanged, and each of the three asks the
// step-up prompt exactly as `docs/spec/actions.yaml` marks `audit.read` and `audit.verify`,
// which the fake reads. A streamed export hides its problem body from the step-up
// interceptor, so the repository asks with one entry of the range first (D-M4-29-4).

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:auth/auth.dart';
import 'package:client/admin/admin_route.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';

/// Answers every step-up with yes and records its purpose.
class RecordingPrompt implements StepUpPrompt {
  /// The purposes asked.
  final List<String> asked = [];

  @override
  Future<bool> ask(String purposeText) async {
    asked.add(purposeText);
    return true;
  }
}

void main() {
  HttpOverrides? saved;
  late FakeInstance fake;
  late ProviderContainer c;
  late RecordingPrompt prompt;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    prompt = RecordingPrompt();
    fake = await FakeInstance.start(stepUpPrompt: prompt);
    c = appContainer(fake);
  });

  tearDown(() async {
    c.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  test(
    'list, check and export over the wire with the catalogue step-up',
    () async {
      final control = await connected(c, fake);
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      expect(
        c.read(onboardingControllerProvider).step,
        OnboardingStep.signedIn,
      );
      final admin = c.read(adminRepositoryProvider)!;
      final world = fake.server.world;
      final log = fake.server.audit;
      final alex = fake.seed.alex.id;
      final sam = fake.seed.sam.id;
      final space = world.nextId();
      log
        ..add(
          world,
          action: 'grant.create',
          actor: alex,
          resourceKind: 'space',
          resourceId: space,
          details: const {'reason': 'Deputy'},
          at: DateTime.utc(2026, 10, 16, 9),
        )
        ..add(
          world,
          action: 'agent.invoke',
          decision: 'deny',
          denyReason: 'missing_grant',
          actor: sam,
          at: DateTime.utc(2026, 10, 16, 10),
        )
        ..add(
          world,
          action: 'grant.create',
          actor: alex,
          at: DateTime.utc(2026, 10, 17, 8),
        );

      void stale() {
        for (final s in world.sessions.values) {
          s.stepUpAt = null;
        }
        prompt.asked.clear();
      }

      final steps = <String, String>{};

      stale();
      final page = await admin.auditEvents(
        AuditFilter(
          action: 'grant.create',
          actorId: alex,
          result: AuditResult.allow,
          resourceKind: 'space',
          resourceId: space,
          from: DateTime.utc(2026, 10, 16),
          to: DateTime.utc(2026, 10, 17),
        ),
        purpose: 'listAuditEvents',
      );
      expect(page.items.single.seq, 1);
      expect(page.items.single.reason, 'Deputy');
      final query = Uri.parse(
        fake.server.requestsOf('listAuditEvents').last.path,
      );
      expect(fake.server.requestsOf('listAuditEvents').last.status, 200);
      steps['listAuditEvents'] = prompt.asked.join();
      expect(query.path, endsWith('/audit/events'));

      stale();
      final intact = await admin.verifyAuditChain(purpose: 'verifyAuditChain');
      expect(intact.ok, isTrue);
      expect(intact.checkedToSeq, 3);
      steps['verifyAuditChain'] = prompt.asked.join();

      log.tamper(2);
      final broken = await admin.verifyAuditChain(
        fromSeq: 1,
        toSeq: 3,
        purpose: 'verifyAuditChain',
      );
      expect(broken.ok, isFalse);
      expect(broken.firstBrokenSeq, 2);
      expect(broken.failureReason, 'hash_mismatch');

      stale();
      final bytes = await admin
          .exportAudit(
            from: DateTime.utc(2026, 10, 16, 9, 30),
            to: DateTime.utc(2026, 10, 18),
            purpose: 'exportAuditEvents',
          )
          .expand((chunk) => chunk)
          .toList();
      steps['exportAuditEvents'] = prompt.asked.join();
      final lines = String.fromCharCodes(bytes).trim().split('\n');
      expect(lines, hasLength(2), reason: 'seq 2 and 3');
      expect(fake.server.requestsOf('exportAuditEvents').last.status, 200);

      for (final MapEntry(key: op, value: asked) in steps.entries) {
        expect(
          asked,
          fake.server.stepUpOf(op) == 'true' ? op : isEmpty,
          reason: '$op (${fake.server.actionOf(op)})',
        );
      }
      expect(fake.server.violations, isEmpty);
    },
  );

  test('a person who is not an owner sees the server refusal', () async {
    final control = await connected(c, fake);
    await control.loginWithPassword(
      WorldSeed.samLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    final admin = c.read(adminRepositoryProvider)!;
    await expectLater(
      admin.auditEvents(const AuditFilter(), purpose: 'read'),
      throwsA(
        isA<SelfActionFailed>().having((e) => e.problem?.status, 'status', 403),
      ),
    );
    await expectLater(
      admin
          .exportAudit(
            from: DateTime.utc(2026, 10),
            to: DateTime.utc(2026, 11),
            purpose: 'export',
          )
          .drain<void>(),
      throwsA(
        isA<SelfActionFailed>().having((e) => e.problem?.status, 'status', 403),
      ),
    );
  });
}

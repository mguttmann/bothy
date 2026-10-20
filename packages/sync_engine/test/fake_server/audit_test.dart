// SPDX-License-Identifier: Apache-2.0

// The audit handlers of the fake (brief M4-29 Scope "Fake server handlers"): the list filters
// one to one by the query parameters of `listAuditEvents` with cursor paging, newest first;
// the export streams the contiguous seq span as NDJSON; the check finds the first broken entry
// of a chain broken with `FakeAuditLog.tamper`. Reading and checking are owner actions with the
// step-up of `docs/spec/actions.yaml`.

import 'dart:convert';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;

  setUp(() async => h = await FakeWorldHarness.start());
  tearDown(() => h.stop());

  void seed() {
    final world = h.server.world;
    final log = h.server.audit;
    final alex = h.seed.alex.id;
    final sam = h.seed.sam.id;
    log
      ..add(
        world,
        action: 'grant.create',
        actor: alex,
        resourceKind: 'space',
        resourceId: world.nextId(),
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
  }

  test('the catalogue gives audit.read and audit.verify their step-up', () {
    expect(h.server.actionOf('listAuditEvents'), 'audit.read');
    expect(h.server.actionOf('exportAuditEvents'), 'audit.read');
    expect(h.server.actionOf('verifyAuditChain'), 'audit.verify');
    expect(h.server.stepUpOf('listAuditEvents'), 'true');
    expect(h.server.stepUpOf('verifyAuditChain'), 'true');
  });

  test('the list filters, pages newest first and asks for step-up', () async {
    seed();
    final alex = h.device();
    await signedIn(h, alex);
    h.server.requests.clear();
    final audit = alex.api.getAuditApi();

    final all = (await audit.listAuditEvents()).data!;
    expect(all.items.map((e) => e.seq), [3, 2, 1]);
    expect(
      [for (final r in h.server.requests) '${r.operationId} ${r.status}'],
      [
        'listAuditEvents 401',
        'createStepUpChallenge 200',
        'completeStepUpWithDeviceKey 200',
        'listAuditEvents 200',
      ],
    );

    final denied = (await audit.listAuditEvents(decision: AuditDecision.deny))
        .data!;
    expect(denied.items.single.denyReason, 'missing_grant');
    final bySam = (await audit.listAuditEvents(actorId: h.seed.sam.id)).data!;
    expect(bySam.items.single.action, 'agent.invoke');
    final byAction = (await audit.listAuditEvents(action: 'grant.create'))
        .data!;
    expect(byAction.items.map((e) => e.seq), [3, 1]);
    final byKind = (await audit.listAuditEvents(resourceKind: 'space')).data!;
    expect(byKind.items.single.seq, 1);
    final byTime = (await audit.listAuditEvents(
      from: DateTime.utc(2026, 10, 16, 9, 30),
      to: DateTime.utc(2026, 10, 17),
    )).data!;
    expect(byTime.items.single.seq, 2);

    final first = (await audit.listAuditEvents(limit: 2)).data!;
    expect(first.items.map((e) => e.seq), [3, 2]);
    final second = (await audit.listAuditEvents(
      limit: 2,
      cursor: first.nextCursor,
    )).data!;
    expect(second.items.map((e) => e.seq), [1]);
    expect(second.nextCursor, isNull);
    expect(h.server.violations, isEmpty);
  });

  test('a person who is not an owner gets 403', () async {
    seed();
    final sam = h.device(login: WorldSeed.samLogin, keySeed: 3);
    await signedIn(h, sam);
    try {
      await sam.api.getAuditApi().listAuditEvents();
      fail('expected 403');
    } on DioException catch (e) {
      expect(ApiProblem.of(e)?.status, 403);
    }
  });

  test('the export streams the contiguous span as NDJSON', () async {
    seed();
    final alex = h.device();
    await signedIn(h, alex);
    // A bytes response hides the problem body from the step-up interceptor, so the step-up
    // runs on a JSON call first (the admin client does the same, D-M4-29-4).
    await alex.api.getAuditApi().listAuditEvents(limit: 1);
    final response = await alex.api.dio.get<List<int>>(
      '/audit/export',
      queryParameters: {
        'from': '2026-10-16T09:30:00.000Z',
        'to': '2026-10-18T00:00:00.000Z',
      },
      options: Options(
        responseType: ResponseType.bytes,
        extra: {
          'secure': [
            {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          ],
        },
      ),
    );
    expect(response.headers.value('content-type'), contains('ndjson'));
    final lines = const LineSplitter().convert(utf8.decode(response.data!));
    final rows = [for (final l in lines) jsonDecode(l) as Map<String, Object?>];
    expect(rows.map((r) => r['seq']), [2, 3]);
    expect(rows[1]['prev_hash'], rows[0]['hash']);
    expect(h.server.violations, isEmpty);
  });

  test('the check finds the first broken entry', () async {
    seed();
    h.server.audit.warnings.add('checkpoint_sink_not_configured');
    final alex = h.device();
    await signedIn(h, alex);
    final audit = alex.api.getAuditApi();

    final intact = (await audit.verifyAuditChain(
      auditVerifyRequest: AuditVerifyRequest(),
    )).data!;
    expect(intact.ok, isTrue);
    expect(intact.checkedToSeq, 3);
    expect(intact.warnings, ['checkpoint_sink_not_configured']);

    h.server.audit.tamper(2);
    final broken = (await audit.verifyAuditChain(
      auditVerifyRequest: AuditVerifyRequest(
        (b) => b
          ..fromSeq = 1
          ..toSeq = 3,
      ),
    )).data!;
    expect(broken.ok, isFalse);
    expect(broken.firstBrokenSeq, 2);
    expect(
      broken.failureReason,
      AuditVerifyResultFailureReasonEnum.hashMismatch,
    );
    expect(h.server.violations, isEmpty);
  });
}

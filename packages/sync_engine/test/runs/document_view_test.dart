// SPDX-License-Identifier: Apache-2.0

// RunDocument: typed getters for the live box fields of 09 5.4 over the raw map.

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

Map<String, Object?> _step(
  String id,
  int? ordinal, {
  String kind = 'tool_call',
  String status = 'running',
  String? parent,
}) => {
  'id': id,
  'ordinal': ?ordinal,
  'kind': kind,
  'name': 'name-$id',
  'title': 'title-$id',
  'status': status,
  'parent_step_id': ?parent,
  'started_at': '2026-10-15T09:00:00Z',
};

String _id(int n) =>
    '0192f4a0-7b1c-7000-8a00-${n.toRadixString(16).padLeft(12, '0')}';

RunDocument _withSteps(List<Map<String, Object?>> steps) =>
    RunDocument.fromJson({
      'version': 1,
      'steps': {for (final s in steps) s['id']! as String: s},
    });

void main() {
  test('reads the fields of 09 5.4 from the fixture', () {
    final d = RunDocument.fromJson(doc(4));
    expect(d.id, runId);
    expect(d.version, const ResourceVersion(4));
    expect(d.state, RunState.running);
    expect(d.stateReason, isNull);
    expect(d.statusLine, isNull);
    expect(d.lastTextExcerpt, 'The export fails on empty rows.');
    expect(d.model, 'model-large');
    expect(d.summary, isNull);
    expect(d.metrics.contextPct, 21.0);
    expect(d.metrics.tokensIn, 1200);
    expect(d.metrics.tokensOut, 300);
    expect(d.metrics.tokensCacheRead, 800);
    expect(d.metrics.tokensCacheWrite, isNull);
    expect(d.metrics.lastApiRetry!.attempt, 2);
    expect(d.metrics.lastApiRetry!.maxRetries, 5);
    expect(d.metrics.lastApiRetry!.retryDelayMs, 4000);
    expect(d.metrics.lastApiRetry!.error, 'overloaded');
    expect(
      d.cost,
      const Money(amountMicros: 2500000, currency: 'USD', isEstimate: true),
    );
    expect(d.billingModeUsed, BillingMode.organization);
    expect(d.billingModeExpected, BillingMode.organization);
    expect(d.planItems.map((i) => (i.id, i.text, i.status)), [
      ('step-1', 'Reproduce the export error', PlanItemStatus.inProgress),
      ('step-2', 'Fix the export', PlanItemStatus.pending),
    ]);
    expect(d.followups.single.messageId, followupB1);
    expect(d.followups.single.state, FollowupState.absorbed);
    expect(d.queuedAt, DateTime.utc(2026, 10, 15, 9));
    expect(d.startedAt, DateTime.utc(2026, 10, 15, 9, 0, 0, 500));
    expect(d.finishedAt, isNull);
    expect(d.durationMs, isNull);
    expect(d.stallWarnedAt, isNull);
  });

  test('stall_warned_at, times and the summary are read when present', () {
    final d = RunDocument.fromJson({
      ...doc(1),
      'stall_warned_at': '2026-10-15T09:05:00Z',
      'finished_at': '2026-10-15T09:06:00Z',
      'duration_ms': 360000,
      'summary': 'Fixed the export',
      'state': 'completed',
      'state_reason': 'none',
    });
    expect(d.stallWarnedAt, DateTime.utc(2026, 10, 15, 9, 5));
    expect(d.finishedAt, DateTime.utc(2026, 10, 15, 9, 6));
    expect(d.durationMs, 360000);
    expect(d.summary, 'Fixed the export');
    expect(d.state, RunState.completed);
    expect(d.stateReason, 'none');
  });

  test('missing or malformed fields read as null, empty or unknown', () {
    final d = RunDocument.fromJson({
      'version': 7,
      'id': 'not-a-uuid',
      'state': 'teleporting',
      'metrics': 'n/a',
      'cost': {'amount_micros': 'many'},
      'billing_mode_used': 'barter',
      'plan': {'items': 'none'},
      'steps': [1, 2],
      'followups': {'x': 'y'},
      'stall_warned_at': 'yesterday',
      'status_line': 5,
    });
    expect(d.id, isNull);
    expect(d.state, RunState.unknown);
    expect(d.metrics.contextPct, isNull);
    expect(d.metrics.lastApiRetry, isNull);
    expect(d.cost, isNull);
    expect(d.billingModeUsed, BillingMode.unknown);
    expect(d.billingModeExpected, isNull);
    expect(d.planItems, isEmpty);
    expect(d.stepsInOrder, isEmpty);
    expect(d.followups, isEmpty);
    expect(d.stallWarnedAt, isNull);
    expect(d.statusLine, isNull);
  });

  test('a document without an integer version is rejected', () {
    expect(
      () => RunDocument.fromJson({'state': 'running'}),
      throwsFormatException,
    );
    expect(() => RunDocument.fromJson({'version': '3'}), throwsFormatException);
  });

  test('steps are ordered by ordinal, children under their parent', () {
    final d = RunDocument.fromJson(doc(3));
    final roots = d.stepsInOrder;
    expect(roots.map((n) => n.step.id), [stepA1, stepA2, stepA3]);
    expect(roots[2].step.kind, StepKind.subagent);
    expect(roots[2].children.map((n) => n.step.id), [stepA4]);
    expect(roots[2].children.single.step.parentStepId, stepA3);
    expect(roots[0].step.status, StepStatus.completed);
    expect(roots[0].step.finishedAt, DateTime.utc(2026, 10, 15, 9, 0, 3));
    expect(roots[0].step.name, 'read_file');
    expect(roots[0].step.title, 'Reading export.log');
    expect(roots[0].step.ordinal, 1);
    expect(roots[0].step.detail, isNull);
  });

  test('ordering ignores the order of the map keys', () {
    final d = _withSteps([
      _step(_id(3), 30),
      _step(_id(1), 10),
      _step(_id(2), 20, parent: _id(1)),
      _step(_id(4), 5, parent: _id(1)),
    ]);
    expect(d.stepsInOrder.map((n) => n.step.id), [_id(1), _id(3)]);
    expect(d.stepsInOrder.first.children.map((n) => n.step.id), [
      _id(4),
      _id(2),
    ]);
  });

  test(
    'a step without ordinal sorts last, an orphan or a cycle stays visible',
    () {
      final d = _withSteps([
        _step(_id(1), null),
        _step(_id(2), 2, parent: _id(99)),
        _step(_id(3), 3, parent: _id(4)),
        _step(_id(4), 4, parent: _id(3)),
        _step(_id(5), 1),
      ]);
      final ids = <String>[];
      void walk(List<RunStepNode> nodes) {
        for (final n in nodes) {
          ids.add(n.step.id);
          walk(n.children);
        }
      }

      walk(d.stepsInOrder);
      expect(ids.toSet(), {_id(1), _id(2), _id(3), _id(4), _id(5)});
      expect(ids, hasLength(5), reason: 'every step exactly once');
      expect(d.stepsInOrder.first.step.id, _id(5));
      expect(d.stepsInOrder.map((n) => n.step.id).contains(_id(2)), isTrue);
    },
  );

  test('the current tool is the latest running tool call', () {
    expect(RunDocument.fromJson(doc(1)).currentTool!.id, stepA1);
    expect(RunDocument.fromJson(doc(3)).currentTool!.id, stepA4);
    expect(RunDocument.fromJson(doc(4)).currentTool!.id, stepA2);
    expect(
      _withSteps([_step(_id(1), 1, status: 'completed')]).currentTool,
      isNull,
    );
    final d = _withSteps([
      _step(_id(1), 1),
      _step(_id(2), 2, kind: 'subagent'),
      _step(_id(3), 3, status: 'completed'),
    ]);
    expect(d.currentTool!.id, _id(1));
  });

  test('open subagents, workflows and background jobs', () {
    final d = _withSteps([
      _step(_id(1), 1, kind: 'subagent'),
      _step(_id(2), 2, kind: 'workflow', status: 'pending'),
      _step(_id(3), 3, kind: 'background_job', status: 'completed'),
      _step(_id(4), 4, kind: 'background_job'),
      _step(_id(5), 5),
    ]);
    expect(d.openSubagents.map((s) => s.id), [_id(1), _id(2), _id(4)]);
    expect(RunDocument.fromJson(doc(4)).openSubagents.single.id, stepA3);
  });

  test('unknown fields are kept in the map and ignored by the getters', () {
    final d = RunDocument.fromJson({
      ...doc(1),
      'flux_capacitor': {'level': 3},
    });
    expect(d.raw['flux_capacitor'], {'level': 3});
    expect(d.statusLine, 'Reading the export logs');
    expect(() => d.raw['status_line'] = 'x', throwsUnsupportedError);
  });

  test('unknown codes of enums decode to unknown', () {
    final d = RunDocument.fromJson({
      'version': 1,
      'plan': {
        'items': [
          {'id': 'a', 'text': 't', 'status': 'skipped'},
          'not an item',
        ],
      },
      'followups': {
        'm': {'message_id': 'm', 'state': 'lost'},
      },
      'steps': {
        'x': {'id': 'x', 'ordinal': 1, 'kind': 'dance', 'status': 'paused'},
      },
    });
    expect(d.planItems.single.status, PlanItemStatus.unknown);
    expect(d.followups.single.state, FollowupState.unknown);
    expect(d.stepsInOrder.single.step.kind, StepKind.unknown);
    expect(d.stepsInOrder.single.step.status, StepStatus.unknown);
  });

  test('a step without an id field takes its key', () {
    final d = RunDocument.fromJson({
      'version': 1,
      'steps': {
        'k': {'ordinal': 1, 'kind': 'tool_call', 'status': 'running'},
      },
    });
    expect(d.stepsInOrder.single.step.id, 'k');
    expect(d.stepsInOrder.single.step.name, isNull);
  });
}

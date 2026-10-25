// SPDX-License-Identifier: Apache-2.0

import 'package:api_client/api_client.dart' as api;

/// The run ID of the unit tests.
const String runId = '0192f4a0-7b1c-7000-8a00-000000000901';

/// The start of the run of the unit tests.
final DateTime t0 = DateTime.utc(2026, 10, 24, 9);

/// [t0] plus [seconds].
String at(int seconds) => t0.add(Duration(seconds: seconds)).toIso8601String();

/// One step of `run_document.v1` (04 11.1) as JSON.
Map<String, Object?> step(
  String id,
  int ordinal, {
  String kind = 'tool_call',
  String name = 'Read',
  String title = '',
  String status = 'completed',
  String? parent,
  int start = 0,
  int? end,
  Map<String, Object?>? detail,
}) => {
  'id': id,
  'ordinal': ordinal,
  'kind': kind,
  'name': name,
  'title': title,
  'status': status,
  'parent_step_id': ?parent,
  'detail': ?detail,
  'started_at': at(start),
  'finished_at': ?(end == null ? null : at(end)),
};

/// A run document with [steps], decoded by the generated serializer.
api.Run runOf(
  List<Map<String, Object?>> steps, {
  String state = 'running',
  String mode = 'organization',
  String? used = 'organization',
  int micros = 620000,
  bool estimate = true,
  int? finishedAfter,
  int? stalledAfter,
  bool started = true,
}) {
  final json = <String, Object?>{
    'id': runId,
    'thread_id': '0192f4a0-7b1c-7000-8a00-000000000301',
    'channel_id': '0192f4a0-7b1c-7000-8a00-000000000102',
    'agent_id': '0192f4a0-7b1c-7000-8a00-000000000202',
    'triggered_by': '0192f4a0-7b1c-7000-8a00-000000000401',
    'state': state,
    'version': 7,
    'billing_mode_expected': mode,
    'billing_mode_used': ?used,
    'metrics': <String, Object?>{},
    'cost': {
      'amount_micros': micros,
      'currency': 'USD',
      'is_estimate': estimate,
    },
    'steps': {for (final s in steps) s['id']! as String: s},
    'followups': <String, Object?>{},
    'queued_at': t0.subtract(const Duration(seconds: 5)).toIso8601String(),
    'started_at': ?(started ? t0.toIso8601String() : null),
    'finished_at': ?(finishedAfter == null ? null : at(finishedAfter)),
    'stall_warned_at': ?(stalledAfter == null ? null : at(stalledAfter)),
  };
  return api.standardSerializers.deserializeWith(api.Run.serializer, json)!;
}

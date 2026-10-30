// SPDX-License-Identifier: Apache-2.0

// The cause of a failed run (RUNNER-NO-GIT): the adapter's `summary` of a failed or cancelled
// run becomes the reason detail of the live box, from the run document and from the summary.

import 'package:client/agents/run_mapping.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/agent_fakes.dart';
import '../support/structure.dart';

const _cause = 'This forum has no project folder. Bind a folder.';

void main() {
  test('a failed run document carries its summary as the reason detail', () {
    final doc = RunDocument.fromJson({
      ...runDoc(1, state: 'failed', reason: 'adapter_error'),
      'summary': _cause,
    });
    final data = runFromDocument(doc, agentName: 'Robin', now: t0);
    expect(data.reasonDetail, _cause);
  });

  test('a completed run document keeps its summary out of the reason', () {
    final doc = RunDocument.fromJson({
      ...runDoc(1, state: 'completed'),
      'summary': 'Found the cause',
    });
    final data = runFromDocument(doc, agentName: 'Robin', now: t0);
    expect(data.reasonDetail, isNull);
  });

  test('a failed run summary shows its cause once, as the reason detail', () {
    final data = runFromSummary(
      run(
        1,
        RunState.failed,
        reason: 'adapter_error',
      ).copyWith(summary: _cause),
      agentName: 'Robin',
      now: t0,
    );
    expect(data.reasonDetail, _cause);
    expect(data.lastTextExcerpt, isNull);
  });

  test('a completed run summary stays the last text', () {
    final data = runFromSummary(
      run(1, RunState.completed).copyWith(summary: 'Found the cause'),
      agentName: 'Robin',
      now: t0,
    );
    expect(data.reasonDetail, isNull);
    expect(data.lastTextExcerpt, 'Found the cause');
  });
}

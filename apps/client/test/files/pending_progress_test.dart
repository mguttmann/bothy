// SPDX-License-Identifier: Apache-2.0

// The waiting message in the history (brief M4-22 Design note 1; 12 2): its upload chips
// follow the progress of every confirmed part, like the chips of the composer.

import 'dart:convert';

import 'package:client/files/upload_executor.dart';
import 'package:client/thread/outbox_rows.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/widgets.dart';

void main() {
  final t0 = DateTime.utc(2026, 10, 22, 9, 40);

  OutboxEntryView entry(String id, String kind, Map<String, Object?> payload) =>
      OutboxEntryView(
        entry: OutboxEntry(
          id: id,
          kind: kind,
          payload: jsonEncode(payload),
          state: OutboxState.pending,
          attempts: 1,
          createdAt: t0,
          stateChangedAt: t0,
        ),
        stateLabel: OutboxStateLabel.sending,
      );

  testWidgets('the chip of a waiting message shows each new share', (
    tester,
  ) async {
    final progress = UploadProgress();
    final message = entry('m1', MessageOps.createMessage, {
      'content_md': 'the report',
      'attachment_preview': [
        {'key': 'k1', 'name': 'report.pdf', 'size_bytes': 100},
      ],
    });
    final upload = entry('u1', UploadOps.upload, {'attachment_key': 'k1'});
    progress.debugReport('k1', confirmed: 0, total: 100);
    await tester.pumpWidget(
      hostScreen(
        PendingRow(
          view: message,
          authorName: 'Alex',
          timeLabel: '09:40',
          collapseLines: 6,
          onResend: () {},
          onEdit: () {},
          onDiscard: () {},
          onOpenLink: (_) {},
          uploads: {'k1': upload},
          progress: progress,
        ),
      ),
    );
    expect(find.textContaining('0%'), findsOneWidget);
    progress.debugReport('k1', confirmed: 50, total: 100);
    await tester.pump();
    expect(find.textContaining('50%'), findsOneWidget);
    progress.dispose();
  });
}

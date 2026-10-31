// SPDX-License-Identifier: Apache-2.0

// The events the server writes when its scan of a file ends (FIX-scan-state-stale, 12 2): the
// envelopes of `test/fixtures/file_updated_after_scan.json` come from the real scan pass of the
// server (TestScanEventGolden in services/server/internal/files/app), once for a file that is not
// attached yet (personal, to the uploader, in the scope of its channel) and once for an attached
// one (in the scope of the thread of its message). The sync engine stores both, and the stored
// row then follows every later `file.updated`, so a view of the row leaves "being checked".

import 'dart:convert';
import 'dart:io';

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  final golden = [
    for (final e in jsonDecode(
      File('test/fixtures/file_updated_after_scan.json').readAsStringSync(),
    ) as List)
      (e as Map).cast<String, Object?>(),
  ];
  final file = FileId.parse(id(0x7001));

  Future<void> open() async {
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
    await w.dispatch([threadCreated(31, 1), messageCreated(32, 1)]);
  }

  Map<String, Object?> withSeq(Map<String, Object?> e, int seq) => {
    ...e,
    'seq': seq,
  };

  test('the golden file holds the unattached and the attached event', () {
    expect(golden, hasLength(2));
    expect(golden[0]['audience'], 'self');
    expect((golden[1]['scope']! as Map)['thread_id'], threadId(1));
  });

  test('the scan event of an unattached file is stored', () async {
    await open();
    await w.dispatch([withSeq(golden[0], 33)]);
    final stored = await w.db.messagesDao.fileById(file);
    expect(stored?.scanStatus, ScanStatus.clean);
    expect(stored?.uploadState, UploadState.pending);
    expect(stored?.scopeChannelId, ChannelId.parse(channelId(1)));
    expect(await w.cursor(), 33);
  });

  test('the scan event of an attached file replaces the pending row, and the '
      'image job event makes it servable', () async {
    await open();
    final pending = jsonDecode(jsonEncode(golden[1])) as Map<String, Object?>;
    final payload = (pending['payload']! as Map)['file']! as Map;
    payload['scan_status'] = 'pending';
    await w.dispatch([withSeq(pending, 33)]);
    expect(
      (await w.db.messagesDao.fileById(file))?.scanStatus,
      ScanStatus.pending,
    );

    await w.dispatch([withSeq(golden[1], 34)]);
    final scanned = await w.db.messagesDao.fileById(file);
    expect(scanned?.scanStatus, ScanStatus.clean);
    expect(scanned?.uploadState, UploadState.pending);
    expect(scanned?.scopeChannelId, isNull);

    // The image job stores the cleaned original and its thumbnails (12 7.1, 7.3).
    final done = jsonDecode(jsonEncode(golden[1])) as Map<String, Object?>;
    ((done['payload']! as Map)['file']! as Map)
      ..['upload_state'] = 'complete'
      ..['sha256'] = 'ab' * 32
      ..['thumbnails'] = [
        {
          'variant': 'small',
          'mime_type': 'image/jpeg',
          'width': 8,
          'height': 8,
        },
      ];
    await w.dispatch([withSeq(done, 35)]);
    final servable = await w.db.messagesDao.fileById(file);
    expect(servable?.uploadState, UploadState.complete);
    expect(servable?.thumbnails, hasLength(1));
    expect(await w.cursor(), 35);
  });
}

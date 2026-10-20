// SPDX-License-Identifier: Apache-2.0

// Downloads against the fake server (brief M4-22 Tests; 12 9): the bytes stay only when their
// SHA-256 equals `Repr-Digest`, a mismatch deletes them; "being checked" and "blocked" are
// their own results; Save as copies the checked bytes; a ticket URL reaches the player and
// never a log, the database or the cache.

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:client/files/download_service.dart';
import 'package:client/files/file_cache.dart';
import 'package:client/files/ticket_player.dart';
import 'package:domain/domain.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:sync_engine/testing.dart';

import '../support/file_fakes.dart';

void main() {
  HttpOverrides? saved;
  late UploadHarness h;
  late FileCache cache;
  late DownloadService downloads;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    h = await UploadHarness.start();
    cache = FileCache(db: h.db, root: Directory('${h.folder.path}/cache'));
    downloads = DownloadService(api: h.api, cache: cache);
  });
  tearDown(() async {
    await h.stop();
    HttpOverrides.global = saved;
  });

  final bytes = utf8.encode('the server copy of a report');

  FileId ready({String scan = 'clean', String upload = 'complete'}) =>
      FileId.parse(
        h.server.world.addReadyFile(
          uploaderId: h.seed.alex.id,
          channelId: h.seed.general,
          name: 'report.txt',
          mimeType: 'text/plain',
          bytes: bytes,
          scanStatus: scan,
          uploadState: upload,
        ),
      );

  List<FileSystemEntity> cached() => cache.root.existsSync()
      ? cache.root.listSync(recursive: true).whereType<File>().toList()
      : const [];

  test('a matching Repr-Digest keeps the bytes in the cache', () async {
    final id = ready();
    final result = await downloads.fetch(id, name: 'report.txt');
    expect(result, isA<Downloaded>());
    final file = (result as Downloaded).file;
    expect(await file.readAsBytes(), bytes);
    expect(file.path, endsWith('report.txt'));
    expect(await h.db.fileCacheDao.entry(id, 'original'), isNotNull);
    // A second fetch is a cache hit: no second request.
    await downloads.fetch(id, name: 'report.txt');
    expect(h.server.requestsOf('downloadFile'), hasLength(1));
  });

  test('a stale DPoP nonce is renewed by the auth interceptors', () async {
    final id = ready();
    await downloads.fetch(ready());
    // Both nonces the client knows become stale: the next request gets `use_dpop_nonce`.
    h.server.verifier
      ..rotateNonce()
      ..rotateNonce();
    final result = await downloads.fetch(id);
    expect(result, isA<Downloaded>());
    expect(
      h.server.requestsOf('downloadFile').map((r) => r.status),
      containsAllInOrder([401, 200]),
    );
  });

  test('a mismatching Repr-Digest deletes the bytes', () async {
    final id = ready();
    h.server.world.fileSettings.wrongDigest.add(id.value);
    final result = await downloads.fetch(id);
    expect(result, isA<DownloadDamaged>());
    expect(await h.db.fileCacheDao.entry(id, 'original'), isNull);
    expect(cached(), isEmpty, reason: 'no partial file stays');
  });

  test('being checked and blocked are their own results', () async {
    expect(
      await downloads.fetch(ready(upload: 'pending')),
      isA<DownloadNotReady>(),
    );
    expect(
      await downloads.fetch(ready(scan: 'infected')),
      isA<DownloadBlocked>(),
    );
    expect(cached(), isEmpty);
  });

  test('Save as copies the checked bytes to the chosen place', () async {
    final id = ready();
    final picker = FakePicker()..saveTo = '${h.folder.path}/saved.txt';
    final meta = FileMeta(
      id: id,
      sizeBytes: bytes.length,
      mimeType: 'text/plain',
      originalName: 'report.txt',
      uploadedBy: PrincipalId.parse(h.seed.alex.id),
      purpose: FilePurpose.attachment,
      uploadState: UploadState.complete,
      scanStatus: ScanStatus.clean,
      transcriptStatus: TranscriptStatus.none,
      thumbnails: const [],
      createdAt: DateTime.utc(2026, 10, 17),
    );
    final result = await downloads.saveAs(meta, picker);
    expect(result, isA<Downloaded>());
    expect(picker.asked, ['report.txt']);
    expect(await File('${h.folder.path}/saved.txt').readAsBytes(), bytes);
    picker.saveTo = null;
    expect(await downloads.saveAs(meta, picker), isA<DownloadCancelled>());
  });

  test('a ticket URL reaches the player and never a log', () async {
    final id = ready();
    final lines = <String>[];
    final previous = debugPrint;
    debugPrint = (message, {wrapWidth}) => lines.add('$message');
    addTearDown(() => debugPrint = previous);
    final player = FakePlayer();
    await runZoned(
      () async {
        final url = await FileTickets(h.api, h.server.baseUri).url(id);
        await player.play(url);
      },
      zoneSpecification: ZoneSpecification(
        print: (self, parent, zone, line) => lines.add(line),
      ),
    );
    final played = player.played.single;
    expect(played.queryParameters['ticket'], isNotEmpty);
    // The player's own request needs no DPoP: the ticket alone serves the bytes.
    final http = HttpClient();
    addTearDown(() => http.close(force: true));
    final response = await (await http.getUrl(played)).close();
    expect(await response.fold<List<int>>([], (a, b) => a..addAll(b)), bytes);
    final ticket = played.queryParameters['ticket']!;
    expect(lines.where((l) => l.contains(ticket)), isEmpty);
    // Nothing the app stored holds the ticket.
    final stored = [
      for (final e in await h.db.outboxDao.allEntries()) e.payload,
      for (final f in cached()) f.path,
    ];
    expect(stored.where((s) => s.contains(ticket)), isEmpty);
  });
}

// SPDX-License-Identifier: Apache-2.0

// The file handlers M4-22 added to the fake (brief M4-22 Scope): upload sessions with parts,
// the fault "abort after part n" and resuming from `getUpload`, `part_conflict`, completion
// with the scan, the states "being checked" and "blocked" (12 2), downloads with
// `Repr-Digest`, file tickets without DPoP, transcripts and the attach rules of 12 10. Every
// request and response passes the contract check of the fake.

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:crypto/crypto.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late FakeWorld world;
  late TestDevice alex;
  late FilesApi files;

  setUp(() async {
    h = await FakeWorldHarness.start();
    world = h.server.world;
    world.fileSettings.partBytes = 4;
    alex = h.device();
    await signedIn(h, alex);
    files = alex.api.getFilesApi();
  });
  tearDown(() => h.stop());

  Future<(int?, String?)> problem(Future<Object?> call) => call.then(
    (_) => (null, null),
    onError: (Object e) {
      final d = e as DioException;
      final data = d.response?.data;
      // A binary operation (`downloadFile`) gets its problem as bytes.
      final json = data is List<int>
          ? jsonDecode(utf8.decode(data)) as Map<String, Object?>
          : null;
      final p = ApiProblem.of(d);
      return (
        d.response?.statusCode,
        p?.reason ?? (json?['code'] as String?) ?? p?.code,
      );
    },
  );

  String digest(List<int> b) =>
      'sha-256=:${base64.encode(sha256.convert(b).bytes)}:';

  Future<UploadSession> session(
    List<int> bytes, {
    String name = 'a.txt',
  }) async => (await files.createUpload(
    channelId: h.seed.general,
    idempotencyKey: world.nextId(),
    uploadCreate: UploadCreate(
      (b) => b
        ..name = name
        ..sizeBytes = bytes.length
        ..sha256 = sha256.convert(bytes).toString()
        ..declaredMimeType = 'text/plain',
    ),
  )).data!;

  Future<UploadPartResult> part(String id, int n, List<int> bytes) async =>
      (await files.uploadPart(
        fileId: id,
        partNumber: n,
        contentDigest: digest(bytes),
        body: MultipartFile.fromBytes(bytes),
      )).data!;

  Future<File> complete(String id, List<UploadPartResult> parts) async =>
      (await files.completeUpload(
        fileId: id,
        idempotencyKey: world.nextId(),
        uploadComplete: UploadComplete((b) => b.parts.replace(parts)),
      )).data!;

  final bytes = utf8.encode('hello world');

  test(
    'parts with digests, completion and a download with Repr-Digest',
    () async {
      final s = await session(bytes);
      expect(s.parts, hasLength(3));
      final done = <UploadPartResult>[];
      for (var n = 1; n <= 3; n++) {
        done.add(
          await part(
            s.fileId,
            n,
            bytes.sublist(
              (n - 1) * 4,
              n * 4 > bytes.length ? bytes.length : n * 4,
            ),
          ),
        );
      }
      final file = await complete(s.fileId, done);
      expect(file.uploadState, FileUploadStateEnum.complete);
      expect(file.scanStatus, FileScanStatusEnum.clean);

      final download = await files.downloadFile(fileId: s.fileId);
      expect(download.data, bytes);
      expect(download.headers.value('repr-digest'), digest(bytes));
      expect(download.headers.value('x-content-type-options'), 'nosniff');
      expect(
        world.log.where((e) => e.envelope['type'] == 'file.updated'),
        isNotEmpty,
      );
      expect(h.server.violations, isEmpty);
    },
  );

  test(
    'an abort after part 2 leaves two confirmed parts to resume from',
    () async {
      world.fileSettings.abortAfterParts = 2;
      final s = await session(bytes);
      await part(s.fileId, 1, bytes.sublist(0, 4));
      await part(s.fileId, 2, bytes.sublist(4, 8));
      expect((await problem(part(s.fileId, 3, bytes.sublist(8)))).$1, 503);
      final state = (await files.getUpload(fileId: s.fileId)).data!;
      expect([for (final p in state.completedParts) p.partNumber], [1, 2]);
      expect(state.uploadState, UploadStateUploadStateEnum.uploading);
      final third = await part(s.fileId, 3, bytes.sublist(8));
      final file = await complete(s.fileId, [
        for (final p in state.completedParts)
          UploadPartResult(
            (b) => b
              ..partNumber = p.partNumber
              ..etag = p.etag,
          ),
        third,
      ]);
      expect(file.uploadState, FileUploadStateEnum.complete);
      expect(h.server.violations, isEmpty);
    },
  );

  test('the same part with other bytes is a part_conflict', () async {
    final s = await session(bytes);
    await part(s.fileId, 1, bytes.sublist(0, 4));
    expect(await part(s.fileId, 1, bytes.sublist(0, 4)), isNotNull);
    expect(await problem(part(s.fileId, 1, utf8.encode('HELL'))), (
      409,
      'part_conflict',
    ));
    expect(
      (await problem(part(s.fileId, 2, utf8.encode('xx')))).$2,
      'validation_failed',
    );
  });

  test(
    'being checked answers file_not_ready, blocked answers file_blocked',
    () async {
      world.fileSettings.holdScan = true;
      final s = await session(bytes, name: 'b.txt');
      final done = [
        await part(s.fileId, 1, bytes.sublist(0, 4)),
        await part(s.fileId, 2, bytes.sublist(4, 8)),
        await part(s.fileId, 3, bytes.sublist(8)),
      ];
      final file = await complete(s.fileId, done);
      expect(file.scanStatus, FileScanStatusEnum.pending);
      expect(await problem(files.downloadFile(fileId: s.fileId)), (
        409,
        'file_not_ready',
      ));
      world.finishScan(s.fileId, result: 'infected');
      expect(await problem(files.downloadFile(fileId: s.fileId)), (
        409,
        'file_blocked',
      ));
      expect(h.server.violations, isEmpty);
    },
  );

  test('a ticket downloads without DPoP and only its own variant', () async {
    final png = _png();
    final id = world.addReadyFile(
      uploaderId: h.seed.alex.id,
      channelId: h.seed.general,
      name: 'shot.png',
      mimeType: 'image/png',
      bytes: png,
    );
    final ticket = (await files.createFileTicket(
      fileId: id,
      fileTicketCreate: FileTicketCreate(
        (b) => b..variant = FileTicketCreateVariantEnum.original,
      ),
    )).data!;
    final http = HttpClient();
    addTearDown(() => http.close(force: true));
    Future<(int, List<int>)> get(String path) async {
      final req = await http.getUrl(h.server.baseUri.resolve(path));
      final res = await req.close();
      final body = await res.fold<List<int>>([], (a, b) => a..addAll(b));
      return (res.statusCode, body);
    }

    final (status, body) = await get(ticket.url);
    expect(status, 200);
    expect(body, png);
    final other = ticket.url.replaceFirst('/content', '/thumbnails/small');
    expect((await get(other)).$1, 404);
    expect((await get('/api/v1/files/$id/content?ticket=nope')).$1, 404);
    expect(h.server.violations, isEmpty);
  });

  test('thumbnails come from the server copy of an image', () async {
    final id = world.addReadyFile(
      uploaderId: h.seed.alex.id,
      channelId: h.seed.general,
      name: 'shot.png',
      mimeType: 'image/png',
      bytes: _png(),
    );
    final meta = (await files.getFile(fileId: id)).data!;
    expect(
      [for (final t in meta.thumbnails) t.variant.name],
      ['small', 'medium'],
    );
    final thumb = await files.downloadThumbnail(fileId: id, variant: 'small');
    expect(thumb.data, _png());
    expect(h.server.violations, isEmpty);
  });

  test(
    'transcripts: unavailable without a service, the text once done',
    () async {
      final id = world.addReadyFile(
        uploaderId: h.seed.alex.id,
        channelId: h.seed.general,
        name: 'voice.m4a',
        mimeType: 'audio/mp4',
        bytes: utf8.encode('audio'),
      );
      expect(
        await problem(
          files.requestTranscription(
            fileId: id,
            idempotencyKey: world.nextId(),
          ),
        ),
        (409, 'transcription_unavailable'),
      );
      world.setTranscript(id, 'Call Sam back');
      final t = (await files.getTranscript(fileId: id)).data!;
      expect(t.text, 'Call Sam back');
      world.fileSettings.transcriptionMode = 'service';
      final again = await files.requestTranscription(
        fileId: id,
        idempotencyKey: world.nextId(),
      );
      expect(again.statusCode, 202);
      expect(h.server.violations, isEmpty);
    },
  );

  test('a file attaches once, and others read it only after that', () async {
    final id = world.addReadyFile(
      uploaderId: h.seed.alex.id,
      channelId: h.seed.general,
      name: 'notes.txt',
      mimeType: 'text/plain',
      bytes: bytes,
    );
    expect(
      () => world.visibleFile(h.seed.sam.id, id),
      throwsA(isA<FakeProblem>()),
    );
    final m = world.createMessage(h.seed.alex.id, h.seed.releaseNotes, {
      'client_msg_id': world.nextId(),
      'content_md': 'see the notes',
      'attachment_file_ids': [id],
    }).message;
    expect((m['attachments']! as List).single, containsPair('position', 1));
    expect(world.visibleFile(h.seed.sam.id, id).id, id);
    expect(
      () => world.createMessage(h.seed.alex.id, h.seed.releaseNotes, {
        'client_msg_id': world.nextId(),
        'attachment_file_ids': [id],
      }),
      throwsA(
        isA<FakeProblem>().having(
          (p) => p.reason,
          'reason',
          'file_not_attachable',
        ),
      ),
    );
  });

  test(
    'a presigned part goes to the object store without Authorization',
    () async {
      world.fileSettings
        ..mode = 'presigned'
        ..partBytes = 64;
      final s = await session(bytes);
      expect(s.mode, UploadSessionModeEnum.presigned);
      final http = HttpClient();
      addTearDown(() => http.close(force: true));
      final req = await http.putUrl(
        h.server.baseUri.resolve(s.parts.single.url),
      );
      req.add(bytes);
      final res = await req.close();
      await res.drain<void>();
      expect(res.statusCode, 200);
      final etag = res.headers.value('etag')!;
      final file = await complete(s.fileId, [
        UploadPartResult(
          (b) => b
            ..partNumber = 1
            ..etag = etag,
        ),
      ]);
      expect(file.uploadState, FileUploadStateEnum.complete);
      expect(h.server.objectStorePuts, hasLength(1));
    },
  );
}

/// A 1 by 1 PNG.
Uint8List _png() => base64.decode(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
);

// SPDX-License-Identifier: Apache-2.0

part of 'world.dart';

/// The settings of the file handlers of the fake (brief M4-22 Scope): the part plan, the mode,
/// the scan, the transcription and the faults "abort after part n", "being checked" and
/// "blocked".
final class FakeFileSettings {
  /// `file.upload_part_bytes` (12 3.1); tests lower it so a small file has several parts.
  int partBytes = 8388608;

  /// The `mode` of new sessions, `direct` or `presigned` (12 3.2, 3.3).
  String mode = 'direct';

  /// Whether a completed upload stays "being checked" (`pending`) until [FilesWorld.finishScan]
  /// (`file_not_ready`, 12 2); otherwise the scan ends at the completion with [scanResult].
  bool holdScan = false;

  /// The `scan_status` a scan ends with: `clean`, `skipped`, `infected` (blocked) or `error`.
  String scanResult = 'clean';

  /// `voice.transcription_mode` (12 8.2): `service` accepts `requestTranscription`, `agent`
  /// and `off` answer `409 conflict` with `reason` `transcription_unavailable`.
  String transcriptionMode = 'agent';

  /// `duration_ms` the fake reports for an `audio/mp4` voice file (the server reads `mvhd`).
  int audioDurationMs = 4000;

  /// The fault "abort after part n": once [abortAfterParts] parts are stored, the next part
  /// answers `503 unavailable` and nothing is stored; then the fault is used up.
  int? abortAfterParts;

  /// The files whose downloads carry a wrong `Repr-Digest` (a damaged transfer).
  final Set<String> wrongDigest = {};

  int _partsStored = 0;
}

/// One stored part of an upload session (12 3.2).
final class FakeUploadPart {
  /// Creates the part.
  const FakeUploadPart(this.number, this.digest, this.etag, this.bytes);

  /// The part number, from 1.
  final int number;

  /// The `Content-Digest` it was sent with.
  final String digest;

  /// The ETag the fake answered: `"<sha256 hex>"` of the part.
  final String etag;

  /// The bytes.
  final Uint8List bytes;
}

/// A file of the fake: the API `File`, its bytes, thumbnails, upload session and transcript.
final class FakeFile {
  /// Creates the file of [json].
  FakeFile(this.json);

  /// The API `File`; changed in place, so the attachments of messages show the current state.
  final JsonMap json;

  /// The joined bytes after the completion, or null before.
  Uint8List? bytes;

  /// The bytes of each thumbnail variant (`small`, `medium`, `large`).
  final Map<String, Uint8List> thumbnails = {};

  /// The session state of 03 `file_upload_sessions`: `open`, `completed`, `aborted`, `expired`.
  String sessionState = 'open';

  /// The part size of the session.
  int partBytes = 0;

  /// The number of parts of the session.
  int partCount = 0;

  /// The declared SHA-256 (lowercase hex).
  String declaredSha256 = '';

  /// The declared type, a hint only (12 6.1).
  String declaredMime = 'application/octet-stream';

  /// The stored parts by number.
  final Map<int, FakeUploadPart> parts = {};

  /// When the open session expires (`file.upload_session_ttl`).
  DateTime expiresAt = DateTime.utc(2100);

  /// The transcript text once `transcript_status = done`.
  String? transcriptText;

  /// The message the file is attached to, or null (12 10 item 3).
  String? attachedTo;

  /// The file ID.
  String get id => json['id']! as String;

  /// The uploader.
  String get uploadedBy => json['uploaded_by']! as String;

  /// The channel of the file.
  String? get channelId => json['scope_channel_id'] as String?;

  /// Whether the server serves the content (12 2: complete, clean or skipped).
  bool get servable =>
      json['upload_state'] == 'complete' &&
      (json['scan_status'] == 'clean' || json['scan_status'] == 'skipped');
}

/// A file ticket of `createFileTicket` (12 9.2): file, variant, principal, login session and
/// expiry.
final class FakeFileTicket {
  /// Creates the ticket.
  const FakeFileTicket({
    required this.fileId,
    required this.variant,
    required this.principalId,
    required this.sessionId,
    required this.expiresAt,
  });

  /// The file.
  final String fileId;

  /// `original` or a thumbnail variant.
  final String variant;

  /// Who asked for it.
  final String principalId;

  /// The login session it is bound to.
  final String sessionId;

  /// The end of its validity (`file.ticket_ttl`).
  final DateTime expiresAt;
}

/// The files of 12: upload sessions with parts (3.1, 3.2), completion with the check and the
/// scan (3.4), the states of 2, thumbnails (7.3), the transcript (8.2), downloads with
/// `Repr-Digest` (9.3), tickets (9.2) and attaching (10). The checks are the visibility and
/// the rules of 12 10, nothing else (README).
///
/// Example:
/// ```dart
/// world.fileSettings
///   ..partBytes = 4
///   ..holdScan = true;
/// // the app uploads; the file shows "being checked"
/// world.finishScan(fileId);
/// ```
extension FilesWorld on FakeWorld {
  /// The file [fileId] when [principalId] may read it: the uploader, or anyone who sees the
  /// thread of the message it is attached to (12 3.7); else `404 not_found`.
  FakeFile visibleFile(String principalId, String fileId) {
    final f = files[fileId];
    if (f == null) throw const FakeProblem(404, 'not_found');
    if (f.uploadedBy == principalId) return f;
    final message = messages[f.attachedTo];
    if (message != null &&
        message['deleted_at'] == null &&
        seesThread(principalId, message['thread_id']! as String)) {
      return f;
    }
    throw const FakeProblem(404, 'not_found');
  }

  FakeFile _ownFile(String principalId, String fileId) {
    final f = files[fileId];
    if (f == null || f.uploadedBy != principalId) {
      throw const FakeProblem(404, 'not_found');
    }
    return f;
  }

  /// `createUpload` (12 3.1): a session for an `UploadCreate` [body] of [principalId] in
  /// [channelId], with the parts of [FakeFileSettings.partBytes].
  ///
  /// Throws `404` for a channel the principal does not see, `403 forbidden` below `write`,
  /// `413 payload_too_large` above `file_max_upload_bytes`.
  JsonMap createUploadSession(
    String principalId,
    String channelId,
    JsonMap body,
  ) {
    if (!seesChannel(principalId, channelId)) {
      throw const FakeProblem(404, 'not_found');
    }
    final level = levelOf(principalId, channelId);
    if (level == 'read') throw const FakeProblem(403, 'forbidden');
    final size = body['size_bytes']! as int;
    if (size > (limits['file_max_upload_bytes']! as int)) {
      throw const FakeProblem(413, 'payload_too_large');
    }
    final id = nextId();
    final name = (body['name']! as String).split(RegExp(r'[/\\]')).last;
    final file = FakeFile({
      'id': id,
      'sha256': body['sha256'],
      'size_bytes': size,
      'mime_type': 'application/octet-stream',
      'original_name': name.isEmpty ? 'file' : name,
      'uploaded_by': principalId,
      'purpose': 'attachment',
      'scope_channel_id': channelId,
      'upload_state': 'pending',
      'scan_status': 'pending',
      'transcript_status': 'none',
      'thumbnails': <Object?>[],
      'created_at': stamp(),
    });
    final settings = fileSettings;
    file
      ..partBytes = settings.partBytes
      ..partCount = (size + settings.partBytes - 1) ~/ settings.partBytes
      ..declaredSha256 = body['sha256']! as String
      ..declaredMime =
          body['declared_mime_type'] as String? ?? 'application/octet-stream'
      ..expiresAt = now().add(const Duration(hours: 24));
    files[id] = file;
    return {
      'file_id': id,
      'mode': settings.mode,
      'part_size_bytes': file.partBytes,
      'parts': [
        for (var n = 1; n <= file.partCount; n++)
          {
            'part_number': n,
            'url': settings.mode == 'presigned'
                ? '/objectstore/$id/$n'
                : '/api/v1/uploads/$id/parts/$n',
            'headers': [
              {'name': 'Content-Type', 'value': 'application/octet-stream'},
            ],
          },
      ],
      'expires_at': stamp(file.expiresAt),
    };
  }

  /// `uploadPart` (12 3.2): stores part [number] of [fileId] after the length and digest
  /// checks; the same part with the same digest answers again without new work.
  ///
  /// Throws `404` for a file of another principal, `410 gone` for an aborted or expired
  /// session, `400 validation_failed` (`part_digest_mismatch`) for a wrong length or digest,
  /// `409 conflict` (`part_conflict`) for a used number with other bytes, and `503
  /// unavailable` for the fault "abort after part n".
  JsonMap putPart(
    String principalId,
    String fileId,
    int number,
    String? contentDigest,
    Uint8List bytes,
  ) {
    final f = _ownFile(principalId, fileId);
    return _storePart(f, number, contentDigest, bytes);
  }

  /// A part of a `presigned` session, PUT to the object store URL of the session (12 3.3);
  /// returns the ETag. Throws like [putPart].
  String putPresignedPart(String fileId, int number, Uint8List bytes) {
    final f = files[fileId];
    if (f == null) throw const FakeProblem(404, 'not_found');
    return _storePart(f, number, null, bytes)['etag']! as String;
  }

  JsonMap _storePart(
    FakeFile f,
    int number,
    String? contentDigest,
    Uint8List bytes,
  ) {
    _expire(f);
    if (f.sessionState == 'aborted' || f.sessionState == 'expired') {
      throw const FakeProblem(410, 'gone');
    }
    if (number < 1 || number > f.partCount) {
      throw const FakeProblem(
        400,
        'validation_failed',
        errors: [
          {'pointer': '/part_number', 'code': 'out_of_range'},
        ],
      );
    }
    final size = f.json['size_bytes']! as int;
    final expected = number < f.partCount
        ? f.partBytes
        : size - f.partBytes * (f.partCount - 1);
    final digest = 'sha-256=:${base64.encode(sha256.convert(bytes).bytes)}:';
    if (bytes.length != expected ||
        (contentDigest != null &&
            !contentDigest.contains(digest.substring(8)))) {
      throw const FakeProblem(
        400,
        'validation_failed',
        errors: [
          {'pointer': '', 'code': 'part_digest_mismatch'},
        ],
      );
    }
    final stored = f.parts[number];
    if (stored != null) {
      if (stored.digest != digest) {
        throw const FakeProblem(409, 'conflict', reason: 'part_conflict');
      }
      return {'part_number': number, 'etag': stored.etag};
    }
    final settings = fileSettings;
    final abortAt = settings.abortAfterParts;
    if (abortAt != null && settings._partsStored >= abortAt) {
      settings.abortAfterParts = null;
      throw const FakeProblem(503, 'unavailable');
    }
    final etag = '"${sha256.convert(bytes)}"';
    f.parts[number] = FakeUploadPart(number, digest, etag, bytes);
    settings._partsStored++;
    return {'part_number': number, 'etag': etag};
  }

  /// `getUpload` (12 3.2 item 5): the confirmed parts of an own session.
  ///
  /// Throws `404` for another principal's file and `410 gone` for an aborted or expired
  /// session.
  JsonMap uploadState(String principalId, String fileId) {
    final f = _ownFile(principalId, fileId);
    _expire(f);
    if (f.sessionState == 'aborted' || f.sessionState == 'expired') {
      throw const FakeProblem(410, 'gone');
    }
    final numbers = f.parts.keys.toList()..sort();
    return {
      'file_id': fileId,
      'upload_state': f.sessionState == 'completed'
          ? 'complete'
          : numbers.isEmpty
          ? 'pending'
          : 'uploading',
      'part_size_bytes': f.partBytes,
      'completed_parts': [
        for (final n in numbers) {'part_number': n, 'etag': f.parts[n]!.etag},
      ],
      'expires_at': stamp(f.expiresAt),
    };
  }

  /// `completeUpload` (12 3.4): checks the listed parts, joins them, checks size and SHA-256
  /// and scans (or holds the scan, [FakeFileSettings.holdScan]); returns the `File`. A
  /// completed session answers the file again.
  ///
  /// Throws `404`, `410 gone`, `409 conflict` with `upload_incomplete` or `part_conflict`.
  JsonMap completeUpload(String principalId, String fileId, JsonMap body) {
    final f = _ownFile(principalId, fileId);
    _expire(f);
    if (f.sessionState == 'completed') return f.json;
    if (f.sessionState == 'aborted' || f.sessionState == 'expired') {
      throw const FakeProblem(410, 'gone');
    }
    for (final p in (body['parts'] as List<Object?>? ?? const [])) {
      final part = p! as JsonMap;
      final stored = f.parts[part['part_number']! as int];
      if (stored != null && stored.etag != part['etag']) {
        throw const FakeProblem(409, 'conflict', reason: 'part_conflict');
      }
    }
    final missing = [
      for (var n = 1; n <= f.partCount; n++)
        if (!f.parts.containsKey(n)) n,
    ];
    if (missing.isNotEmpty) {
      throw FakeProblem(
        409,
        'conflict',
        reason: 'upload_incomplete',
        extra: {'detail': 'missing parts ${missing.join(', ')}'},
      );
    }
    final joined = BytesBuilder(copy: false);
    for (var n = 1; n <= f.partCount; n++) {
      joined.add(f.parts[n]!.bytes);
    }
    final bytes = joined.takeBytes();
    f
      ..sessionState = 'completed'
      ..parts.clear()
      ..bytes = bytes;
    if (sha256.convert(bytes).toString() != f.declaredSha256) {
      f.json['upload_state'] = 'failed';
      _fileUpdated(f);
      return f.json;
    }
    f.json['mime_type'] = _detect(f.declaredMime, bytes);
    if (!fileSettings.holdScan) finishScan(fileId);
    return f.json;
  }

  /// Ends the scan of [fileId] with [result] (default [FakeFileSettings.scanResult]): the
  /// file becomes `complete`; a servable image gets thumbnails and a voice file its duration;
  /// appends `file.updated` (12 3.4).
  void finishScan(String fileId, {String? result}) {
    final f = files[fileId]!;
    _scanned(f, result ?? fileSettings.scanResult);
    _fileUpdated(f);
  }

  void _scanned(FakeFile f, String status) {
    f.json
      ..['upload_state'] = 'complete'
      ..['scan_status'] = status;
    final mime = f.json['mime_type']! as String;
    if (f.servable && mime.startsWith('image/')) {
      final (w, h) = _imageSize(f.bytes!);
      f.json
        ..['width'] = w
        ..['height'] = h
        ..['thumbnails'] = [
          for (final v in const ['small', 'medium'])
            {'variant': v, 'mime_type': mime, 'width': w, 'height': h},
        ];
      for (final v in const ['small', 'medium']) {
        f.thumbnails[v] = f.bytes!;
      }
    }
    if (f.servable && mime.startsWith('audio/')) {
      f.json['duration_ms'] = fileSettings.audioDurationMs;
    }
  }

  /// Sets the transcript of [fileId] to [text] (`transcript_status = done`) and appends
  /// `file.updated` (12 8.2).
  void setTranscript(
    String fileId,
    String text, {
    String language = 'en',
    String source = 'agent',
  }) {
    final f = files[fileId]!;
    f.transcriptText = text;
    f.json
      ..['transcript_status'] = 'done'
      ..['transcript_language'] = language
      ..['transcript_source'] = source;
    _fileUpdated(f);
  }

  /// `abortUpload`: ends an open own session; the file becomes `failed` (12 2).
  ///
  /// Throws `404` and `409 invalid_state_transition` for a completed session.
  void abortUpload(String principalId, String fileId) {
    final f = _ownFile(principalId, fileId);
    if (f.sessionState == 'completed') {
      throw const FakeProblem(409, 'invalid_state_transition');
    }
    if (f.sessionState != 'open') return;
    f
      ..sessionState = 'aborted'
      ..parts.clear();
    f.json['upload_state'] = 'failed';
    _fileUpdated(f);
  }

  /// `deleteFile`: deletes an own file that is not attached; `409 conflict` for an attached
  /// one.
  void deleteOwnFile(String principalId, String fileId) {
    final f = _ownFile(principalId, fileId);
    if (f.attachedTo != null) throw const FakeProblem(409, 'conflict');
    files.remove(fileId);
  }

  /// The bytes and type of [variant] (`original` or a thumbnail) of [fileId] for
  /// [principalId] (12 9.3).
  ///
  /// Throws `404`, `409 file_not_ready` with `Retry-After` while the check runs and
  /// `409 file_blocked` for a blocked or failed file; `404` for a variant the file lacks.
  (Uint8List, String) content(
    String principalId,
    String fileId,
    String variant,
  ) {
    final f = visibleFile(principalId, fileId);
    _requireServable(f);
    if (variant == 'original') {
      return (f.bytes!, f.json['mime_type']! as String);
    }
    final thumb = f.thumbnails[variant];
    if (thumb == null) throw const FakeProblem(404, 'not_found');
    return (thumb, f.json['mime_type']! as String);
  }

  void _requireServable(FakeFile f) {
    if (f.servable) return;
    final failed =
        f.json['upload_state'] == 'failed' ||
        f.json['scan_status'] == 'infected' ||
        f.json['scan_status'] == 'error';
    if (failed) throw const FakeProblem(409, 'file_blocked');
    throw const FakeProblem(
      409,
      'file_not_ready',
      retryAfter: Duration(seconds: 1),
    );
  }

  /// `createFileTicket` (12 9.2): a ticket of [variant] bound to [sessionId], valid for five
  /// minutes; returns the API `FileTicket` with a path below the origin.
  ///
  /// Throws `404` and the `409` problems of [content].
  JsonMap createTicket(
    String principalId,
    String sessionId,
    String fileId,
    String variant,
  ) {
    content(principalId, fileId, variant);
    final token = base64Url
        .encode(utf8.encode('ticket-${nextId()}'))
        .replaceAll('=', '');
    final expires = now().add(const Duration(minutes: 5));
    fileTickets[token] = FakeFileTicket(
      fileId: fileId,
      variant: variant,
      principalId: principalId,
      sessionId: sessionId,
      expiresAt: expires,
    );
    final path = variant == 'original'
        ? '/api/v1/files/$fileId/content'
        : '/api/v1/files/$fileId/thumbnails/$variant';
    return {'url': '$path?ticket=$token', 'expires_at': stamp(expires)};
  }

  /// `getTranscript` (12 8.2): the API `Transcript`; `text` only with `status` `done`.
  ///
  /// Throws `404` and the `409` problems of [content].
  JsonMap transcriptOf(String principalId, String fileId) {
    final f = visibleFile(principalId, fileId);
    _requireServable(f);
    return _transcript(f);
  }

  /// `requestTranscription` (12 8.2): in mode `service` the transcript becomes `pending`;
  /// otherwise `409 conflict` with `reason` `transcription_unavailable`.
  JsonMap requestTranscription(String principalId, String fileId) {
    final f = visibleFile(principalId, fileId);
    _requireServable(f);
    if (fileSettings.transcriptionMode != 'service') {
      throw const FakeProblem(
        409,
        'conflict',
        reason: 'transcription_unavailable',
      );
    }
    if (f.json['transcript_status'] != 'done') {
      f.json['transcript_status'] = 'pending';
      _fileUpdated(f);
    }
    return _transcript(f);
  }

  JsonMap _transcript(FakeFile f) {
    final status = f.json['transcript_status']! as String;
    return {
      'file_id': f.id,
      'status': status,
      if (status == 'done') ...{
        'text': f.transcriptText ?? '',
        'language': ?f.json['transcript_language'],
        'source': ?f.json['transcript_source'],
      },
    };
  }

  /// A file of [uploaderId] in [channelId] that is uploaded and checked already, for tests
  /// that start with attachments: [scanStatus] and [uploadState] set its state (12 2);
  /// returns its ID. Attach it with `createMessage` and `attachment_file_ids`.
  String addReadyFile({
    required String uploaderId,
    required String channelId,
    required String name,
    required String mimeType,
    required List<int> bytes,
    String scanStatus = 'clean',
    String uploadState = 'complete',
  }) {
    final id = nextId();
    final data = Uint8List.fromList(bytes);
    final f = FakeFile({
      'id': id,
      'sha256': sha256.convert(data).toString(),
      'size_bytes': data.length,
      'mime_type': mimeType,
      'original_name': name,
      'uploaded_by': uploaderId,
      'purpose': 'attachment',
      'scope_channel_id': channelId,
      'upload_state': 'pending',
      'scan_status': 'pending',
      'transcript_status': 'none',
      'thumbnails': <Object?>[],
      'created_at': stamp(),
    });
    f
      ..bytes = data
      ..sessionState = 'completed'
      ..declaredSha256 = f.json['sha256']! as String;
    files[id] = f;
    if (uploadState == 'complete') {
      // A seeded file is not news: only its later changes are events.
      _scanned(f, scanStatus);
    } else {
      f.json['upload_state'] = uploadState;
    }
    return id;
  }

  /// The `MessageAttachment`s of [ids] for a message of [authorId] in [channelId] (12 10):
  /// own files of purpose `attachment` in the channel with a completed session, not failed and
  /// not attached yet; at most `message_max_attachments`; a voice message has exactly one
  /// ready `audio/mp4` or `audio/mpeg` file (12 8.1).
  List<JsonMap> _attachable(
    String authorId,
    String channelId,
    List<String> ids, {
    required String kind,
  }) {
    if (ids.length > (limits['message_max_attachments']! as int)) {
      throw const FakeProblem(
        400,
        'validation_failed',
        errors: [
          {'pointer': '/attachment_file_ids', 'code': 'too_many'},
        ],
      );
    }
    final out = <JsonMap>[];
    for (final id in ids) {
      final f = files[id];
      final ok =
          f != null &&
          f.uploadedBy == authorId &&
          f.json['purpose'] == 'attachment' &&
          f.channelId == channelId &&
          f.sessionState == 'completed' &&
          f.json['upload_state'] != 'failed' &&
          f.attachedTo == null;
      if (!ok) {
        throw const FakeProblem(409, 'conflict', reason: 'file_not_attachable');
      }
      out.add({'position': out.length + 1, 'file': f.json});
    }
    if (kind == 'voice') {
      // The file of a voice message must be ready first (12 8.1 rules 1 and 2): `409
      // file_not_ready` with `Retry-After` while the upload or check runs, `409 file_blocked`
      // after a block, as the server answers (FIX-voice-send).
      if (out.length == 1) _requireServable(files[ids.single]!);
      final audio =
          out.length == 1 &&
          {
            'audio/mp4',
            'audio/mpeg',
          }.contains(files[ids.single]!.json['mime_type']);
      if (!audio) {
        throw const FakeProblem(
          400,
          'validation_failed',
          errors: [
            {
              'pointer': '/attachment_file_ids',
              'code': 'voice_format_unsupported',
            },
          ],
        );
      }
    }
    return out;
  }

  void _attach(String messageId, List<JsonMap> attachments) {
    for (final a in attachments) {
      files[(a['file']! as JsonMap)['id']! as String]!.attachedTo = messageId;
    }
  }

  void _expire(FakeFile f) {
    if (f.sessionState == 'open' && !now().isBefore(f.expiresAt)) {
      f.sessionState = 'expired';
      f.json['upload_state'] = 'failed';
    }
  }

  /// Appends `file.updated` (12 2): to the channel once the file is attached, to the uploader
  /// alone before (12 3.7).
  void _fileUpdated(FakeFile f) {
    final channel = f.channelId;
    final space = channel == null
        ? null
        : channels[channel]?.json['space_id'] as String?;
    append(
      'file.updated',
      resource: {'kind': 'file', 'id': f.id},
      scope: {'space_id': ?space, 'channel_id': ?channel},
      payload: {'file': f.json},
      audience: f.attachedTo == null ? f.uploadedBy : null,
    );
  }

  /// The type detection of 12 6.1 in small: the magic numbers of PNG, JPEG, GIF and MP4,
  /// else the declared type.
  static String _detect(String declared, Uint8List b) {
    bool starts(List<int> magic, [int at = 0]) {
      if (b.length < at + magic.length) return false;
      for (var i = 0; i < magic.length; i++) {
        if (b[at + i] != magic[i]) return false;
      }
      return true;
    }

    if (starts(const [0x89, 0x50, 0x4e, 0x47])) return 'image/png';
    if (starts(const [0xff, 0xd8, 0xff])) return 'image/jpeg';
    if (starts(const [0x47, 0x49, 0x46, 0x38])) return 'image/gif';
    if (starts(const [0x66, 0x74, 0x79, 0x70], 4)) {
      return declared.startsWith('video/') ? 'video/mp4' : 'audio/mp4';
    }
    return declared;
  }

  /// Width and height of a PNG (the `IHDR` chunk), else 1 by 1.
  static (int, int) _imageSize(Uint8List b) {
    if (b.length >= 24 && b[0] == 0x89 && b[1] == 0x50) {
      final d = ByteData.sublistView(b);
      return (d.getUint32(16), d.getUint32(20));
    }
    return (1, 1);
  }
}

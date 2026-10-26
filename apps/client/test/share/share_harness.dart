// SPDX-License-Identifier: Apache-2.0

// The fake server with Alex's REST client, an in-memory database and an outbox with the
// message, forum post, upload and attaching executors of the app, plus the folders of an App
// Group and of the outbox's own copies (brief M5-06 Design note 5).

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:client/files/upload_executor.dart';
import 'package:client/share/share_apple.dart';
import 'package:client/share/share_outbox.dart';
import 'package:client/sync/thread_ops.dart';
import 'package:crypto/crypto.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fakes.dart';
import '../support/other_device.dart';
import 'share_fakes.dart';

/// The fake, the outbox and the folders of one share test.
class ShareHarness {
  ShareHarness._(
    this.server,
    this.seed,
    this.alex,
    this.db,
    this.outbox,
    this.root,
    this.shares,
  );

  /// Starts the fake and the outbox (not started; [outbox] `start` sends). The test binding
  /// answers every HTTP request with `400`; the harness lets the requests reach the fake on
  /// the loopback until [stop].
  static Future<ShareHarness> start() async {
    _saved = HttpOverrides.current;
    HttpOverrides.global = null;
    final server = await FakeServer.start();
    final seed = WorldSeed.standard(server.world);
    final alex = OtherDevice(server);
    await alex.signIn();
    final db = openInMemoryForTests();
    final progress = UploadProgress();
    final executors = OutboxExecutors();
    MessageOps.register(executors, alex.auth.api);
    ThreadOps.register(executors, alex.auth.api, (_) async {});
    UploadOps.register(
      executors,
      alex.auth.api,
      progress,
      baseUrl: server.baseUri,
    );
    AttachmentMessageOps.wrap(executors, progress);
    var n = 0;
    final outbox = Outbox(
      db: db,
      executors: executors,
      config: AppConfig.defaults().withOverrides(fastConfig),
      ids: Uuid7Generator(
        nowMillis: () => DateTime.utc(2026, 10, 25).millisecondsSinceEpoch,
        random: (bits) => n++,
      ),
    );
    final root = await Directory.systemTemp.createTemp('share');
    final outgoing = Directory('${root.path}/outgoing');
    final shares = ShareOutbox(
      outbox: outbox,
      entries: db.outboxDao,
      outgoing: () async => outgoing,
    );
    return ShareHarness._(server, seed, alex, db, outbox, root, shares);
  }

  /// The fake.
  final FakeServer server;

  /// Alex, Sam and their channels.
  final WorldSeed seed;

  /// Alex's device.
  final OtherDevice alex;

  /// The local database.
  final LocalDatabase db;

  /// The outbox.
  final Outbox outbox;

  /// The temporary folder of the test.
  final Directory root;

  /// The share path into [outbox].
  final ShareOutbox shares;

  /// The instance of the fake.
  String get instanceId => server.world.instanceId;

  /// The folder of the outbox's own copies.
  Directory get outgoing => Directory('${root.path}/outgoing');

  /// Writes [bytes] as file [n] named [name] of the inbox entry [clientMsgId] in the App
  /// Group folder, as the extension does, and returns its record.
  Future<ShareInboxFileRecord> inboxFile(
    String clientMsgId,
    int n,
    String name,
    List<int> bytes, {
    String mimeType = 'text/plain',
    String? completedFileId,
  }) async {
    final file = File('${root.path}/ShareInbox/$clientMsgId/files/$n-$name');
    await file.create(recursive: true);
    await file.writeAsBytes(bytes, flush: true);
    return ShareInboxFileRecord(
      path: file.path,
      name: name,
      mimeType: mimeType,
      sizeBytes: bytes.length,
      completedFileId: completedFileId,
    );
  }

  /// Uploads [bytes] as Alex into [channelId] through the world and returns the completed
  /// file's ID, as an extension that completed the upload before it lost the network.
  String completedFile(String channelId, String name, List<int> bytes) {
    final w = server.world;
    final session = w.createUploadSession(seed.alex.id, channelId, {
      'name': name,
      'size_bytes': bytes.length,
      'sha256': sha256.convert(bytes).toString(),
      'declared_mime_type': 'text/plain',
    });
    final fileId = session['file_id']! as String;
    final part = w.putPart(
      seed.alex.id,
      fileId,
      1,
      null,
      Uint8List.fromList(bytes),
    );
    w.completeUpload(seed.alex.id, fileId, {
      'parts': [
        {'part_number': 1, 'etag': part['etag']},
      ],
    });
    return fileId;
  }

  /// The messages of [threadId] on the server with [clientMsgId].
  List<Map<String, Object?>> sent(String threadId, String clientMsgId) => [
    for (final m in server.world.messagesOf(threadId))
      if (m['client_msg_id'] == clientMsgId) m,
  ];

  /// Stops the outbox, the database and the fake and deletes [root].
  Future<void> stop() async {
    await outbox.close();
    await db.close();
    await alex.dispose();
    await server.stop();
    if (root.existsSync()) await root.delete(recursive: true);
    HttpOverrides.global = _saved;
  }

  static HttpOverrides? _saved;
}

/// A target record of [kind] in [channelId] (thread [threadId]) for an inbox entry.
ShareTargetRecord targetRecord(
  String kind,
  String channelId, {
  String? threadId,
  String path = 'general > Release notes',
}) => ShareTargetRecord(
  kind: kind,
  channelId: channelId,
  threadId: threadId,
  path: path,
);

/// An inbox entry of the extension.
ShareInboxEntryRecord inboxEntry(
  String clientMsgId, {
  required ShareTargetRecord target,
  String instanceId = shareInstance,
  String comment = 'See error above',
  String? title,
  List<ShareInboxFileRecord> files = const [],
}) => ShareInboxEntryRecord(
  clientMsgId: clientMsgId,
  instanceId: instanceId,
  target: target,
  comment: comment,
  title: title,
  createdAtMillis: DateTime.utc(2026, 10, 25, 9).millisecondsSinceEpoch,
  files: files,
);

/// The bytes of a small shared text file.
List<int> bytesOf(String text) => utf8.encode(text);

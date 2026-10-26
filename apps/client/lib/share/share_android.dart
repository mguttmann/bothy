// SPDX-License-Identifier: Apache-2.0

/// Sharing on Android, the app's side (brief M5-06 Design note 6; 01 5.4; 15 12): the shares
/// the runner received through `ACTION_SEND` and `ACTION_SEND_MULTIPLE`, and their send
/// through the app's outbox.
library;

import 'dart:async';
import 'dart:io';

import '../src/generated/share_android_api.g.dart';
import 'share_outbox.dart';
import 'share_target.dart';

export '../src/generated/share_android_api.g.dart'
    show
        ShareAndroidFlutterApi,
        ShareAndroidHostApi,
        SharedContentRecord,
        SharedFileRecord;

/// What another app shared: text or a link and the files the runner copied into the app's
/// cache.
final class SharedContent {
  /// Creates the content.
  const SharedContent({required this.id, required this.files, this.text});

  /// The content of the runner's [record].
  factory SharedContent.fromRecord(SharedContentRecord record) => SharedContent(
    id: record.id,
    text: record.text,
    files: [
      for (final f in record.files)
        ShareFile(
          path: f.path,
          name: f.name,
          mimeType: f.mimeType,
          sizeBytes: f.sizeBytes,
        ),
    ],
  );

  /// The runner's ID of the share.
  final String id;

  /// `EXTRA_TEXT`, or null.
  final String? text;

  /// The copies in the app's cache, in their order.
  final List<ShareFile> files;
}

/// The share receiver of the Android runner for the app.
///
/// [listen] connects `onShareReceived` and takes the share that started the app; every
/// share goes to [shares], which the app widget turns into the route `/share`. A runner
/// without the channel (widget tests, other platforms) gives nothing.
///
/// Example:
/// ```dart
/// final share = AndroidShare(host: ShareAndroidHostApi());
/// share.shares.listen((content) => router.go('/share'));
/// await share.listen();
/// ```
final class AndroidShare implements ShareAndroidFlutterApi {
  /// Creates the receiver over [host].
  AndroidShare({required ShareAndroidHostApi host})
    : _host = host; // ignore: prefer_initializing_formals

  final ShareAndroidHostApi _host;
  final StreamController<SharedContent> _shares =
      StreamController<SharedContent>();
  Future<void> _taking = Future<void>.value();

  /// Every share taken from the runner; one listener (the app), which also gets the shares
  /// taken before it listened.
  Stream<SharedContent> get shares => _shares.stream;

  /// Connects the runner's calls and takes the pending share.
  Future<void> listen() {
    ShareAndroidFlutterApi.setUp(this);
    return _take();
  }

  @override
  void onShareReceived() => unawaited(_take());

  /// Takes one pending share after the takes before it, so two calls never race for it.
  Future<void> _take() => _taking = _taking.then((_) async {
    try {
      final record = await _host.takePendingShare();
      if (record != null) _shares.add(SharedContent.fromRecord(record));
    } on Object {
      // A host without the runner's channel: no share.
    }
  });

  /// Deletes the cache copies of [content]: after the outbox took its own copies, or when the
  /// person cancels.
  static Future<void> discard(SharedContent content) async {
    for (final f in content.files) {
      try {
        final file = File(f.path);
        if (file.existsSync()) await file.delete();
      } on FileSystemException {
        // The system clears the cache folder on its own.
      }
    }
  }
}

/// Sends a [SharedContent] to a target through the app's outbox ([ShareOutbox]): the uploads,
/// then the message or the new post with a new `client_msg_id`; then the cache copies go.
final class SharedContentSender {
  /// Creates the sender over [outbox]; [newId] makes the `client_msg_id` (a UUIDv7).
  SharedContentSender({required this._outbox, required this._newId});

  final ShareOutbox _outbox;
  final String Function() _newId;

  /// Puts [content] with [comment] (and [title] for a new post) into the outbox for [target]
  /// and returns the `client_msg_id`. Throws a [FileSystemException] when a file cannot be
  /// copied; then nothing is written and the copies stay.
  Future<String> send(
    SharedContent content, {
    required ShareTarget target,
    required String comment,
    String? title,
  }) async {
    final id = _newId();
    await _outbox.enqueue(
      ShareDraft(
        clientMsgId: id,
        target: target,
        comment: comment,
        title: title,
        files: content.files,
      ),
    );
    await AndroidShare.discard(content);
    return id;
  }
}

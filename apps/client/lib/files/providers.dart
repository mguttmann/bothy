// SPDX-License-Identifier: Apache-2.0

/// Riverpod providers of files and voice over the local database, the engine and the ports
/// (docs/spec/15-client-architecture.md 4: one provider per query; brief M4-22). Widgets never
/// query the database or the server themselves.
library;

import 'dart:io';
import 'dart:math';
import 'dart:typed_data';

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../voice/transcript_view.dart';
import 'attach_controller.dart';
import 'download_service.dart';
import 'file_cache.dart';
import 'file_ports.dart';
import 'ticket_player.dart';
import 'upload_executor.dart';

part 'providers.g.dart';

/// The ports of files and voice (`AppPorts.files`).
@riverpod
FilePorts filePorts(Ref ref) => ref.watch(appPortsProvider).files;

/// The file cache of the signed-in instance, or null.
@Riverpod(keepAlive: true)
FileCache? fileCache(Ref ref) => ref.watch(signedInServicesProvider)?.fileCache;

/// The progress of the uploads of this run (the registry of the signed-in instance).
@Riverpod(keepAlive: true)
UploadProgress uploadProgress(Ref ref) =>
    ref.watch(signedInServicesProvider)?.uploads ?? UploadProgress();

/// The downloads of the signed-in instance, or null without an engine or a cache.
@Riverpod(keepAlive: true)
DownloadService? downloadService(Ref ref) {
  final engine = ref.watch(syncEngineProvider);
  final cache = ref.watch(fileCacheProvider);
  if (engine == null || cache == null) return null;
  return DownloadService(api: engine.api, cache: cache);
}

/// The file tickets of the signed-in instance (12 9.2), or null.
@Riverpod(keepAlive: true)
FileTickets? fileTickets(Ref ref) {
  final engine = ref.watch(syncEngineProvider);
  final instance = ref.watch(currentInstanceProvider);
  if (engine == null || instance == null) return null;
  return FileTickets(engine.api, instance.address);
}

/// The stored metadata of [fileId] on every `file.updated` (12 2), null while unknown.
@riverpod
Stream<FileMeta?> fileMeta(Ref ref, String fileId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(null)
      : db.messagesDao.watchFile(FileId.parse(fileId));
}

/// The files attached to [messageId] in their order.
@riverpod
Stream<List<FileMeta>> messageFiles(Ref ref, String messageId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.messagesDao.watchAttachedFiles(MessageId.parse(messageId));
}

/// The files of the messages of [threadId], newest first (the section "Files", 01 5.1 D1).
@riverpod
Stream<List<AttachedFile>> threadFiles(Ref ref, String threadId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.messagesDao.watchThreadFiles(ThreadId.parse(threadId));
}

/// The thumbnail [variant] of [fileId] from the server through the cache (SEC-OUT-2): loaded
/// again whenever the file's metadata changes, so "being checked" turns into the image on
/// `file.updated` (12 2, brief M4-22 Design note 3).
@riverpod
Future<DownloadResult?> thumbnail(
  Ref ref,
  String fileId,
  String variant,
) async {
  final meta = ref.watch(fileMetaProvider(fileId)).value;
  final downloads = ref.watch(downloadServiceProvider);
  if (downloads == null || meta == null) return null;
  return downloads.fetch(meta.id, variant: variant);
}

/// The transcript operations of the signed-in instance (12 8.2), or null.
@Riverpod(keepAlive: true)
FileTranscripts? fileTranscripts(Ref ref) {
  final engine = ref.watch(syncEngineProvider);
  return engine == null ? null : FileTranscripts(engine.api);
}

/// Whether `voice.enabled` is on for the person in the stored flags (`features` of the instance
/// row, which `getInstance` and every snapshot refresh); false while unknown.
@Riverpod(keepAlive: true)
Stream<bool> voiceEnabled(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(false)
      : db.instanceDao
            .watch()
            .map((i) => i?.features['voice.enabled'] ?? false)
            .distinct();
}

/// Whether the server transcribes a voice message on request for the person (12 8.2, issue
/// 570): `voice.enabled` on and `getInstance` reporting `voice_transcription_mode = service`.
/// The view offers "Transcribe" only then. With the switch off nothing is asked; a change of the
/// switch asks again, because the effective mode depends on it.
@Riverpod(keepAlive: true)
Future<bool> serverTranscribes(Ref ref) async {
  final on = await ref.watch(voiceEnabledProvider.future);
  final transcripts = ref.watch(fileTranscriptsProvider);
  if (!on || transcripts == null) return false;
  return await transcripts.serverMode() == 'service';
}

/// The transcript text of [fileId] once `file.updated` reports `transcript_status = done`
/// (12 8.2); null before. Read again on every change of the file.
@riverpod
Future<String?> transcriptText(Ref ref, String fileId) async {
  final meta = ref.watch(fileMetaProvider(fileId)).value;
  final transcripts = ref.watch(fileTranscriptsProvider);
  if (meta == null ||
      transcripts == null ||
      meta.transcriptStatus != TranscriptStatus.done) {
    return null;
  }
  return transcripts.text(meta.id);
}

/// Makes the attachment keys of the composers: UUIDv7 of the instance, or of a secure source
/// of their own without one (tests).
@Riverpod(keepAlive: true)
String Function() attachmentKeys(Ref ref) {
  final ids = ref.watch(signedInServicesProvider)?.ids;
  if (ids != null) return ids.next;
  final random = Random.secure();
  return Uuid7Generator(
    nowMillis: () => DateTime.now().millisecondsSinceEpoch,
    random: (bits) {
      var value = 0;
      for (var left = bits; left > 0; left -= 30) {
        final take = left > 30 ? 30 : left;
        value = (value << take) | random.nextInt(1 << take);
      }
      return value;
    },
  ).next;
}

/// The folder of the files the app writes before their upload (recordings, pasted images):
/// `<instance folder>/outgoing`, which the wipe deletes with the instance (D-M4-22-5).
@Riverpod(keepAlive: true)
Directory outgoingFolder(Ref ref) {
  final cache = ref.watch(fileCacheProvider);
  return cache == null
      ? Directory(
          '${Directory.systemTemp.path}${Platform.pathSeparator}outgoing',
        )
      : Directory('${cache.root.parent.path}${Platform.pathSeparator}outgoing');
}

/// The bytes of the cached file at [path] (a thumbnail the server sent, checked against
/// `Repr-Digest`), for `Image.memory`; read once per path while shown.
@riverpod
Future<Uint8List> cachedBytes(Ref ref, String path) => File(path).readAsBytes();

/// The attachments of the composer of [threadId] in [channelId], or null without an engine.
/// They last while the app runs, so files attached and not sent yet are still there when the
/// person comes back to the thread (brief M4-22 Design note 2).
@Riverpod(keepAlive: true)
AttachController? composerAttachments(
  Ref ref,
  String threadId,
  String channelId,
) {
  final engine = ref.watch(syncEngineProvider);
  if (engine == null) return null;
  final attach = AttachController(
    thread: ThreadId.parse(threadId),
    channel: ChannelId.parse(channelId),
    outbox: engine.outbox,
    progress: ref.watch(uploadProgressProvider),
    newKey: ref.watch(attachmentKeysProvider),
    outgoing: () async => ref.read(outgoingFolderProvider),
  );
  ref.onDispose(attach.dispose);
  return attach;
}

/// The folder the files of new posts wait in until their send (FEAT-NEW-CHAT-ATTACH, ruling
/// FNCA/2): `<instance folder>/staged`, one subfolder per forum, which the wipe deletes with
/// the instance (D-M4-22-5).
///
/// Staged files live while the app runs, as the attachments of a thread's composer do (brief
/// M4-22 Design note 2); nothing of them is in the outbox or on the server. So what an earlier
/// run left there is deleted when the folder is first used in a run, before any file of this
/// run is staged. Without an instance folder (tests) it lies in the temporary folder, as the
/// outgoing folder does, and is not emptied.
@Riverpod(keepAlive: true)
Future<Directory> stagedFolder(Ref ref) async {
  final cache = ref.watch(fileCacheProvider);
  if (cache == null) {
    return Directory(
      '${Directory.systemTemp.path}${Platform.pathSeparator}staged',
    );
  }
  final dir = Directory(
    '${cache.root.parent.path}${Platform.pathSeparator}staged',
  );
  try {
    if (dir.existsSync()) await dir.delete(recursive: true);
  } on FileSystemException {
    // A file that cannot be deleted now goes with the instance folder.
  }
  return dir;
}

/// The attachments of the first message of a new post in the forum [forumId] (the new chat
/// view, M4-47, FEAT-NEW-CHAT-ATTACH), or null without an engine; kept while the app runs, as
/// a thread's. The files are staged in `staged/<forum>` until the send (FNCA/2), and the
/// staged folder is made ready (emptied of an earlier run) when the view first opens.
@Riverpod(keepAlive: true)
AttachController? newPostAttachments(Ref ref, String forumId) {
  final engine = ref.watch(syncEngineProvider);
  if (engine == null) return null;
  // The view opening makes the staged folder ready: what an earlier run left there goes.
  final ready = ref.read(stagedFolderProvider.future);
  final attach = AttachController(
    thread: null,
    channel: ChannelId.parse(forumId),
    outbox: engine.outbox,
    progress: ref.watch(uploadProgressProvider),
    newKey: ref.watch(attachmentKeysProvider),
    outgoing: () async => ref.read(outgoingFolderProvider),
    staging: () async {
      await ready;
      final staged = await ref.read(stagedFolderProvider.future);
      return Directory('${staged.path}${Platform.pathSeparator}$forumId');
    },
  );
  ref.onDispose(attach.dispose);
  return attach;
}

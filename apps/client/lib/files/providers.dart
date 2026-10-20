// SPDX-License-Identifier: Apache-2.0

/// Riverpod providers of files and voice over the local database, the engine and the ports
/// (docs/spec/15-client-architektur.md 4: one provider per query; brief M4-22). Widgets never
/// query the database or the server themselves.
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
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

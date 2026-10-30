// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The ports of files and voice (`AppPorts.files`).

@ProviderFor(filePorts)
final filePortsProvider = FilePortsProvider._();

/// The ports of files and voice (`AppPorts.files`).

final class FilePortsProvider
    extends $FunctionalProvider<FilePorts, FilePorts, FilePorts>
    with $Provider<FilePorts> {
  /// The ports of files and voice (`AppPorts.files`).
  FilePortsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'filePortsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$filePortsHash();

  @$internal
  @override
  $ProviderElement<FilePorts> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  FilePorts create(Ref ref) {
    return filePorts(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(FilePorts value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<FilePorts>(value),
    );
  }
}

String _$filePortsHash() => r'f1151e630d3986dbe368fd3e0a3a7fe36a62ad25';

/// The file cache of the signed-in instance, or null.

@ProviderFor(fileCache)
final fileCacheProvider = FileCacheProvider._();

/// The file cache of the signed-in instance, or null.

final class FileCacheProvider
    extends $FunctionalProvider<FileCache?, FileCache?, FileCache?>
    with $Provider<FileCache?> {
  /// The file cache of the signed-in instance, or null.
  FileCacheProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'fileCacheProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$fileCacheHash();

  @$internal
  @override
  $ProviderElement<FileCache?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  FileCache? create(Ref ref) {
    return fileCache(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(FileCache? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<FileCache?>(value),
    );
  }
}

String _$fileCacheHash() => r'8d50a88ac7ad024ec266821e815f86b359218200';

/// The progress of the uploads of this run (the registry of the signed-in instance).

@ProviderFor(uploadProgress)
final uploadProgressProvider = UploadProgressProvider._();

/// The progress of the uploads of this run (the registry of the signed-in instance).

final class UploadProgressProvider
    extends $FunctionalProvider<UploadProgress, UploadProgress, UploadProgress>
    with $Provider<UploadProgress> {
  /// The progress of the uploads of this run (the registry of the signed-in instance).
  UploadProgressProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'uploadProgressProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$uploadProgressHash();

  @$internal
  @override
  $ProviderElement<UploadProgress> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  UploadProgress create(Ref ref) {
    return uploadProgress(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(UploadProgress value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<UploadProgress>(value),
    );
  }
}

String _$uploadProgressHash() => r'ac38c9416a4e30284fe5d7fbcbcbda4452c7d882';

/// The downloads of the signed-in instance, or null without an engine or a cache.

@ProviderFor(downloadService)
final downloadServiceProvider = DownloadServiceProvider._();

/// The downloads of the signed-in instance, or null without an engine or a cache.

final class DownloadServiceProvider
    extends
        $FunctionalProvider<
          DownloadService?,
          DownloadService?,
          DownloadService?
        >
    with $Provider<DownloadService?> {
  /// The downloads of the signed-in instance, or null without an engine or a cache.
  DownloadServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'downloadServiceProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$downloadServiceHash();

  @$internal
  @override
  $ProviderElement<DownloadService?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  DownloadService? create(Ref ref) {
    return downloadService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DownloadService? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DownloadService?>(value),
    );
  }
}

String _$downloadServiceHash() => r'681c0fa9764ac949896024c3cc5f1cbb46081b2e';

/// The file tickets of the signed-in instance (12 9.2), or null.

@ProviderFor(fileTickets)
final fileTicketsProvider = FileTicketsProvider._();

/// The file tickets of the signed-in instance (12 9.2), or null.

final class FileTicketsProvider
    extends $FunctionalProvider<FileTickets?, FileTickets?, FileTickets?>
    with $Provider<FileTickets?> {
  /// The file tickets of the signed-in instance (12 9.2), or null.
  FileTicketsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'fileTicketsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$fileTicketsHash();

  @$internal
  @override
  $ProviderElement<FileTickets?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  FileTickets? create(Ref ref) {
    return fileTickets(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(FileTickets? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<FileTickets?>(value),
    );
  }
}

String _$fileTicketsHash() => r'5f549381aa34cefb59d1f8f8549bed6c1e79b696';

/// The stored metadata of [fileId] on every `file.updated` (12 2), null while unknown.

@ProviderFor(fileMeta)
final fileMetaProvider = FileMetaFamily._();

/// The stored metadata of [fileId] on every `file.updated` (12 2), null while unknown.

final class FileMetaProvider
    extends
        $FunctionalProvider<AsyncValue<FileMeta?>, FileMeta?, Stream<FileMeta?>>
    with $FutureModifier<FileMeta?>, $StreamProvider<FileMeta?> {
  /// The stored metadata of [fileId] on every `file.updated` (12 2), null while unknown.
  FileMetaProvider._({
    required FileMetaFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'fileMetaProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$fileMetaHash();

  @override
  String toString() {
    return r'fileMetaProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<FileMeta?> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<FileMeta?> create(Ref ref) {
    final argument = this.argument as String;
    return fileMeta(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is FileMetaProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$fileMetaHash() => r'9fafad700b9876873396f49d993664b34f5ba02e';

/// The stored metadata of [fileId] on every `file.updated` (12 2), null while unknown.

final class FileMetaFamily extends $Family
    with $FunctionalFamilyOverride<Stream<FileMeta?>, String> {
  FileMetaFamily._()
    : super(
        retry: null,
        name: r'fileMetaProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The stored metadata of [fileId] on every `file.updated` (12 2), null while unknown.

  FileMetaProvider call(String fileId) =>
      FileMetaProvider._(argument: fileId, from: this);

  @override
  String toString() => r'fileMetaProvider';
}

/// The files attached to [messageId] in their order.

@ProviderFor(messageFiles)
final messageFilesProvider = MessageFilesFamily._();

/// The files attached to [messageId] in their order.

final class MessageFilesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<FileMeta>>,
          List<FileMeta>,
          Stream<List<FileMeta>>
        >
    with $FutureModifier<List<FileMeta>>, $StreamProvider<List<FileMeta>> {
  /// The files attached to [messageId] in their order.
  MessageFilesProvider._({
    required MessageFilesFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'messageFilesProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$messageFilesHash();

  @override
  String toString() {
    return r'messageFilesProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<FileMeta>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<FileMeta>> create(Ref ref) {
    final argument = this.argument as String;
    return messageFiles(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is MessageFilesProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$messageFilesHash() => r'ebd8e6335bbc42185b5f06b426d189e330e2e5d8';

/// The files attached to [messageId] in their order.

final class MessageFilesFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<FileMeta>>, String> {
  MessageFilesFamily._()
    : super(
        retry: null,
        name: r'messageFilesProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The files attached to [messageId] in their order.

  MessageFilesProvider call(String messageId) =>
      MessageFilesProvider._(argument: messageId, from: this);

  @override
  String toString() => r'messageFilesProvider';
}

/// The files of the messages of [threadId], newest first (the section "Files", 01 5.1 D1).

@ProviderFor(threadFiles)
final threadFilesProvider = ThreadFilesFamily._();

/// The files of the messages of [threadId], newest first (the section "Files", 01 5.1 D1).

final class ThreadFilesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<AttachedFile>>,
          List<AttachedFile>,
          Stream<List<AttachedFile>>
        >
    with
        $FutureModifier<List<AttachedFile>>,
        $StreamProvider<List<AttachedFile>> {
  /// The files of the messages of [threadId], newest first (the section "Files", 01 5.1 D1).
  ThreadFilesProvider._({
    required ThreadFilesFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'threadFilesProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$threadFilesHash();

  @override
  String toString() {
    return r'threadFilesProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<AttachedFile>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<AttachedFile>> create(Ref ref) {
    final argument = this.argument as String;
    return threadFiles(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ThreadFilesProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$threadFilesHash() => r'65cd87d52196bdde8d62dd4b78e9e350655a19bf';

/// The files of the messages of [threadId], newest first (the section "Files", 01 5.1 D1).

final class ThreadFilesFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<AttachedFile>>, String> {
  ThreadFilesFamily._()
    : super(
        retry: null,
        name: r'threadFilesProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The files of the messages of [threadId], newest first (the section "Files", 01 5.1 D1).

  ThreadFilesProvider call(String threadId) =>
      ThreadFilesProvider._(argument: threadId, from: this);

  @override
  String toString() => r'threadFilesProvider';
}

/// The thumbnail [variant] of [fileId] from the server through the cache (SEC-OUT-2): loaded
/// again whenever the file's metadata changes, so "being checked" turns into the image on
/// `file.updated` (12 2, brief M4-22 Design note 3).

@ProviderFor(thumbnail)
final thumbnailProvider = ThumbnailFamily._();

/// The thumbnail [variant] of [fileId] from the server through the cache (SEC-OUT-2): loaded
/// again whenever the file's metadata changes, so "being checked" turns into the image on
/// `file.updated` (12 2, brief M4-22 Design note 3).

final class ThumbnailProvider
    extends
        $FunctionalProvider<
          AsyncValue<DownloadResult?>,
          DownloadResult?,
          FutureOr<DownloadResult?>
        >
    with $FutureModifier<DownloadResult?>, $FutureProvider<DownloadResult?> {
  /// The thumbnail [variant] of [fileId] from the server through the cache (SEC-OUT-2): loaded
  /// again whenever the file's metadata changes, so "being checked" turns into the image on
  /// `file.updated` (12 2, brief M4-22 Design note 3).
  ThumbnailProvider._({
    required ThumbnailFamily super.from,
    required (String, String) super.argument,
  }) : super(
         retry: null,
         name: r'thumbnailProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$thumbnailHash();

  @override
  String toString() {
    return r'thumbnailProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  $FutureProviderElement<DownloadResult?> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<DownloadResult?> create(Ref ref) {
    final argument = this.argument as (String, String);
    return thumbnail(ref, argument.$1, argument.$2);
  }

  @override
  bool operator ==(Object other) {
    return other is ThumbnailProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$thumbnailHash() => r'59df7fecf046c2f24c69e9de29c565222446d6a7';

/// The thumbnail [variant] of [fileId] from the server through the cache (SEC-OUT-2): loaded
/// again whenever the file's metadata changes, so "being checked" turns into the image on
/// `file.updated` (12 2, brief M4-22 Design note 3).

final class ThumbnailFamily extends $Family
    with
        $FunctionalFamilyOverride<FutureOr<DownloadResult?>, (String, String)> {
  ThumbnailFamily._()
    : super(
        retry: null,
        name: r'thumbnailProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The thumbnail [variant] of [fileId] from the server through the cache (SEC-OUT-2): loaded
  /// again whenever the file's metadata changes, so "being checked" turns into the image on
  /// `file.updated` (12 2, brief M4-22 Design note 3).

  ThumbnailProvider call(String fileId, String variant) =>
      ThumbnailProvider._(argument: (fileId, variant), from: this);

  @override
  String toString() => r'thumbnailProvider';
}

/// The transcript operations of the signed-in instance (12 8.2), or null.

@ProviderFor(fileTranscripts)
final fileTranscriptsProvider = FileTranscriptsProvider._();

/// The transcript operations of the signed-in instance (12 8.2), or null.

final class FileTranscriptsProvider
    extends
        $FunctionalProvider<
          FileTranscripts?,
          FileTranscripts?,
          FileTranscripts?
        >
    with $Provider<FileTranscripts?> {
  /// The transcript operations of the signed-in instance (12 8.2), or null.
  FileTranscriptsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'fileTranscriptsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$fileTranscriptsHash();

  @$internal
  @override
  $ProviderElement<FileTranscripts?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  FileTranscripts? create(Ref ref) {
    return fileTranscripts(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(FileTranscripts? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<FileTranscripts?>(value),
    );
  }
}

String _$fileTranscriptsHash() => r'e533b9184d66627bdd3b1321256570aa45559a95';

/// The transcript text of [fileId] once `file.updated` reports `transcript_status = done`
/// (12 8.2); null before. Read again on every change of the file.

@ProviderFor(transcriptText)
final transcriptTextProvider = TranscriptTextFamily._();

/// The transcript text of [fileId] once `file.updated` reports `transcript_status = done`
/// (12 8.2); null before. Read again on every change of the file.

final class TranscriptTextProvider
    extends $FunctionalProvider<AsyncValue<String?>, String?, FutureOr<String?>>
    with $FutureModifier<String?>, $FutureProvider<String?> {
  /// The transcript text of [fileId] once `file.updated` reports `transcript_status = done`
  /// (12 8.2); null before. Read again on every change of the file.
  TranscriptTextProvider._({
    required TranscriptTextFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'transcriptTextProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$transcriptTextHash();

  @override
  String toString() {
    return r'transcriptTextProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<String?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<String?> create(Ref ref) {
    final argument = this.argument as String;
    return transcriptText(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is TranscriptTextProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$transcriptTextHash() => r'6d9d0f935bc622e90e175108f0a7c0980e68d059';

/// The transcript text of [fileId] once `file.updated` reports `transcript_status = done`
/// (12 8.2); null before. Read again on every change of the file.

final class TranscriptTextFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<String?>, String> {
  TranscriptTextFamily._()
    : super(
        retry: null,
        name: r'transcriptTextProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The transcript text of [fileId] once `file.updated` reports `transcript_status = done`
  /// (12 8.2); null before. Read again on every change of the file.

  TranscriptTextProvider call(String fileId) =>
      TranscriptTextProvider._(argument: fileId, from: this);

  @override
  String toString() => r'transcriptTextProvider';
}

/// Makes the attachment keys of the composers: UUIDv7 of the instance, or of a secure source
/// of their own without one (tests).

@ProviderFor(attachmentKeys)
final attachmentKeysProvider = AttachmentKeysProvider._();

/// Makes the attachment keys of the composers: UUIDv7 of the instance, or of a secure source
/// of their own without one (tests).

final class AttachmentKeysProvider
    extends
        $FunctionalProvider<
          String Function(),
          String Function(),
          String Function()
        >
    with $Provider<String Function()> {
  /// Makes the attachment keys of the composers: UUIDv7 of the instance, or of a secure source
  /// of their own without one (tests).
  AttachmentKeysProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'attachmentKeysProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$attachmentKeysHash();

  @$internal
  @override
  $ProviderElement<String Function()> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  String Function() create(Ref ref) {
    return attachmentKeys(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String Function() value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String Function()>(value),
    );
  }
}

String _$attachmentKeysHash() => r'657867e936669f002387f3c5d055a7689d55ac0d';

/// The folder of the files the app writes before their upload (recordings, pasted images):
/// `<instance folder>/outgoing`, which the wipe deletes with the instance (D-M4-22-5).

@ProviderFor(outgoingFolder)
final outgoingFolderProvider = OutgoingFolderProvider._();

/// The folder of the files the app writes before their upload (recordings, pasted images):
/// `<instance folder>/outgoing`, which the wipe deletes with the instance (D-M4-22-5).

final class OutgoingFolderProvider
    extends $FunctionalProvider<Directory, Directory, Directory>
    with $Provider<Directory> {
  /// The folder of the files the app writes before their upload (recordings, pasted images):
  /// `<instance folder>/outgoing`, which the wipe deletes with the instance (D-M4-22-5).
  OutgoingFolderProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'outgoingFolderProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$outgoingFolderHash();

  @$internal
  @override
  $ProviderElement<Directory> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  Directory create(Ref ref) {
    return outgoingFolder(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Directory value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Directory>(value),
    );
  }
}

String _$outgoingFolderHash() => r'a27677837bba0d4ae67b435915dbba1f3f21c9d5';

/// The bytes of the cached file at [path] (a thumbnail the server sent, checked against
/// `Repr-Digest`), for `Image.memory`; read once per path while shown.

@ProviderFor(cachedBytes)
final cachedBytesProvider = CachedBytesFamily._();

/// The bytes of the cached file at [path] (a thumbnail the server sent, checked against
/// `Repr-Digest`), for `Image.memory`; read once per path while shown.

final class CachedBytesProvider
    extends
        $FunctionalProvider<
          AsyncValue<Uint8List>,
          Uint8List,
          FutureOr<Uint8List>
        >
    with $FutureModifier<Uint8List>, $FutureProvider<Uint8List> {
  /// The bytes of the cached file at [path] (a thumbnail the server sent, checked against
  /// `Repr-Digest`), for `Image.memory`; read once per path while shown.
  CachedBytesProvider._({
    required CachedBytesFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'cachedBytesProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$cachedBytesHash();

  @override
  String toString() {
    return r'cachedBytesProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<Uint8List> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<Uint8List> create(Ref ref) {
    final argument = this.argument as String;
    return cachedBytes(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is CachedBytesProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$cachedBytesHash() => r'529ba510322cfcbd35032ce04f11516b6756b7b4';

/// The bytes of the cached file at [path] (a thumbnail the server sent, checked against
/// `Repr-Digest`), for `Image.memory`; read once per path while shown.

final class CachedBytesFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<Uint8List>, String> {
  CachedBytesFamily._()
    : super(
        retry: null,
        name: r'cachedBytesProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The bytes of the cached file at [path] (a thumbnail the server sent, checked against
  /// `Repr-Digest`), for `Image.memory`; read once per path while shown.

  CachedBytesProvider call(String path) =>
      CachedBytesProvider._(argument: path, from: this);

  @override
  String toString() => r'cachedBytesProvider';
}

/// The attachments of the composer of [threadId] in [channelId], or null without an engine.
/// They last while the app runs, so files attached and not sent yet are still there when the
/// person comes back to the thread (brief M4-22 Design note 2).

@ProviderFor(composerAttachments)
final composerAttachmentsProvider = ComposerAttachmentsFamily._();

/// The attachments of the composer of [threadId] in [channelId], or null without an engine.
/// They last while the app runs, so files attached and not sent yet are still there when the
/// person comes back to the thread (brief M4-22 Design note 2).

final class ComposerAttachmentsProvider
    extends
        $FunctionalProvider<
          AttachController?,
          AttachController?,
          AttachController?
        >
    with $Provider<AttachController?> {
  /// The attachments of the composer of [threadId] in [channelId], or null without an engine.
  /// They last while the app runs, so files attached and not sent yet are still there when the
  /// person comes back to the thread (brief M4-22 Design note 2).
  ComposerAttachmentsProvider._({
    required ComposerAttachmentsFamily super.from,
    required (String, String) super.argument,
  }) : super(
         retry: null,
         name: r'composerAttachmentsProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$composerAttachmentsHash();

  @override
  String toString() {
    return r'composerAttachmentsProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  $ProviderElement<AttachController?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AttachController? create(Ref ref) {
    final argument = this.argument as (String, String);
    return composerAttachments(ref, argument.$1, argument.$2);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AttachController? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AttachController?>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ComposerAttachmentsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$composerAttachmentsHash() =>
    r'102e5a9857964edc98fb3437487964f3403f3630';

/// The attachments of the composer of [threadId] in [channelId], or null without an engine.
/// They last while the app runs, so files attached and not sent yet are still there when the
/// person comes back to the thread (brief M4-22 Design note 2).

final class ComposerAttachmentsFamily extends $Family
    with $FunctionalFamilyOverride<AttachController?, (String, String)> {
  ComposerAttachmentsFamily._()
    : super(
        retry: null,
        name: r'composerAttachmentsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// The attachments of the composer of [threadId] in [channelId], or null without an engine.
  /// They last while the app runs, so files attached and not sent yet are still there when the
  /// person comes back to the thread (brief M4-22 Design note 2).

  ComposerAttachmentsProvider call(String threadId, String channelId) =>
      ComposerAttachmentsProvider._(
        argument: (threadId, channelId),
        from: this,
      );

  @override
  String toString() => r'composerAttachmentsProvider';
}

/// The attachments of the first message of a new post in the forum [forumId] (the new chat
/// view, M4-47), or null without an engine; kept while the app runs, as a thread's.

@ProviderFor(newPostAttachments)
final newPostAttachmentsProvider = NewPostAttachmentsFamily._();

/// The attachments of the first message of a new post in the forum [forumId] (the new chat
/// view, M4-47), or null without an engine; kept while the app runs, as a thread's.

final class NewPostAttachmentsProvider
    extends
        $FunctionalProvider<
          AttachController?,
          AttachController?,
          AttachController?
        >
    with $Provider<AttachController?> {
  /// The attachments of the first message of a new post in the forum [forumId] (the new chat
  /// view, M4-47), or null without an engine; kept while the app runs, as a thread's.
  NewPostAttachmentsProvider._({
    required NewPostAttachmentsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'newPostAttachmentsProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$newPostAttachmentsHash();

  @override
  String toString() {
    return r'newPostAttachmentsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $ProviderElement<AttachController?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AttachController? create(Ref ref) {
    final argument = this.argument as String;
    return newPostAttachments(ref, argument);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AttachController? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AttachController?>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is NewPostAttachmentsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$newPostAttachmentsHash() =>
    r'ba204a681eb288656aa7ec79a56da49dea2e8b48';

/// The attachments of the first message of a new post in the forum [forumId] (the new chat
/// view, M4-47), or null without an engine; kept while the app runs, as a thread's.

final class NewPostAttachmentsFamily extends $Family
    with $FunctionalFamilyOverride<AttachController?, String> {
  NewPostAttachmentsFamily._()
    : super(
        retry: null,
        name: r'newPostAttachmentsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// The attachments of the first message of a new post in the forum [forumId] (the new chat
  /// view, M4-47), or null without an engine; kept while the app runs, as a thread's.

  NewPostAttachmentsProvider call(String forumId) =>
      NewPostAttachmentsProvider._(argument: forumId, from: this);

  @override
  String toString() => r'newPostAttachmentsProvider';
}

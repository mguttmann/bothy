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

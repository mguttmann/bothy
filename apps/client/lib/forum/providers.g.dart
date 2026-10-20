// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The channel [channelId], or null when it is not stored.

@ProviderFor(storedChannel)
final storedChannelProvider = StoredChannelFamily._();

/// The channel [channelId], or null when it is not stored.

final class StoredChannelProvider
    extends
        $FunctionalProvider<AsyncValue<Channel?>, Channel?, FutureOr<Channel?>>
    with $FutureModifier<Channel?>, $FutureProvider<Channel?> {
  /// The channel [channelId], or null when it is not stored.
  StoredChannelProvider._({
    required StoredChannelFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'storedChannelProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$storedChannelHash();

  @override
  String toString() {
    return r'storedChannelProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<Channel?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<Channel?> create(Ref ref) {
    final argument = this.argument as String;
    return storedChannel(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is StoredChannelProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$storedChannelHash() => r'2ccb9e4be9ae053fd28550aa6ff7b447208f8873';

/// The channel [channelId], or null when it is not stored.

final class StoredChannelFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<Channel?>, String> {
  StoredChannelFamily._()
    : super(
        retry: null,
        name: r'storedChannelProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The channel [channelId], or null when it is not stored.

  StoredChannelProvider call(String channelId) =>
      StoredChannelProvider._(argument: channelId, from: this);

  @override
  String toString() => r'storedChannelProvider';
}

/// The stored threads of [channelId].

@ProviderFor(forumThreads)
final forumThreadsProvider = ForumThreadsFamily._();

/// The stored threads of [channelId].

final class ForumThreadsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Thread>>,
          List<Thread>,
          Stream<List<Thread>>
        >
    with $FutureModifier<List<Thread>>, $StreamProvider<List<Thread>> {
  /// The stored threads of [channelId].
  ForumThreadsProvider._({
    required ForumThreadsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'forumThreadsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$forumThreadsHash();

  @override
  String toString() {
    return r'forumThreadsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<Thread>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<Thread>> create(Ref ref) {
    final argument = this.argument as String;
    return forumThreads(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ForumThreadsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$forumThreadsHash() => r'c4998477dc927b00976bcfdcb44ef4d44c3d55a6';

/// The stored threads of [channelId].

final class ForumThreadsFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<Thread>>, String> {
  ForumThreadsFamily._()
    : super(
        retry: null,
        name: r'forumThreadsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The stored threads of [channelId].

  ForumThreadsProvider call(String channelId) =>
      ForumThreadsProvider._(argument: channelId, from: this);

  @override
  String toString() => r'forumThreadsProvider';
}

/// The tags of the forum [channelId] in their order.

@ProviderFor(forumTags)
final forumTagsProvider = ForumTagsFamily._();

/// The tags of the forum [channelId] in their order.

final class ForumTagsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<ForumTag>>,
          List<ForumTag>,
          Stream<List<ForumTag>>
        >
    with $FutureModifier<List<ForumTag>>, $StreamProvider<List<ForumTag>> {
  /// The tags of the forum [channelId] in their order.
  ForumTagsProvider._({
    required ForumTagsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'forumTagsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$forumTagsHash();

  @override
  String toString() {
    return r'forumTagsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<ForumTag>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<ForumTag>> create(Ref ref) {
    final argument = this.argument as String;
    return forumTags(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ForumTagsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$forumTagsHash() => r'a711b3014943930ed77a86a598ac3980d0a97e15';

/// The tags of the forum [channelId] in their order.

final class ForumTagsFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<ForumTag>>, String> {
  ForumTagsFamily._()
    : super(
        retry: null,
        name: r'forumTagsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The tags of the forum [channelId] in their order.

  ForumTagsProvider call(String channelId) =>
      ForumTagsProvider._(argument: channelId, from: this);

  @override
  String toString() => r'forumTagsProvider';
}

/// The stored runs of [channelId].

@ProviderFor(channelRuns)
final channelRunsProvider = ChannelRunsFamily._();

/// The stored runs of [channelId].

final class ChannelRunsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<RunSummary>>,
          List<RunSummary>,
          Stream<List<RunSummary>>
        >
    with $FutureModifier<List<RunSummary>>, $StreamProvider<List<RunSummary>> {
  /// The stored runs of [channelId].
  ChannelRunsProvider._({
    required ChannelRunsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'channelRunsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$channelRunsHash();

  @override
  String toString() {
    return r'channelRunsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<RunSummary>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<RunSummary>> create(Ref ref) {
    final argument = this.argument as String;
    return channelRuns(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ChannelRunsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$channelRunsHash() => r'8e5a9179db89f17e48eb21ec82b73d49da057e51';

/// The stored runs of [channelId].

final class ChannelRunsFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<RunSummary>>, String> {
  ChannelRunsFamily._()
    : super(
        retry: null,
        name: r'channelRunsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The stored runs of [channelId].

  ChannelRunsProvider call(String channelId) =>
      ChannelRunsProvider._(argument: channelId, from: this);

  @override
  String toString() => r'channelRunsProvider';
}

/// Whether the Done section of [channelId] is collapsed on this device (01 6.1; collapsed by
/// default; docs/DECISIONS.md D-M4-19-4).

@ProviderFor(doneCollapsed)
final doneCollapsedProvider = DoneCollapsedFamily._();

/// Whether the Done section of [channelId] is collapsed on this device (01 6.1; collapsed by
/// default; docs/DECISIONS.md D-M4-19-4).

final class DoneCollapsedProvider
    extends $FunctionalProvider<AsyncValue<bool>, bool, Stream<bool>>
    with $FutureModifier<bool>, $StreamProvider<bool> {
  /// Whether the Done section of [channelId] is collapsed on this device (01 6.1; collapsed by
  /// default; docs/DECISIONS.md D-M4-19-4).
  DoneCollapsedProvider._({
    required DoneCollapsedFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'doneCollapsedProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$doneCollapsedHash();

  @override
  String toString() {
    return r'doneCollapsedProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<bool> create(Ref ref) {
    final argument = this.argument as String;
    return doneCollapsed(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is DoneCollapsedProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$doneCollapsedHash() => r'2b02a3dde4d142ecbb6bdc28a9607f2adeb367d7';

/// Whether the Done section of [channelId] is collapsed on this device (01 6.1; collapsed by
/// default; docs/DECISIONS.md D-M4-19-4).

final class DoneCollapsedFamily extends $Family
    with $FunctionalFamilyOverride<Stream<bool>, String> {
  DoneCollapsedFamily._()
    : super(
        retry: null,
        name: r'doneCollapsedProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// Whether the Done section of [channelId] is collapsed on this device (01 6.1; collapsed by
  /// default; docs/DECISIONS.md D-M4-19-4).

  DoneCollapsedProvider call(String channelId) =>
      DoneCollapsedProvider._(argument: channelId, from: this);

  @override
  String toString() => r'doneCollapsedProvider';
}

/// The new posts of [channelId] the server refused (`createThread` entries of the outbox in
/// `failed`), oldest first (04 13.2 item 5).

@ProviderFor(failedPosts)
final failedPostsProvider = FailedPostsFamily._();

/// The new posts of [channelId] the server refused (`createThread` entries of the outbox in
/// `failed`), oldest first (04 13.2 item 5).

final class FailedPostsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<FailedPost>>,
          List<FailedPost>,
          Stream<List<FailedPost>>
        >
    with $FutureModifier<List<FailedPost>>, $StreamProvider<List<FailedPost>> {
  /// The new posts of [channelId] the server refused (`createThread` entries of the outbox in
  /// `failed`), oldest first (04 13.2 item 5).
  FailedPostsProvider._({
    required FailedPostsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'failedPostsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$failedPostsHash();

  @override
  String toString() {
    return r'failedPostsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<FailedPost>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<FailedPost>> create(Ref ref) {
    final argument = this.argument as String;
    return failedPosts(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is FailedPostsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$failedPostsHash() => r'15d618110ce9cdbb54b15677b41d9bd93f563919';

/// The new posts of [channelId] the server refused (`createThread` entries of the outbox in
/// `failed`), oldest first (04 13.2 item 5).

final class FailedPostsFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<FailedPost>>, String> {
  FailedPostsFamily._()
    : super(
        retry: null,
        name: r'failedPostsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The new posts of [channelId] the server refused (`createThread` entries of the outbox in
  /// `failed`), oldest first (04 13.2 item 5).

  FailedPostsProvider call(String channelId) =>
      FailedPostsProvider._(argument: channelId, from: this);

  @override
  String toString() => r'failedPostsProvider';
}

/// The display names of the stored principals (the operator agent of a row).

@ProviderFor(principalNames)
final principalNamesProvider = PrincipalNamesProvider._();

/// The display names of the stored principals (the operator agent of a row).

final class PrincipalNamesProvider
    extends
        $FunctionalProvider<
          AsyncValue<Map<PrincipalId, String>>,
          Map<PrincipalId, String>,
          Stream<Map<PrincipalId, String>>
        >
    with
        $FutureModifier<Map<PrincipalId, String>>,
        $StreamProvider<Map<PrincipalId, String>> {
  /// The display names of the stored principals (the operator agent of a row).
  PrincipalNamesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'principalNamesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$principalNamesHash();

  @$internal
  @override
  $StreamProviderElement<Map<PrincipalId, String>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<Map<PrincipalId, String>> create(Ref ref) {
    return principalNames(ref);
  }
}

String _$principalNamesHash() => r'35603b8dc17b473c929ddeb9b3904d1cefef4908';

/// The filters of the post list of [channelId]; they last while the app runs.

@ProviderFor(PostListFilterOf)
final postListFilterOfProvider = PostListFilterOfFamily._();

/// The filters of the post list of [channelId]; they last while the app runs.
final class PostListFilterOfProvider
    extends $NotifierProvider<PostListFilterOf, PostListFilter> {
  /// The filters of the post list of [channelId]; they last while the app runs.
  PostListFilterOfProvider._({
    required PostListFilterOfFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'postListFilterOfProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$postListFilterOfHash();

  @override
  String toString() {
    return r'postListFilterOfProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  PostListFilterOf create() => PostListFilterOf();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PostListFilter value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PostListFilter>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is PostListFilterOfProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$postListFilterOfHash() => r'18549c39a0e9ff197927b200ddf8942f8f227bc4';

/// The filters of the post list of [channelId]; they last while the app runs.

final class PostListFilterOfFamily extends $Family
    with
        $ClassFamilyOverride<
          PostListFilterOf,
          PostListFilter,
          PostListFilter,
          PostListFilter,
          String
        > {
  PostListFilterOfFamily._()
    : super(
        retry: null,
        name: r'postListFilterOfProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// The filters of the post list of [channelId]; they last while the app runs.

  PostListFilterOfProvider call(String channelId) =>
      PostListFilterOfProvider._(argument: channelId, from: this);

  @override
  String toString() => r'postListFilterOfProvider';
}

/// The filters of the post list of [channelId]; they last while the app runs.

abstract class _$PostListFilterOf extends $Notifier<PostListFilter> {
  late final _$args = ref.$arg as String;
  String get channelId => _$args;

  PostListFilter build(String channelId);
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<PostListFilter, PostListFilter>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<PostListFilter, PostListFilter>,
              PostListFilter,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, () => build(_$args));
  }
}

/// The post list of [channelId] with [filter] (`buildPostList`); null while the channel is
/// not stored.

@ProviderFor(postList)
final postListProvider = PostListFamily._();

/// The post list of [channelId] with [filter] (`buildPostList`); null while the channel is
/// not stored.

final class PostListProvider
    extends $FunctionalProvider<PostListModel?, PostListModel?, PostListModel?>
    with $Provider<PostListModel?> {
  /// The post list of [channelId] with [filter] (`buildPostList`); null while the channel is
  /// not stored.
  PostListProvider._({
    required PostListFamily super.from,
    required (String, PostListFilter) super.argument,
  }) : super(
         retry: null,
         name: r'postListProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$postListHash();

  @override
  String toString() {
    return r'postListProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  $ProviderElement<PostListModel?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  PostListModel? create(Ref ref) {
    final argument = this.argument as (String, PostListFilter);
    return postList(ref, argument.$1, argument.$2);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PostListModel? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PostListModel?>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is PostListProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$postListHash() => r'610e113eedb440268c0a9d6a637ff6fbc346b63d';

/// The post list of [channelId] with [filter] (`buildPostList`); null while the channel is
/// not stored.

final class PostListFamily extends $Family
    with $FunctionalFamilyOverride<PostListModel?, (String, PostListFilter)> {
  PostListFamily._()
    : super(
        retry: null,
        name: r'postListProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The post list of [channelId] with [filter] (`buildPostList`); null while the channel is
  /// not stored.

  PostListProvider call(String channelId, PostListFilter filter) =>
      PostListProvider._(argument: (channelId, filter), from: this);

  @override
  String toString() => r'postListProvider';
}

/// The scroll offset of the post list of [channelId] where the person left it; it lasts while
/// the app runs, so opening a post (another route, a new column) keeps the place.

@ProviderFor(PostListOffset)
final postListOffsetProvider = PostListOffsetFamily._();

/// The scroll offset of the post list of [channelId] where the person left it; it lasts while
/// the app runs, so opening a post (another route, a new column) keeps the place.
final class PostListOffsetProvider
    extends $NotifierProvider<PostListOffset, double> {
  /// The scroll offset of the post list of [channelId] where the person left it; it lasts while
  /// the app runs, so opening a post (another route, a new column) keeps the place.
  PostListOffsetProvider._({
    required PostListOffsetFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'postListOffsetProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$postListOffsetHash();

  @override
  String toString() {
    return r'postListOffsetProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  PostListOffset create() => PostListOffset();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(double value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<double>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is PostListOffsetProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$postListOffsetHash() => r'51de882ffee678df7e41d1c1238e1c114312e214';

/// The scroll offset of the post list of [channelId] where the person left it; it lasts while
/// the app runs, so opening a post (another route, a new column) keeps the place.

final class PostListOffsetFamily extends $Family
    with $ClassFamilyOverride<PostListOffset, double, double, double, String> {
  PostListOffsetFamily._()
    : super(
        retry: null,
        name: r'postListOffsetProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// The scroll offset of the post list of [channelId] where the person left it; it lasts while
  /// the app runs, so opening a post (another route, a new column) keeps the place.

  PostListOffsetProvider call(String channelId) =>
      PostListOffsetProvider._(argument: channelId, from: this);

  @override
  String toString() => r'postListOffsetProvider';
}

/// The scroll offset of the post list of [channelId] where the person left it; it lasts while
/// the app runs, so opening a post (another route, a new column) keeps the place.

abstract class _$PostListOffset extends $Notifier<double> {
  late final _$args = ref.$arg as String;
  String get channelId => _$args;

  double build(String channelId);
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<double, double>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<double, double>,
              double,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, () => build(_$args));
  }
}

/// The paging of the post list of [channelId] in the order [sort] (a `ForumSort` code):
/// pages of `listChannelThreads` go into `threads` (brief M4-19 Design note 3;
/// docs/DECISIONS.md D-M4-19-5). A new engine (a logout, a new login) starts the paging again,
/// because the new database holds none of the loaded pages (D-M4-19-16).

@ProviderFor(ForumPager)
final forumPagerProvider = ForumPagerFamily._();

/// The paging of the post list of [channelId] in the order [sort] (a `ForumSort` code):
/// pages of `listChannelThreads` go into `threads` (brief M4-19 Design note 3;
/// docs/DECISIONS.md D-M4-19-5). A new engine (a logout, a new login) starts the paging again,
/// because the new database holds none of the loaded pages (D-M4-19-16).
final class ForumPagerProvider
    extends $NotifierProvider<ForumPager, PagerState> {
  /// The paging of the post list of [channelId] in the order [sort] (a `ForumSort` code):
  /// pages of `listChannelThreads` go into `threads` (brief M4-19 Design note 3;
  /// docs/DECISIONS.md D-M4-19-5). A new engine (a logout, a new login) starts the paging again,
  /// because the new database holds none of the loaded pages (D-M4-19-16).
  ForumPagerProvider._({
    required ForumPagerFamily super.from,
    required (String, String) super.argument,
  }) : super(
         retry: null,
         name: r'forumPagerProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$forumPagerHash();

  @override
  String toString() {
    return r'forumPagerProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  ForumPager create() => ForumPager();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PagerState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PagerState>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ForumPagerProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$forumPagerHash() => r'1a461fead8f43303d4dc12fbfe1a80362d3a0ce2';

/// The paging of the post list of [channelId] in the order [sort] (a `ForumSort` code):
/// pages of `listChannelThreads` go into `threads` (brief M4-19 Design note 3;
/// docs/DECISIONS.md D-M4-19-5). A new engine (a logout, a new login) starts the paging again,
/// because the new database holds none of the loaded pages (D-M4-19-16).

final class ForumPagerFamily extends $Family
    with
        $ClassFamilyOverride<
          ForumPager,
          PagerState,
          PagerState,
          PagerState,
          (String, String)
        > {
  ForumPagerFamily._()
    : super(
        retry: null,
        name: r'forumPagerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// The paging of the post list of [channelId] in the order [sort] (a `ForumSort` code):
  /// pages of `listChannelThreads` go into `threads` (brief M4-19 Design note 3;
  /// docs/DECISIONS.md D-M4-19-5). A new engine (a logout, a new login) starts the paging again,
  /// because the new database holds none of the loaded pages (D-M4-19-16).

  ForumPagerProvider call(String channelId, String sort) =>
      ForumPagerProvider._(argument: (channelId, sort), from: this);

  @override
  String toString() => r'forumPagerProvider';
}

/// The paging of the post list of [channelId] in the order [sort] (a `ForumSort` code):
/// pages of `listChannelThreads` go into `threads` (brief M4-19 Design note 3;
/// docs/DECISIONS.md D-M4-19-5). A new engine (a logout, a new login) starts the paging again,
/// because the new database holds none of the loaded pages (D-M4-19-16).

abstract class _$ForumPager extends $Notifier<PagerState> {
  late final _$args = ref.$arg as (String, String);
  String get channelId => _$args.$1;
  String get sort => _$args.$2;

  PagerState build(String channelId, String sort);
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<PagerState, PagerState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<PagerState, PagerState>,
              PagerState,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, () => build(_$args.$1, _$args.$2));
  }
}

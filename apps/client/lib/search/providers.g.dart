// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The server search of the signed-in instance (`search`), or null while signed out.

@ProviderFor(searchPort)
final searchPortProvider = SearchPortProvider._();

/// The server search of the signed-in instance (`search`), or null while signed out.

final class SearchPortProvider
    extends $FunctionalProvider<SearchPort?, SearchPort?, SearchPort?>
    with $Provider<SearchPort?> {
  /// The server search of the signed-in instance (`search`), or null while signed out.
  SearchPortProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'searchPortProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$searchPortHash();

  @$internal
  @override
  $ProviderElement<SearchPort?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  SearchPort? create(Ref ref) {
    return searchPort(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SearchPort? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SearchPort?>(value),
    );
  }
}

String _$searchPortHash() => r'4d2d01e18380c4335b553402fad3e3da0188b5ac';

/// The offline search over the local index (14 6), or null while no database is open.

@ProviderFor(localSearchPort)
final localSearchPortProvider = LocalSearchPortProvider._();

/// The offline search over the local index (14 6), or null while no database is open.

final class LocalSearchPortProvider
    extends
        $FunctionalProvider<
          LocalSearchPort?,
          LocalSearchPort?,
          LocalSearchPort?
        >
    with $Provider<LocalSearchPort?> {
  /// The offline search over the local index (14 6), or null while no database is open.
  LocalSearchPortProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'localSearchPortProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$localSearchPortHash();

  @$internal
  @override
  $ProviderElement<LocalSearchPort?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  LocalSearchPort? create(Ref ref) {
    return localSearchPort(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(LocalSearchPort? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<LocalSearchPort?>(value),
    );
  }
}

String _$localSearchPortHash() => r'bfea5fbfb4831acd6a7713ac64b85b2062aac924';

/// The stored people and agents by display name: `from:` and the "From" menu.

@ProviderFor(searchPrincipals)
final searchPrincipalsProvider = SearchPrincipalsProvider._();

/// The stored people and agents by display name: `from:` and the "From" menu.

final class SearchPrincipalsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Principal>>,
          List<Principal>,
          Stream<List<Principal>>
        >
    with $FutureModifier<List<Principal>>, $StreamProvider<List<Principal>> {
  /// The stored people and agents by display name: `from:` and the "From" menu.
  SearchPrincipalsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'searchPrincipalsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$searchPrincipalsHash();

  @$internal
  @override
  $StreamProviderElement<List<Principal>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<Principal>> create(Ref ref) {
    return searchPrincipals(ref);
  }
}

String _$searchPrincipalsHash() => r'49016a8776e053d5665782e57670eb61bd3c8c3e';

/// The stored spaces in their order: the "Space" menu.

@ProviderFor(searchSpaces)
final searchSpacesProvider = SearchSpacesProvider._();

/// The stored spaces in their order: the "Space" menu.

final class SearchSpacesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Space>>,
          List<Space>,
          Stream<List<Space>>
        >
    with $FutureModifier<List<Space>>, $StreamProvider<List<Space>> {
  /// The stored spaces in their order: the "Space" menu.
  SearchSpacesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'searchSpacesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$searchSpacesHash();

  @$internal
  @override
  $StreamProviderElement<List<Space>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<Space>> create(Ref ref) {
    return searchSpaces(ref);
  }
}

String _$searchSpacesHash() => r'd922189fa29c96a5bc89701e90f1664e71b4c1f9';

/// The stored channels of [spaceId] in their order.

@ProviderFor(searchSpaceChannels)
final searchSpaceChannelsProvider = SearchSpaceChannelsFamily._();

/// The stored channels of [spaceId] in their order.

final class SearchSpaceChannelsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Channel>>,
          List<Channel>,
          Stream<List<Channel>>
        >
    with $FutureModifier<List<Channel>>, $StreamProvider<List<Channel>> {
  /// The stored channels of [spaceId] in their order.
  SearchSpaceChannelsProvider._({
    required SearchSpaceChannelsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'searchSpaceChannelsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$searchSpaceChannelsHash();

  @override
  String toString() {
    return r'searchSpaceChannelsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<Channel>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<Channel>> create(Ref ref) {
    final argument = this.argument as String;
    return searchSpaceChannels(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is SearchSpaceChannelsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$searchSpaceChannelsHash() =>
    r'8228a0c7de48ca64c17d842d7c0e76d9af359a6b';

/// The stored channels of [spaceId] in their order.

final class SearchSpaceChannelsFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<Channel>>, String> {
  SearchSpaceChannelsFamily._()
    : super(
        retry: null,
        name: r'searchSpaceChannelsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The stored channels of [spaceId] in their order.

  SearchSpaceChannelsProvider call(String spaceId) =>
      SearchSpaceChannelsProvider._(argument: spaceId, from: this);

  @override
  String toString() => r'searchSpaceChannelsProvider';
}

/// The stored channels of every space, space by space: `in:` and the "Channel" menu. The
/// database holds only what the person may see (15 1 principle 1).

@ProviderFor(searchChannels)
final searchChannelsProvider = SearchChannelsProvider._();

/// The stored channels of every space, space by space: `in:` and the "Channel" menu. The
/// database holds only what the person may see (15 1 principle 1).

final class SearchChannelsProvider
    extends $FunctionalProvider<List<Channel>, List<Channel>, List<Channel>>
    with $Provider<List<Channel>> {
  /// The stored channels of every space, space by space: `in:` and the "Channel" menu. The
  /// database holds only what the person may see (15 1 principle 1).
  SearchChannelsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'searchChannelsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$searchChannelsHash();

  @$internal
  @override
  $ProviderElement<List<Channel>> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  List<Channel> create(Ref ref) {
    return searchChannels(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(List<Channel> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<List<Channel>>(value),
    );
  }
}

String _$searchChannelsHash() => r'1db6ca1d51bddfaa570f5c9b8502d7a7e5ed52b8';

/// Whether the engine's connection is live (15 7.2); any other phase counts as offline for the
/// search (brief M4-23 Design note 1).

@ProviderFor(searchOnline)
final searchOnlineProvider = SearchOnlineProvider._();

/// Whether the engine's connection is live (15 7.2); any other phase counts as offline for the
/// search (brief M4-23 Design note 1).

final class SearchOnlineProvider extends $FunctionalProvider<bool, bool, bool>
    with $Provider<bool> {
  /// Whether the engine's connection is live (15 7.2); any other phase counts as offline for the
  /// search (brief M4-23 Design note 1).
  SearchOnlineProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'searchOnlineProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$searchOnlineHash();

  @$internal
  @override
  $ProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  bool create(Ref ref) {
    return searchOnline(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool>(value),
    );
  }
}

String _$searchOnlineHash() => r'148725333ba11f689c3af683746dcce890b24196';

/// The search of D7, or with [channelId] and [threadId] the in-thread search of M4-24, which
/// sends both with every request.
///
/// Example:
/// ```dart
/// final search = ref.read(searchQueryControllerProvider().notifier);
/// search.setInput('from:sam deploy');
/// await search.submit();
/// ```

@ProviderFor(SearchQueryController)
final searchQueryControllerProvider = SearchQueryControllerFamily._();

/// The search of D7, or with [channelId] and [threadId] the in-thread search of M4-24, which
/// sends both with every request.
///
/// Example:
/// ```dart
/// final search = ref.read(searchQueryControllerProvider().notifier);
/// search.setInput('from:sam deploy');
/// await search.submit();
/// ```
final class SearchQueryControllerProvider
    extends $NotifierProvider<SearchQueryController, SearchState> {
  /// The search of D7, or with [channelId] and [threadId] the in-thread search of M4-24, which
  /// sends both with every request.
  ///
  /// Example:
  /// ```dart
  /// final search = ref.read(searchQueryControllerProvider().notifier);
  /// search.setInput('from:sam deploy');
  /// await search.submit();
  /// ```
  SearchQueryControllerProvider._({
    required SearchQueryControllerFamily super.from,
    required ({String? channelId, String? threadId}) super.argument,
  }) : super(
         retry: null,
         name: r'searchQueryControllerProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$searchQueryControllerHash();

  @override
  String toString() {
    return r'searchQueryControllerProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  SearchQueryController create() => SearchQueryController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SearchState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SearchState>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is SearchQueryControllerProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$searchQueryControllerHash() =>
    r'262130500962d0fc1d4b8fa6d60ea2f1cffdf5d1';

/// The search of D7, or with [channelId] and [threadId] the in-thread search of M4-24, which
/// sends both with every request.
///
/// Example:
/// ```dart
/// final search = ref.read(searchQueryControllerProvider().notifier);
/// search.setInput('from:sam deploy');
/// await search.submit();
/// ```

final class SearchQueryControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          SearchQueryController,
          SearchState,
          SearchState,
          SearchState,
          ({String? channelId, String? threadId})
        > {
  SearchQueryControllerFamily._()
    : super(
        retry: null,
        name: r'searchQueryControllerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The search of D7, or with [channelId] and [threadId] the in-thread search of M4-24, which
  /// sends both with every request.
  ///
  /// Example:
  /// ```dart
  /// final search = ref.read(searchQueryControllerProvider().notifier);
  /// search.setInput('from:sam deploy');
  /// await search.submit();
  /// ```

  SearchQueryControllerProvider call({String? channelId, String? threadId}) =>
      SearchQueryControllerProvider._(
        argument: (channelId: channelId, threadId: threadId),
        from: this,
      );

  @override
  String toString() => r'searchQueryControllerProvider';
}

/// The search of D7, or with [channelId] and [threadId] the in-thread search of M4-24, which
/// sends both with every request.
///
/// Example:
/// ```dart
/// final search = ref.read(searchQueryControllerProvider().notifier);
/// search.setInput('from:sam deploy');
/// await search.submit();
/// ```

abstract class _$SearchQueryController extends $Notifier<SearchState> {
  late final _$args = ref.$arg as ({String? channelId, String? threadId});
  String? get channelId => _$args.channelId;
  String? get threadId => _$args.threadId;

  SearchState build({String? channelId, String? threadId});
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<SearchState, SearchState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<SearchState, SearchState>,
              SearchState,
              Object?,
              Object?
            >;
    return element.handleCreate(
      ref,
      () => build(channelId: _$args.channelId, threadId: _$args.threadId),
    );
  }
}

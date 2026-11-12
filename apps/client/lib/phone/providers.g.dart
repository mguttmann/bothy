// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The number of outbox entries that are not final, for the offline banner (01 5.3 M7:
/// "Offline. 1 message waiting"); 0 while signed out.

@ProviderFor(outboxWaiting)
final outboxWaitingProvider = OutboxWaitingProvider._();

/// The number of outbox entries that are not final, for the offline banner (01 5.3 M7:
/// "Offline. 1 message waiting"); 0 while signed out.

final class OutboxWaitingProvider
    extends $FunctionalProvider<AsyncValue<int>, int, Stream<int>>
    with $FutureModifier<int>, $StreamProvider<int> {
  /// The number of outbox entries that are not final, for the offline banner (01 5.3 M7:
  /// "Offline. 1 message waiting"); 0 while signed out.
  OutboxWaitingProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'outboxWaitingProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$outboxWaitingHash();

  @$internal
  @override
  $StreamProviderElement<int> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<int> create(Ref ref) {
    return outboxWaiting(ref);
  }
}

String _$outboxWaitingHash() => r'b677c1a1320f797843645d514cbfeb090baac8fd';

/// Deletes the photos of a pick during which Android ended the app (brief M5-07 requirement
/// 16; `MediaPicker.discardLost`), once per start of an Android build: the phone shell reads it
/// when it first shows. Every other build completes without asking.

@ProviderFor(lostPhotosDiscarded)
final lostPhotosDiscardedProvider = LostPhotosDiscardedProvider._();

/// Deletes the photos of a pick during which Android ended the app (brief M5-07 requirement
/// 16; `MediaPicker.discardLost`), once per start of an Android build: the phone shell reads it
/// when it first shows. Every other build completes without asking.

final class LostPhotosDiscardedProvider
    extends $FunctionalProvider<AsyncValue<void>, void, FutureOr<void>>
    with $FutureModifier<void>, $FutureProvider<void> {
  /// Deletes the photos of a pick during which Android ended the app (brief M5-07 requirement
  /// 16; `MediaPicker.discardLost`), once per start of an Android build: the phone shell reads it
  /// when it first shows. Every other build completes without asking.
  LostPhotosDiscardedProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'lostPhotosDiscardedProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$lostPhotosDiscardedHash();

  @$internal
  @override
  $FutureProviderElement<void> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<void> create(Ref ref) {
    return lostPhotosDiscarded(ref);
  }
}

String _$lostPhotosDiscardedHash() =>
    r'3c2f98b8bde5e134fb995f6d6f49ed9f221b2070';

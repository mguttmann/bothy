// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// [AppleShare] of the build kept in step with the signed-in account, or null where the build
/// has none (Windows, Android, tests).
///
/// The account is handed over once per signed-in services: after a login (the session's
/// `SignedIn` event) the child session is always created anew, at a start with a stored
/// session only when the shared keychain has none. The recent targets are written again when
/// the connection becomes live (the person is known from the snapshot), after every change of
/// the outbox (a confirmed send) and of the stored channels (`access.changed`); the
/// `share_inbox` rows of confirmed or discarded entries go on every change of the outbox. A
/// return to the foreground creates a missing child session and takes the inbox over, and the
/// instance limits follow `getInstance`.

@ProviderFor(appleShare)
final appleShareProvider = AppleShareProvider._();

/// [AppleShare] of the build kept in step with the signed-in account, or null where the build
/// has none (Windows, Android, tests).
///
/// The account is handed over once per signed-in services: after a login (the session's
/// `SignedIn` event) the child session is always created anew, at a start with a stored
/// session only when the shared keychain has none. The recent targets are written again when
/// the connection becomes live (the person is known from the snapshot), after every change of
/// the outbox (a confirmed send) and of the stored channels (`access.changed`); the
/// `share_inbox` rows of confirmed or discarded entries go on every change of the outbox. A
/// return to the foreground creates a missing child session and takes the inbox over, and the
/// instance limits follow `getInstance`.

final class AppleShareProvider
    extends $FunctionalProvider<AppleShare?, AppleShare?, AppleShare?>
    with $Provider<AppleShare?> {
  /// [AppleShare] of the build kept in step with the signed-in account, or null where the build
  /// has none (Windows, Android, tests).
  ///
  /// The account is handed over once per signed-in services: after a login (the session's
  /// `SignedIn` event) the child session is always created anew, at a start with a stored
  /// session only when the shared keychain has none. The recent targets are written again when
  /// the connection becomes live (the person is known from the snapshot), after every change of
  /// the outbox (a confirmed send) and of the stored channels (`access.changed`); the
  /// `share_inbox` rows of confirmed or discarded entries go on every change of the outbox. A
  /// return to the foreground creates a missing child session and takes the inbox over, and the
  /// instance limits follow `getInstance`.
  AppleShareProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'appleShareProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$appleShareHash();

  @$internal
  @override
  $ProviderElement<AppleShare?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AppleShare? create(Ref ref) {
    return appleShare(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AppleShare? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AppleShare?>(value),
    );
  }
}

String _$appleShareHash() => r'599d4d08e842c035d9dc0ddb1f48465f83dc28d8';

/// The share the Android runner received last, until the route `/share` sent or cancelled it.

@ProviderFor(PendingShare)
final pendingShareProvider = PendingShareProvider._();

/// The share the Android runner received last, until the route `/share` sent or cancelled it.
final class PendingShareProvider
    extends $NotifierProvider<PendingShare, SharedContent?> {
  /// The share the Android runner received last, until the route `/share` sent or cancelled it.
  PendingShareProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'pendingShareProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$pendingShareHash();

  @$internal
  @override
  PendingShare create() => PendingShare();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SharedContent? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SharedContent?>(value),
    );
  }
}

String _$pendingShareHash() => r'075bec5969d37de34524259613993f7d38b0c1e9';

/// The share the Android runner received last, until the route `/share` sent or cancelled it.

abstract class _$PendingShare extends $Notifier<SharedContent?> {
  SharedContent? build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<SharedContent?, SharedContent?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<SharedContent?, SharedContent?>,
              SharedContent?,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

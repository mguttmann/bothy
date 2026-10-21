// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Shows `notification.created` of the signed-in engine as local notifications, on Windows and
/// on macOS while the app is connected (13 5.3, D-M4-30-5); the app widget listens to its
/// [DesktopNotifications.targets].

@ProviderFor(desktopNotifications)
final desktopNotificationsProvider = DesktopNotificationsProvider._();

/// Shows `notification.created` of the signed-in engine as local notifications, on Windows and
/// on macOS while the app is connected (13 5.3, D-M4-30-5); the app widget listens to its
/// [DesktopNotifications.targets].

final class DesktopNotificationsProvider
    extends
        $FunctionalProvider<
          DesktopNotifications,
          DesktopNotifications,
          DesktopNotifications
        >
    with $Provider<DesktopNotifications> {
  /// Shows `notification.created` of the signed-in engine as local notifications, on Windows and
  /// on macOS while the app is connected (13 5.3, D-M4-30-5); the app widget listens to its
  /// [DesktopNotifications.targets].
  DesktopNotificationsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'desktopNotificationsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$desktopNotificationsHash();

  @$internal
  @override
  $ProviderElement<DesktopNotifications> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  DesktopNotifications create(Ref ref) {
    return desktopNotifications(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DesktopNotifications value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DesktopNotifications>(value),
    );
  }
}

String _$desktopNotificationsHash() =>
    r'8aeb6cff0dd63fff72b8dfd4af5275076df1dde4';

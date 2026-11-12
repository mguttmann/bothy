// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The banners of the phone app, or null on the desktop: a phone is connected only while it is
/// in front, so it shows the hints of the gateway inside the app instead of as notifications
/// of the system (13 4.6 item 1; docs/decisions.d/AUD-client-app-01.md). The app widget lays
/// them over its content with an `InAppBannerHost`.

@ProviderFor(inAppBanners)
final inAppBannersProvider = InAppBannersProvider._();

/// The banners of the phone app, or null on the desktop: a phone is connected only while it is
/// in front, so it shows the hints of the gateway inside the app instead of as notifications
/// of the system (13 4.6 item 1; docs/decisions.d/AUD-client-app-01.md). The app widget lays
/// them over its content with an `InAppBannerHost`.

final class InAppBannersProvider
    extends $FunctionalProvider<InAppBanners?, InAppBanners?, InAppBanners?>
    with $Provider<InAppBanners?> {
  /// The banners of the phone app, or null on the desktop: a phone is connected only while it is
  /// in front, so it shows the hints of the gateway inside the app instead of as notifications
  /// of the system (13 4.6 item 1; docs/decisions.d/AUD-client-app-01.md). The app widget lays
  /// them over its content with an `InAppBannerHost`.
  InAppBannersProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'inAppBannersProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$inAppBannersHash();

  @$internal
  @override
  $ProviderElement<InAppBanners?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  InAppBanners? create(Ref ref) {
    return inAppBanners(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(InAppBanners? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<InAppBanners?>(value),
    );
  }
}

String _$inAppBannersHash() => r'cf6fed55545da86e5216bd7ca697ba12245a1094';

/// Shows `notification.created` of the signed-in engine while the app is connected: as local
/// notifications on Windows and macOS (13 5.3, D-M4-30-5), as the in-app banner on iOS and
/// Android ([inAppBanners], with the same rules of the [NotificationRouter]), and removes the
/// notification or the banner of a thread once the person has read it (01 10.3,
/// [NotificationRouter.onCountChange]); the app widget listens to its
/// [DesktopNotifications.targets].

@ProviderFor(desktopNotifications)
final desktopNotificationsProvider = DesktopNotificationsProvider._();

/// Shows `notification.created` of the signed-in engine while the app is connected: as local
/// notifications on Windows and macOS (13 5.3, D-M4-30-5), as the in-app banner on iOS and
/// Android ([inAppBanners], with the same rules of the [NotificationRouter]), and removes the
/// notification or the banner of a thread once the person has read it (01 10.3,
/// [NotificationRouter.onCountChange]); the app widget listens to its
/// [DesktopNotifications.targets].

final class DesktopNotificationsProvider
    extends
        $FunctionalProvider<
          DesktopNotifications,
          DesktopNotifications,
          DesktopNotifications
        >
    with $Provider<DesktopNotifications> {
  /// Shows `notification.created` of the signed-in engine while the app is connected: as local
  /// notifications on Windows and macOS (13 5.3, D-M4-30-5), as the in-app banner on iOS and
  /// Android ([inAppBanners], with the same rules of the [NotificationRouter]), and removes the
  /// notification or the banner of a thread once the person has read it (01 10.3,
  /// [NotificationRouter.onCountChange]); the app widget listens to its
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
    r'29642e5c2d96a7915f6ad2ab299b3f6bc7772723';

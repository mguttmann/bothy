// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The behavior of the app window: on Windows a close hides it to the notification area
/// (15 10.3), and Quit closes the services of the instance (engine and database) before the
/// window ends, so the device has no gateway connection any more.

@ProviderFor(windowBehavior)
final windowBehaviorProvider = WindowBehaviorProvider._();

/// The behavior of the app window: on Windows a close hides it to the notification area
/// (15 10.3), and Quit closes the services of the instance (engine and database) before the
/// window ends, so the device has no gateway connection any more.

final class WindowBehaviorProvider
    extends $FunctionalProvider<WindowBehavior, WindowBehavior, WindowBehavior>
    with $Provider<WindowBehavior> {
  /// The behavior of the app window: on Windows a close hides it to the notification area
  /// (15 10.3), and Quit closes the services of the instance (engine and database) before the
  /// window ends, so the device has no gateway connection any more.
  WindowBehaviorProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'windowBehaviorProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$windowBehaviorHash();

  @$internal
  @override
  $ProviderElement<WindowBehavior> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  WindowBehavior create(Ref ref) {
    return windowBehavior(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(WindowBehavior value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<WindowBehavior>(value),
    );
  }
}

String _$windowBehaviorHash() => r'433d3bdc5bfa7a7b25e2a6d9adbc30bf98353452';

/// The tray of the desktop app, or null where the app has none: on Windows the icon with Open
/// and Quit stays (15 10.3); on macOS the icon shows only while a computer session of the owner
/// is live (D-CU-07-4, which narrows D-M4-30-5).

@ProviderFor(desktopTray)
final desktopTrayProvider = DesktopTrayProvider._();

/// The tray of the desktop app, or null where the app has none: on Windows the icon with Open
/// and Quit stays (15 10.3); on macOS the icon shows only while a computer session of the owner
/// is live (D-CU-07-4, which narrows D-M4-30-5).

final class DesktopTrayProvider
    extends $FunctionalProvider<DesktopTray?, DesktopTray?, DesktopTray?>
    with $Provider<DesktopTray?> {
  /// The tray of the desktop app, or null where the app has none: on Windows the icon with Open
  /// and Quit stays (15 10.3); on macOS the icon shows only while a computer session of the owner
  /// is live (D-CU-07-4, which narrows D-M4-30-5).
  DesktopTrayProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'desktopTrayProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$desktopTrayHash();

  @$internal
  @override
  $ProviderElement<DesktopTray?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  DesktopTray? create(Ref ref) {
    return desktopTray(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DesktopTray? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DesktopTray?>(value),
    );
  }
}

String _$desktopTrayHash() => r'f829886b1424e63085a16225a04557e60e578326';

/// Starts the window behavior and the tray: on Windows the tray icon with Open and Quit, on
/// macOS the texts of the icon that shows during computer use; the app widget watches it once.

@ProviderFor(desktopShell)
final desktopShellProvider = DesktopShellProvider._();

/// Starts the window behavior and the tray: on Windows the tray icon with Open and Quit, on
/// macOS the texts of the icon that shows during computer use; the app widget watches it once.

final class DesktopShellProvider
    extends $FunctionalProvider<AsyncValue<void>, void, FutureOr<void>>
    with $FutureModifier<void>, $FutureProvider<void> {
  /// Starts the window behavior and the tray: on Windows the tray icon with Open and Quit, on
  /// macOS the texts of the icon that shows during computer use; the app widget watches it once.
  DesktopShellProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'desktopShellProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$desktopShellHash();

  @$internal
  @override
  $FutureProviderElement<void> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<void> create(Ref ref) {
    return desktopShell(ref);
  }
}

String _$desktopShellHash() => r'dfc3a95a05d3023c72a177f3621d40e1e23f27ae';

/// The computer entry of the tray (27 11 item 4; brief CU-07): follows the owner's live
/// computer sessions; null where the app has no tray.

@ProviderFor(computerTray)
final computerTrayProvider = ComputerTrayProvider._();

/// The computer entry of the tray (27 11 item 4; brief CU-07): follows the owner's live
/// computer sessions; null where the app has no tray.

final class ComputerTrayProvider
    extends $FunctionalProvider<ComputerTray?, ComputerTray?, ComputerTray?>
    with $Provider<ComputerTray?> {
  /// The computer entry of the tray (27 11 item 4; brief CU-07): follows the owner's live
  /// computer sessions; null where the app has no tray.
  ComputerTrayProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'computerTrayProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$computerTrayHash();

  @$internal
  @override
  $ProviderElement<ComputerTray?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ComputerTray? create(Ref ref) {
    return computerTray(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ComputerTray? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ComputerTray?>(value),
    );
  }
}

String _$computerTrayHash() => r'5ba86fa2db4d0630c8213db60db222ef9ab4f6aa';

/// "Start at login", read from and written to the system (D-M4-30-4); off until the person
/// turns it on.

@ProviderFor(StartAtLoginSetting)
final startAtLoginSettingProvider = StartAtLoginSettingProvider._();

/// "Start at login", read from and written to the system (D-M4-30-4); off until the person
/// turns it on.
final class StartAtLoginSettingProvider
    extends $AsyncNotifierProvider<StartAtLoginSetting, StartAtLoginState> {
  /// "Start at login", read from and written to the system (D-M4-30-4); off until the person
  /// turns it on.
  StartAtLoginSettingProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'startAtLoginSettingProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$startAtLoginSettingHash();

  @$internal
  @override
  StartAtLoginSetting create() => StartAtLoginSetting();
}

String _$startAtLoginSettingHash() =>
    r'f6b50a4694b5be8f3e34cd14cf92d73b7b8598f4';

/// "Start at login", read from and written to the system (D-M4-30-4); off until the person
/// turns it on.

abstract class _$StartAtLoginSetting extends $AsyncNotifier<StartAtLoginState> {
  FutureOr<StartAtLoginState> build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref =
        this.ref as $Ref<AsyncValue<StartAtLoginState>, StartAtLoginState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<StartAtLoginState>, StartAtLoginState>,
              AsyncValue<StartAtLoginState>,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

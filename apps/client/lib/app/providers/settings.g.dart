// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'settings.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Every stored personal setting by key; empty before the login.

@ProviderFor(userSettings)
final userSettingsProvider = UserSettingsProvider._();

/// Every stored personal setting by key; empty before the login.

final class UserSettingsProvider
    extends
        $FunctionalProvider<
          AsyncValue<Map<String, Object?>>,
          Map<String, Object?>,
          Stream<Map<String, Object?>>
        >
    with
        $FutureModifier<Map<String, Object?>>,
        $StreamProvider<Map<String, Object?>> {
  /// Every stored personal setting by key; empty before the login.
  UserSettingsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'userSettingsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$userSettingsHash();

  @$internal
  @override
  $StreamProviderElement<Map<String, Object?>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<Map<String, Object?>> create(Ref ref) {
    return userSettings(ref);
  }
}

String _$userSettingsHash() => r'bfc3f10d6cbab632710aea9113aaa79f74e61b40';

/// The column widths of `layout.column_widths`, or the defaults.

@ProviderFor(columnWidths)
final columnWidthsProvider = ColumnWidthsProvider._();

/// The column widths of `layout.column_widths`, or the defaults.

final class ColumnWidthsProvider
    extends $FunctionalProvider<ColumnWidths, ColumnWidths, ColumnWidths>
    with $Provider<ColumnWidths> {
  /// The column widths of `layout.column_widths`, or the defaults.
  ColumnWidthsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'columnWidthsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$columnWidthsHash();

  @$internal
  @override
  $ProviderElement<ColumnWidths> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ColumnWidths create(Ref ref) {
    return columnWidths(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ColumnWidths value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ColumnWidths>(value),
    );
  }
}

String _$columnWidthsHash() => r'e86c91fb6b7080c9f5ffe17bce4c81c48e34e2e8';

/// The design context from `appearance.*`, the system brightness and the accent of the
/// instance (01 11, 01 12).

@ProviderFor(appearance)
final appearanceProvider = AppearanceFamily._();

/// The design context from `appearance.*`, the system brightness and the accent of the
/// instance (01 11, 01 12).

final class AppearanceProvider
    extends $FunctionalProvider<DsContext, DsContext, DsContext>
    with $Provider<DsContext> {
  /// The design context from `appearance.*`, the system brightness and the accent of the
  /// instance (01 11, 01 12).
  AppearanceProvider._({
    required AppearanceFamily super.from,
    required (Brightness, bool) super.argument,
  }) : super(
         retry: null,
         name: r'appearanceProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$appearanceHash();

  @override
  String toString() {
    return r'appearanceProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  $ProviderElement<DsContext> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  DsContext create(Ref ref) {
    final argument = this.argument as (Brightness, bool);
    return appearance(ref, argument.$1, argument.$2);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DsContext value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DsContext>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is AppearanceProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$appearanceHash() => r'62317c87f25c16639c0598961a1ff3513edccba5';

/// The design context from `appearance.*`, the system brightness and the accent of the
/// instance (01 11, 01 12).

final class AppearanceFamily extends $Family
    with $FunctionalFamilyOverride<DsContext, (Brightness, bool)> {
  AppearanceFamily._()
    : super(
        retry: null,
        name: r'appearanceProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The design context from `appearance.*`, the system brightness and the accent of the
  /// instance (01 11, 01 12).

  AppearanceProvider call(
    Brightness platformBrightness,
    bool platformReduceMotion,
  ) => AppearanceProvider._(
    argument: (platformBrightness, platformReduceMotion),
    from: this,
  );

  @override
  String toString() => r'appearanceProvider';
}

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'data_export.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The export of the own data of the signed-in instance, or null before the login.

@ProviderFor(dataExportPort)
final dataExportPortProvider = DataExportPortProvider._();

/// The export of the own data of the signed-in instance, or null before the login.

final class DataExportPortProvider
    extends
        $FunctionalProvider<DataExportPort?, DataExportPort?, DataExportPort?>
    with $Provider<DataExportPort?> {
  /// The export of the own data of the signed-in instance, or null before the login.
  DataExportPortProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'dataExportPortProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$dataExportPortHash();

  @$internal
  @override
  $ProviderElement<DataExportPort?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  DataExportPort? create(Ref ref) {
    return dataExportPort(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DataExportPort? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DataExportPort?>(value),
    );
  }
}

String _$dataExportPortHash() => r'da2e55bb43f98df4171d474a6d0a2b449cd5a7c0';

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'target.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Whether the target of [path] (path and query) is visible; unknown while signed out.

@ProviderFor(targetStatus)
final targetStatusProvider = TargetStatusFamily._();

/// Whether the target of [path] (path and query) is visible; unknown while signed out.

final class TargetStatusProvider
    extends
        $FunctionalProvider<
          AsyncValue<TargetStatus>,
          TargetStatus,
          FutureOr<TargetStatus>
        >
    with $FutureModifier<TargetStatus>, $FutureProvider<TargetStatus> {
  /// Whether the target of [path] (path and query) is visible; unknown while signed out.
  TargetStatusProvider._({
    required TargetStatusFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'targetStatusProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$targetStatusHash();

  @override
  String toString() {
    return r'targetStatusProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<TargetStatus> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<TargetStatus> create(Ref ref) {
    final argument = this.argument as String;
    return targetStatus(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is TargetStatusProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$targetStatusHash() => r'f96174bc9db39afc688c34ae80d0fe7efe846d53';

/// Whether the target of [path] (path and query) is visible; unknown while signed out.

final class TargetStatusFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<TargetStatus>, String> {
  TargetStatusFamily._()
    : super(
        retry: null,
        name: r'targetStatusProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// Whether the target of [path] (path and query) is visible; unknown while signed out.

  TargetStatusProvider call(String path) =>
      TargetStatusProvider._(argument: path, from: this);

  @override
  String toString() => r'targetStatusProvider';
}

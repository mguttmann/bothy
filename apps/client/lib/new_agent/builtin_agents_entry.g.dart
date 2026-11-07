// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'builtin_agents_entry.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Whether the signed-in person gets "Built-in agents": an owner whose server offers "New
/// agent" (26 8.1) or lists a built-in agent, so a disabled runner does not hide the agents
/// that are already there. False while signed out and on any failure.

@ProviderFor(builtinAgentsAvailable)
final builtinAgentsAvailableProvider = BuiltinAgentsAvailableProvider._();

/// Whether the signed-in person gets "Built-in agents": an owner whose server offers "New
/// agent" (26 8.1) or lists a built-in agent, so a disabled runner does not hide the agents
/// that are already there. False while signed out and on any failure.

final class BuiltinAgentsAvailableProvider
    extends $FunctionalProvider<AsyncValue<bool>, bool, FutureOr<bool>>
    with $FutureModifier<bool>, $FutureProvider<bool> {
  /// Whether the signed-in person gets "Built-in agents": an owner whose server offers "New
  /// agent" (26 8.1) or lists a built-in agent, so a disabled runner does not hide the agents
  /// that are already there. False while signed out and on any failure.
  BuiltinAgentsAvailableProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'builtinAgentsAvailableProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$builtinAgentsAvailableHash();

  @$internal
  @override
  $FutureProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<bool> create(Ref ref) {
    return builtinAgentsAvailable(ref);
  }
}

String _$builtinAgentsAvailableHash() =>
    r'fd8710b2a01677c180bf38586e7ac47348f878ba';

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'emergency_entry.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The [EmergencyRole] of the signed-in person; [EmergencyRole.none] while signed out. Follows
/// the stored spaces, so a `manage` given or taken through `access.changed` shows or hides the
/// entry.

@ProviderFor(emergencyRole)
final emergencyRoleProvider = EmergencyRoleProvider._();

/// The [EmergencyRole] of the signed-in person; [EmergencyRole.none] while signed out. Follows
/// the stored spaces, so a `manage` given or taken through `access.changed` shows or hides the
/// entry.

final class EmergencyRoleProvider
    extends
        $FunctionalProvider<
          AsyncValue<EmergencyRole>,
          EmergencyRole,
          Stream<EmergencyRole>
        >
    with $FutureModifier<EmergencyRole>, $StreamProvider<EmergencyRole> {
  /// The [EmergencyRole] of the signed-in person; [EmergencyRole.none] while signed out. Follows
  /// the stored spaces, so a `manage` given or taken through `access.changed` shows or hides the
  /// entry.
  EmergencyRoleProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'emergencyRoleProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$emergencyRoleHash();

  @$internal
  @override
  $StreamProviderElement<EmergencyRole> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<EmergencyRole> create(Ref ref) {
    return emergencyRole(ref);
  }
}

String _$emergencyRoleHash() => r'53191ebbdcc4ef7368882039e1434866ed1df3dd';

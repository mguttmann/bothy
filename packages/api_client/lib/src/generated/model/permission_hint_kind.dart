//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'permission_hint_kind.g.dart';

/// Kind of a permission hint (07 section 9.5), in the order of the list.
class PermissionHintKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'h1_read_horizon')
  static const PermissionHintKind h1ReadHorizon = _$h1ReadHorizon;
  @BuiltValueEnumConst(wireName: r'grant_to_disabled')
  static const PermissionHintKind grantToDisabled = _$grantToDisabled;
  @BuiltValueEnumConst(wireName: r'trigger_blocked')
  static const PermissionHintKind triggerBlocked = _$triggerBlocked;
  @BuiltValueEnumConst(wireName: r'agent_personal')
  static const PermissionHintKind agentPersonal = _$agentPersonal;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const PermissionHintKind unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<PermissionHintKind> get serializer =>
      _$permissionHintKindSerializer;

  const PermissionHintKind._(String name) : super(name);

  static BuiltSet<PermissionHintKind> get values => _$values;
  static PermissionHintKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class PermissionHintKindMixin = Object with _$PermissionHintKindMixin;

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'capability.g.dart';

/// Additional right next to the level. `terminal` exists only on the target `host`.
class Capability extends EnumClass {
  @BuiltValueEnumConst(wireName: r'approve_actions')
  static const Capability approveActions = _$approveActions;
  @BuiltValueEnumConst(wireName: r'upload')
  static const Capability upload = _$upload;
  @BuiltValueEnumConst(wireName: r'export')
  static const Capability export_ = _$export_;
  @BuiltValueEnumConst(wireName: r'manage')
  static const Capability manage = _$manage;
  @BuiltValueEnumConst(wireName: r'terminal')
  static const Capability terminal = _$terminal;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const Capability unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<Capability> get serializer => _$capabilitySerializer;

  const Capability._(String name) : super(name);

  static BuiltSet<Capability> get values => _$values;
  static Capability valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class CapabilityMixin = Object with _$CapabilityMixin;

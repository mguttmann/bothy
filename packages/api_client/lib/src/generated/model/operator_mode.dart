//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'operator_mode.g.dart';

/// `inherit` takes the default operator of the space (channels only).
class OperatorMode extends EnumClass {
  @BuiltValueEnumConst(wireName: r'inherit')
  static const OperatorMode inherit = _$inherit;
  @BuiltValueEnumConst(wireName: r'none')
  static const OperatorMode none = _$none;
  @BuiltValueEnumConst(wireName: r'agent')
  static const OperatorMode agent = _$agent;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const OperatorMode unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<OperatorMode> get serializer => _$operatorModeSerializer;

  const OperatorMode._(String name) : super(name);

  static BuiltSet<OperatorMode> get values => _$values;
  static OperatorMode valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class OperatorModeMixin = Object with _$OperatorModeMixin;

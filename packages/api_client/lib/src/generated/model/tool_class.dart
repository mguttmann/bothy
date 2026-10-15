//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'tool_class.g.dart';

/// Tool class of an agent runtime, ordered `chat` < `scoped` < `host` (07, 2.11).
class ToolClass extends EnumClass {
  @BuiltValueEnumConst(wireName: r'chat')
  static const ToolClass chat = _$chat;
  @BuiltValueEnumConst(wireName: r'scoped')
  static const ToolClass scoped = _$scoped;
  @BuiltValueEnumConst(wireName: r'host')
  static const ToolClass host = _$host;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ToolClass unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<ToolClass> get serializer => _$toolClassSerializer;

  const ToolClass._(String name) : super(name);

  static BuiltSet<ToolClass> get values => _$values;
  static ToolClass valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class ToolClassMixin = Object with _$ToolClassMixin;

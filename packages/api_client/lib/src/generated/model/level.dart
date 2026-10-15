//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'level.g.dart';

/// Grant level. Ordered `read` < `write` < `trigger`.
class Level extends EnumClass {
  @BuiltValueEnumConst(wireName: r'read')
  static const Level read = _$read;
  @BuiltValueEnumConst(wireName: r'write')
  static const Level write = _$write;
  @BuiltValueEnumConst(wireName: r'trigger')
  static const Level trigger = _$trigger;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const Level unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<Level> get serializer => _$levelSerializer;

  const Level._(String name) : super(name);

  static BuiltSet<Level> get values => _$values;
  static Level valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class LevelMixin = Object with _$LevelMixin;

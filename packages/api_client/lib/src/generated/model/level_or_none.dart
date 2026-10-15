//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'level_or_none.g.dart';

/// Level including `none` for no access.
class LevelOrNone extends EnumClass {
  @BuiltValueEnumConst(wireName: r'none')
  static const LevelOrNone none = _$none;
  @BuiltValueEnumConst(wireName: r'read')
  static const LevelOrNone read = _$read;
  @BuiltValueEnumConst(wireName: r'write')
  static const LevelOrNone write = _$write;
  @BuiltValueEnumConst(wireName: r'trigger')
  static const LevelOrNone trigger = _$trigger;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const LevelOrNone unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<LevelOrNone> get serializer => _$levelOrNoneSerializer;

  const LevelOrNone._(String name) : super(name);

  static BuiltSet<LevelOrNone> get values => _$values;
  static LevelOrNone valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class LevelOrNoneMixin = Object with _$LevelOrNoneMixin;

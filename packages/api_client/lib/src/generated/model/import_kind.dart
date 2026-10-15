//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_kind.g.dart';

class ImportKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'discord')
  static const ImportKind discord = _$discord;
  @BuiltValueEnumConst(wireName: r'archive')
  static const ImportKind archive = _$archive;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ImportKind unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<ImportKind> get serializer => _$importKindSerializer;

  const ImportKind._(String name) : super(name);

  static BuiltSet<ImportKind> get values => _$values;
  static ImportKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class ImportKindMixin = Object with _$ImportKindMixin;

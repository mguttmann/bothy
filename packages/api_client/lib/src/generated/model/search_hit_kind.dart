//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_hit_kind.g.dart';

class SearchHitKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'message')
  static const SearchHitKind message = _$message;
  @BuiltValueEnumConst(wireName: r'thread')
  static const SearchHitKind thread = _$thread;
  @BuiltValueEnumConst(wireName: r'file')
  static const SearchHitKind file = _$file;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const SearchHitKind unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<SearchHitKind> get serializer => _$searchHitKindSerializer;

  const SearchHitKind._(String name) : super(name);

  static BuiltSet<SearchHitKind> get values => _$values;
  static SearchHitKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class SearchHitKindMixin = Object with _$SearchHitKindMixin;

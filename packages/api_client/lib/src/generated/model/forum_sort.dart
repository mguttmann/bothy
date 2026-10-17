//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'forum_sort.g.dart';

/// Sort order of forum posts.
class ForumSort extends EnumClass {
  @BuiltValueEnumConst(wireName: r'last_activity')
  static const ForumSort lastActivity = _$lastActivity;
  @BuiltValueEnumConst(wireName: r'created')
  static const ForumSort created = _$created;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ForumSort unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<ForumSort> get serializer => _$forumSortSerializer;

  const ForumSort._(String name) : super(name);

  static BuiltSet<ForumSort> get values => _$values;
  static ForumSort valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class ForumSortMixin = Object with _$ForumSortMixin;

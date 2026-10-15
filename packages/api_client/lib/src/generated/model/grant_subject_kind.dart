//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'grant_subject_kind.g.dart';

class GrantSubjectKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'user')
  static const GrantSubjectKind user = _$user;
  @BuiltValueEnumConst(wireName: r'group')
  static const GrantSubjectKind group = _$group;
  @BuiltValueEnumConst(wireName: r'agent')
  static const GrantSubjectKind agent = _$agent;
  @BuiltValueEnumConst(wireName: r'app')
  static const GrantSubjectKind app = _$app;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const GrantSubjectKind unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<GrantSubjectKind> get serializer =>
      _$grantSubjectKindSerializer;

  const GrantSubjectKind._(String name) : super(name);

  static BuiltSet<GrantSubjectKind> get values => _$values;
  static GrantSubjectKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class GrantSubjectKindMixin = Object with _$GrantSubjectKindMixin;

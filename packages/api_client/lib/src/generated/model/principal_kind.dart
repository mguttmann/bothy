//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'principal_kind.g.dart';

class PrincipalKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'user')
  static const PrincipalKind user = _$user;
  @BuiltValueEnumConst(wireName: r'agent')
  static const PrincipalKind agent = _$agent;
  @BuiltValueEnumConst(wireName: r'app')
  static const PrincipalKind app = _$app;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const PrincipalKind unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<PrincipalKind> get serializer => _$principalKindSerializer;

  const PrincipalKind._(String name) : super(name);

  static BuiltSet<PrincipalKind> get values => _$values;
  static PrincipalKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class PrincipalKindMixin = Object with _$PrincipalKindMixin;

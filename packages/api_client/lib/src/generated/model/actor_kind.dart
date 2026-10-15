//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'actor_kind.g.dart';

class ActorKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'user')
  static const ActorKind user = _$user;
  @BuiltValueEnumConst(wireName: r'agent')
  static const ActorKind agent = _$agent;
  @BuiltValueEnumConst(wireName: r'app')
  static const ActorKind app = _$app;
  @BuiltValueEnumConst(wireName: r'system')
  static const ActorKind system = _$system;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ActorKind unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<ActorKind> get serializer => _$actorKindSerializer;

  const ActorKind._(String name) : super(name);

  static BuiltSet<ActorKind> get values => _$values;
  static ActorKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class ActorKindMixin = Object with _$ActorKindMixin;

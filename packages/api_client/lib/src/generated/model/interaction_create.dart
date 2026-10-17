//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'interaction_create.g.dart';

/// InteractionCreate
///
/// Properties:
/// * [clientInteractionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [values] - Selected options or form values, validated against the component definition.
@BuiltValue()
abstract class InteractionCreate
    implements Built<InteractionCreate, InteractionCreateBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'client_interaction_id')
  String get clientInteractionId;

  /// Selected options or form values, validated against the component definition.
  @BuiltValueField(wireName: r'values')
  BuiltMap<String, JsonObject?>? get values;

  InteractionCreate._();

  factory InteractionCreate([void updates(InteractionCreateBuilder b)]) =
      _$InteractionCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InteractionCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<InteractionCreate> get serializer =>
      _$InteractionCreateSerializer();
}

class _$InteractionCreateSerializer
    implements PrimitiveSerializer<InteractionCreate> {
  @override
  final Iterable<Type> types = const [InteractionCreate, _$InteractionCreate];

  @override
  final String wireName = r'InteractionCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    InteractionCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'client_interaction_id';
    yield serializers.serialize(
      object.clientInteractionId,
      specifiedType: const FullType(String),
    );
    if (object.values != null) {
      yield r'values';
      yield serializers.serialize(
        object.values,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    InteractionCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return _serializeProperties(
      serializers,
      object,
      specifiedType: specifiedType,
    ).toList();
  }

  void _deserializeProperties(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
    required List<Object?> serializedList,
    required InteractionCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'client_interaction_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.clientInteractionId = valueDes;
          break;
        case r'values':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.values.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  InteractionCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InteractionCreateBuilder();
    final serializedList = (serialized as Iterable<Object?>).toList();
    final unhandled = <Object?>[];
    _deserializeProperties(
      serializers,
      serialized,
      specifiedType: specifiedType,
      serializedList: serializedList,
      unhandled: unhandled,
      result: result,
    );
    return result.build();
  }
}

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/interaction.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'interaction_created_payload.g.dart';

/// InteractionCreatedPayload
///
/// Properties:
/// * [interaction]
@BuiltValue()
abstract class InteractionCreatedPayload
    implements
        Built<InteractionCreatedPayload, InteractionCreatedPayloadBuilder> {
  @BuiltValueField(wireName: r'interaction')
  Interaction get interaction;

  InteractionCreatedPayload._();

  factory InteractionCreatedPayload([
    void updates(InteractionCreatedPayloadBuilder b),
  ]) = _$InteractionCreatedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InteractionCreatedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<InteractionCreatedPayload> get serializer =>
      _$InteractionCreatedPayloadSerializer();
}

class _$InteractionCreatedPayloadSerializer
    implements PrimitiveSerializer<InteractionCreatedPayload> {
  @override
  final Iterable<Type> types = const [
    InteractionCreatedPayload,
    _$InteractionCreatedPayload,
  ];

  @override
  final String wireName = r'InteractionCreatedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    InteractionCreatedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'interaction';
    yield serializers.serialize(
      object.interaction,
      specifiedType: const FullType(Interaction),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    InteractionCreatedPayload object, {
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
    required InteractionCreatedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'interaction':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Interaction),
          ) as Interaction;
          result.interaction.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  InteractionCreatedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InteractionCreatedPayloadBuilder();
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

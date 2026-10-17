//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/space.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'space_payload.g.dart';

/// SpacePayload
///
/// Properties:
/// * [space]
@BuiltValue()
abstract class SpacePayload
    implements Built<SpacePayload, SpacePayloadBuilder> {
  @BuiltValueField(wireName: r'space')
  Space get space;

  SpacePayload._();

  factory SpacePayload([void updates(SpacePayloadBuilder b)]) = _$SpacePayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SpacePayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SpacePayload> get serializer => _$SpacePayloadSerializer();
}

class _$SpacePayloadSerializer implements PrimitiveSerializer<SpacePayload> {
  @override
  final Iterable<Type> types = const [SpacePayload, _$SpacePayload];

  @override
  final String wireName = r'SpacePayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SpacePayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'space';
    yield serializers.serialize(
      object.space,
      specifiedType: const FullType(Space),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SpacePayload object, {
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
    required SpacePayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'space':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Space),
          ) as Space;
          result.space.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SpacePayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SpacePayloadBuilder();
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

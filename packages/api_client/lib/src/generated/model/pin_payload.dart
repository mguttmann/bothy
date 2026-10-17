//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/pin.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'pin_payload.g.dart';

/// PinPayload
///
/// Properties:
/// * [pin]
@BuiltValue()
abstract class PinPayload implements Built<PinPayload, PinPayloadBuilder> {
  @BuiltValueField(wireName: r'pin')
  Pin get pin;

  PinPayload._();

  factory PinPayload([void updates(PinPayloadBuilder b)]) = _$PinPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PinPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PinPayload> get serializer => _$PinPayloadSerializer();
}

class _$PinPayloadSerializer implements PrimitiveSerializer<PinPayload> {
  @override
  final Iterable<Type> types = const [PinPayload, _$PinPayload];

  @override
  final String wireName = r'PinPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PinPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'pin';
    yield serializers.serialize(object.pin, specifiedType: const FullType(Pin));
  }

  @override
  Object serialize(
    Serializers serializers,
    PinPayload object, {
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
    required PinPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'pin':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Pin),
          ) as Pin;
          result.pin.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PinPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PinPayloadBuilder();
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

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/read_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'read_state_payload.g.dart';

/// ReadStatePayload
///
/// Properties:
/// * [readState]
@BuiltValue()
abstract class ReadStatePayload
    implements Built<ReadStatePayload, ReadStatePayloadBuilder> {
  @BuiltValueField(wireName: r'read_state')
  ReadState get readState;

  ReadStatePayload._();

  factory ReadStatePayload([void updates(ReadStatePayloadBuilder b)]) =
      _$ReadStatePayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ReadStatePayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ReadStatePayload> get serializer =>
      _$ReadStatePayloadSerializer();
}

class _$ReadStatePayloadSerializer
    implements PrimitiveSerializer<ReadStatePayload> {
  @override
  final Iterable<Type> types = const [ReadStatePayload, _$ReadStatePayload];

  @override
  final String wireName = r'ReadStatePayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ReadStatePayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'read_state';
    yield serializers.serialize(
      object.readState,
      specifiedType: const FullType(ReadState),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ReadStatePayload object, {
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
    required ReadStatePayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'read_state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ReadState),
          ) as ReadState;
          result.readState.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ReadStatePayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ReadStatePayloadBuilder();
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

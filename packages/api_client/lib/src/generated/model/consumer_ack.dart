//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'consumer_ack.g.dart';

/// ConsumerAck
///
/// Properties:
/// * [seq]
@BuiltValue()
abstract class ConsumerAck implements Built<ConsumerAck, ConsumerAckBuilder> {
  @BuiltValueField(wireName: r'seq')
  int get seq;

  ConsumerAck._();

  factory ConsumerAck([void updates(ConsumerAckBuilder b)]) = _$ConsumerAck;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ConsumerAckBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ConsumerAck> get serializer => _$ConsumerAckSerializer();
}

class _$ConsumerAckSerializer implements PrimitiveSerializer<ConsumerAck> {
  @override
  final Iterable<Type> types = const [ConsumerAck, _$ConsumerAck];

  @override
  final String wireName = r'ConsumerAck';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ConsumerAck object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'seq';
    yield serializers.serialize(object.seq, specifiedType: const FullType(int));
  }

  @override
  Object serialize(
    Serializers serializers,
    ConsumerAck object, {
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
    required ConsumerAckBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.seq = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ConsumerAck deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ConsumerAckBuilder();
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

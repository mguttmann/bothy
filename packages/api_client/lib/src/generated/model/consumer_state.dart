//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'consumer_state.g.dart';

/// ConsumerState
///
/// Properties:
/// * [consumer]
/// * [ackedSeq]
/// * [headSeq]
@BuiltValue()
abstract class ConsumerState
    implements Built<ConsumerState, ConsumerStateBuilder> {
  @BuiltValueField(wireName: r'consumer')
  String get consumer;

  @BuiltValueField(wireName: r'acked_seq')
  int get ackedSeq;

  @BuiltValueField(wireName: r'head_seq')
  int get headSeq;

  ConsumerState._();

  factory ConsumerState([void updates(ConsumerStateBuilder b)]) =
      _$ConsumerState;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ConsumerStateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ConsumerState> get serializer =>
      _$ConsumerStateSerializer();
}

class _$ConsumerStateSerializer implements PrimitiveSerializer<ConsumerState> {
  @override
  final Iterable<Type> types = const [ConsumerState, _$ConsumerState];

  @override
  final String wireName = r'ConsumerState';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ConsumerState object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'consumer';
    yield serializers.serialize(
      object.consumer,
      specifiedType: const FullType(String),
    );
    yield r'acked_seq';
    yield serializers.serialize(
      object.ackedSeq,
      specifiedType: const FullType(int),
    );
    yield r'head_seq';
    yield serializers.serialize(
      object.headSeq,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ConsumerState object, {
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
    required ConsumerStateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'consumer':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.consumer = valueDes;
          break;
        case r'acked_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.ackedSeq = valueDes;
          break;
        case r'head_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.headSeq = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ConsumerState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ConsumerStateBuilder();
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

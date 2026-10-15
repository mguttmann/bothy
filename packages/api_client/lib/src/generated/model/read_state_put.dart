//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'read_state_put.g.dart';

/// Advances the read position. The server keeps the maximum of old and new.
///
/// Properties:
/// * [lastReadSeq]
@BuiltValue()
abstract class ReadStatePut
    implements Built<ReadStatePut, ReadStatePutBuilder> {
  @BuiltValueField(wireName: r'last_read_seq')
  int get lastReadSeq;

  ReadStatePut._();

  factory ReadStatePut([void updates(ReadStatePutBuilder b)]) = _$ReadStatePut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ReadStatePutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ReadStatePut> get serializer => _$ReadStatePutSerializer();
}

class _$ReadStatePutSerializer implements PrimitiveSerializer<ReadStatePut> {
  @override
  final Iterable<Type> types = const [ReadStatePut, _$ReadStatePut];

  @override
  final String wireName = r'ReadStatePut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ReadStatePut object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'last_read_seq';
    yield serializers.serialize(
      object.lastReadSeq,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ReadStatePut object, {
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
    required ReadStatePutBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'last_read_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.lastReadSeq = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ReadStatePut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ReadStatePutBuilder();
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

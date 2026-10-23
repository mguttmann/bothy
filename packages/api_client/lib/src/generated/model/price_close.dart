//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'price_close.g.dart';

/// The new end of the validity, after `valid_from`.
///
/// Properties:
/// * [validTo] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class PriceClose implements Built<PriceClose, PriceCloseBuilder> {
  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'valid_to')
  DateTime get validTo;

  PriceClose._();

  factory PriceClose([void updates(PriceCloseBuilder b)]) = _$PriceClose;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PriceCloseBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PriceClose> get serializer => _$PriceCloseSerializer();
}

class _$PriceCloseSerializer implements PrimitiveSerializer<PriceClose> {
  @override
  final Iterable<Type> types = const [PriceClose, _$PriceClose];

  @override
  final String wireName = r'PriceClose';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PriceClose object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'valid_to';
    yield serializers.serialize(
      object.validTo,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    PriceClose object, {
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
    required PriceCloseBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'valid_to':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.validTo = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PriceClose deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PriceCloseBuilder();
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

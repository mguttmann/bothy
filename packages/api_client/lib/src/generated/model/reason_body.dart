//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'reason_body.g.dart';

/// ReasonBody
///
/// Properties:
/// * [reason]
@BuiltValue()
abstract class ReasonBody implements Built<ReasonBody, ReasonBodyBuilder> {
  @BuiltValueField(wireName: r'reason')
  String get reason;

  ReasonBody._();

  factory ReasonBody([void updates(ReasonBodyBuilder b)]) = _$ReasonBody;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ReasonBodyBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ReasonBody> get serializer => _$ReasonBodySerializer();
}

class _$ReasonBodySerializer implements PrimitiveSerializer<ReasonBody> {
  @override
  final Iterable<Type> types = const [ReasonBody, _$ReasonBody];

  @override
  final String wireName = r'ReasonBody';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ReasonBody object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ReasonBody object, {
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
    required ReasonBodyBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ReasonBody deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ReasonBodyBuilder();
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

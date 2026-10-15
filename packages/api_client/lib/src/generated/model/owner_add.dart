//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'owner_add.g.dart';

/// OwnerAdd
///
/// Properties:
/// * [userId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [reason]
@BuiltValue()
abstract class OwnerAdd implements Built<OwnerAdd, OwnerAddBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'user_id')
  String get userId;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  OwnerAdd._();

  factory OwnerAdd([void updates(OwnerAddBuilder b)]) = _$OwnerAdd;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(OwnerAddBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<OwnerAdd> get serializer => _$OwnerAddSerializer();
}

class _$OwnerAddSerializer implements PrimitiveSerializer<OwnerAdd> {
  @override
  final Iterable<Type> types = const [OwnerAdd, _$OwnerAdd];

  @override
  final String wireName = r'OwnerAdd';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    OwnerAdd object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'user_id';
    yield serializers.serialize(
      object.userId,
      specifiedType: const FullType(String),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    OwnerAdd object, {
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
    required OwnerAddBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'user_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.userId = valueDes;
          break;
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
  OwnerAdd deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = OwnerAddBuilder();
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

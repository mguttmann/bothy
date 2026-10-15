//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/user.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'user_payload.g.dart';

/// UserPayload
///
/// Properties:
/// * [user]
@BuiltValue()
abstract class UserPayload implements Built<UserPayload, UserPayloadBuilder> {
  @BuiltValueField(wireName: r'user')
  User get user;

  UserPayload._();

  factory UserPayload([void updates(UserPayloadBuilder b)]) = _$UserPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(UserPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<UserPayload> get serializer => _$UserPayloadSerializer();
}

class _$UserPayloadSerializer implements PrimitiveSerializer<UserPayload> {
  @override
  final Iterable<Type> types = const [UserPayload, _$UserPayload];

  @override
  final String wireName = r'UserPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    UserPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'user';
    yield serializers.serialize(
      object.user,
      specifiedType: const FullType(User),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    UserPayload object, {
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
    required UserPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'user':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(User),
          ) as User;
          result.user.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  UserPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = UserPayloadBuilder();
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

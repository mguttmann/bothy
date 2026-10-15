//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/user.dart';
import 'package:api_client/src/generated/model/enrollment_code.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'user_created.g.dart';

/// UserCreated
///
/// Properties:
/// * [user]
/// * [enrollment]
@BuiltValue()
abstract class UserCreated implements Built<UserCreated, UserCreatedBuilder> {
  @BuiltValueField(wireName: r'user')
  User get user;

  @BuiltValueField(wireName: r'enrollment')
  EnrollmentCode? get enrollment;

  UserCreated._();

  factory UserCreated([void updates(UserCreatedBuilder b)]) = _$UserCreated;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(UserCreatedBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<UserCreated> get serializer => _$UserCreatedSerializer();
}

class _$UserCreatedSerializer implements PrimitiveSerializer<UserCreated> {
  @override
  final Iterable<Type> types = const [UserCreated, _$UserCreated];

  @override
  final String wireName = r'UserCreated';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    UserCreated object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'user';
    yield serializers.serialize(
      object.user,
      specifiedType: const FullType(User),
    );
    if (object.enrollment != null) {
      yield r'enrollment';
      yield serializers.serialize(
        object.enrollment,
        specifiedType: const FullType(EnrollmentCode),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    UserCreated object, {
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
    required UserCreatedBuilder result,
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
        case r'enrollment':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(EnrollmentCode),
          ) as EnrollmentCode?;
          if (valueDes == null) continue;
          result.enrollment.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  UserCreated deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = UserCreatedBuilder();
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

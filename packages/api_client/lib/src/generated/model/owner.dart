//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/principal_ref.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'owner.g.dart';

/// Owner
///
/// Properties:
/// * [user]
/// * [grantedBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [grantedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [reason]
@BuiltValue()
abstract class Owner implements Built<Owner, OwnerBuilder> {
  @BuiltValueField(wireName: r'user')
  PrincipalRef get user;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'granted_by')
  String? get grantedBy;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'granted_at')
  DateTime get grantedAt;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  Owner._();

  factory Owner([void updates(OwnerBuilder b)]) = _$Owner;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(OwnerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Owner> get serializer => _$OwnerSerializer();
}

class _$OwnerSerializer implements PrimitiveSerializer<Owner> {
  @override
  final Iterable<Type> types = const [Owner, _$Owner];

  @override
  final String wireName = r'Owner';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Owner object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'user';
    yield serializers.serialize(
      object.user,
      specifiedType: const FullType(PrincipalRef),
    );
    if (object.grantedBy != null) {
      yield r'granted_by';
      yield serializers.serialize(
        object.grantedBy,
        specifiedType: const FullType(String),
      );
    }
    yield r'granted_at';
    yield serializers.serialize(
      object.grantedAt,
      specifiedType: const FullType(DateTime),
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
    Owner object, {
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
    required OwnerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'user':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PrincipalRef),
          ) as PrincipalRef;
          result.user.replace(valueDes);
          break;
        case r'granted_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.grantedBy = valueDes;
          break;
        case r'granted_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.grantedAt = valueDes;
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
  Owner deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = OwnerBuilder();
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

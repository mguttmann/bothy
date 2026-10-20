//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/resolve_i1.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'group_member_add.g.dart';

/// GroupMemberAdd
///
/// Properties:
/// * [userId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [resolveI1]
@BuiltValue()
abstract class GroupMemberAdd
    implements Built<GroupMemberAdd, GroupMemberAddBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'user_id')
  String get userId;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime? get expiresAt;

  @BuiltValueField(wireName: r'resolve_i1')
  ResolveI1? get resolveI1;

  GroupMemberAdd._();

  factory GroupMemberAdd([void updates(GroupMemberAddBuilder b)]) =
      _$GroupMemberAdd;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GroupMemberAddBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GroupMemberAdd> get serializer =>
      _$GroupMemberAddSerializer();
}

class _$GroupMemberAddSerializer
    implements PrimitiveSerializer<GroupMemberAdd> {
  @override
  final Iterable<Type> types = const [GroupMemberAdd, _$GroupMemberAdd];

  @override
  final String wireName = r'GroupMemberAdd';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GroupMemberAdd object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'user_id';
    yield serializers.serialize(
      object.userId,
      specifiedType: const FullType(String),
    );
    if (object.expiresAt != null) {
      yield r'expires_at';
      yield serializers.serialize(
        object.expiresAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.resolveI1 != null) {
      yield r'resolve_i1';
      yield serializers.serialize(
        object.resolveI1,
        specifiedType: const FullType(ResolveI1),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    GroupMemberAdd object, {
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
    required GroupMemberAddBuilder result,
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
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.expiresAt = valueDes;
          break;
        case r'resolve_i1':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ResolveI1),
          ) as ResolveI1?;
          if (valueDes == null) continue;
          result.resolveI1.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GroupMemberAdd deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GroupMemberAddBuilder();
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

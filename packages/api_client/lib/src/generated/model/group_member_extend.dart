//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'group_member_extend.g.dart';

/// The later expiry of a time-limited membership in effect (`extendGroupMember`).
///
/// Properties:
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class GroupMemberExtend
    implements Built<GroupMemberExtend, GroupMemberExtendBuilder> {
  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  GroupMemberExtend._();

  factory GroupMemberExtend([void updates(GroupMemberExtendBuilder b)]) =
      _$GroupMemberExtend;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GroupMemberExtendBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GroupMemberExtend> get serializer =>
      _$GroupMemberExtendSerializer();
}

class _$GroupMemberExtendSerializer
    implements PrimitiveSerializer<GroupMemberExtend> {
  @override
  final Iterable<Type> types = const [GroupMemberExtend, _$GroupMemberExtend];

  @override
  final String wireName = r'GroupMemberExtend';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GroupMemberExtend object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GroupMemberExtend object, {
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
    required GroupMemberExtendBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GroupMemberExtend deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GroupMemberExtendBuilder();
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

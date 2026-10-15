//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/group.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'group_changed_payload.g.dart';

/// GroupChangedPayload
///
/// Properties:
/// * [group]
/// * [change] - `member_expired`: a membership ended by expiry, reported once by the job `authz.expiry_sweep`.
/// * [userId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class GroupChangedPayload
    implements Built<GroupChangedPayload, GroupChangedPayloadBuilder> {
  @BuiltValueField(wireName: r'group')
  Group get group;

  /// `member_expired`: a membership ended by expiry, reported once by the job `authz.expiry_sweep`.
  @BuiltValueField(wireName: r'change')
  GroupChangedPayloadChangeEnum get change;
  // enum changeEnum {  created,  updated,  deleted,  member_added,  member_removed,  member_expired,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'user_id')
  String? get userId;

  GroupChangedPayload._();

  factory GroupChangedPayload([void updates(GroupChangedPayloadBuilder b)]) =
      _$GroupChangedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GroupChangedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GroupChangedPayload> get serializer =>
      _$GroupChangedPayloadSerializer();
}

class _$GroupChangedPayloadSerializer
    implements PrimitiveSerializer<GroupChangedPayload> {
  @override
  final Iterable<Type> types = const [
    GroupChangedPayload,
    _$GroupChangedPayload,
  ];

  @override
  final String wireName = r'GroupChangedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GroupChangedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'group';
    yield serializers.serialize(
      object.group,
      specifiedType: const FullType(Group),
    );
    yield r'change';
    yield serializers.serialize(
      object.change,
      specifiedType: const FullType(GroupChangedPayloadChangeEnum),
    );
    if (object.userId != null) {
      yield r'user_id';
      yield serializers.serialize(
        object.userId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    GroupChangedPayload object, {
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
    required GroupChangedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'group':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Group),
          ) as Group;
          result.group.replace(valueDes);
          break;
        case r'change':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(GroupChangedPayloadChangeEnum),
          ) as GroupChangedPayloadChangeEnum;
          result.change = valueDes;
          break;
        case r'user_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.userId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GroupChangedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GroupChangedPayloadBuilder();
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

/// `member_expired`: a membership ended by expiry, reported once by the job `authz.expiry_sweep`.
class GroupChangedPayloadChangeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'created')
  static const GroupChangedPayloadChangeEnum created =
      _$groupChangedPayloadChangeEnum_created;
  @BuiltValueEnumConst(wireName: r'updated')
  static const GroupChangedPayloadChangeEnum updated =
      _$groupChangedPayloadChangeEnum_updated;
  @BuiltValueEnumConst(wireName: r'deleted')
  static const GroupChangedPayloadChangeEnum deleted =
      _$groupChangedPayloadChangeEnum_deleted;
  @BuiltValueEnumConst(wireName: r'member_added')
  static const GroupChangedPayloadChangeEnum memberAdded =
      _$groupChangedPayloadChangeEnum_memberAdded;
  @BuiltValueEnumConst(wireName: r'member_removed')
  static const GroupChangedPayloadChangeEnum memberRemoved =
      _$groupChangedPayloadChangeEnum_memberRemoved;
  @BuiltValueEnumConst(wireName: r'member_expired')
  static const GroupChangedPayloadChangeEnum memberExpired =
      _$groupChangedPayloadChangeEnum_memberExpired;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const GroupChangedPayloadChangeEnum unknownDefaultOpenApi =
      _$groupChangedPayloadChangeEnum_unknownDefaultOpenApi;

  static Serializer<GroupChangedPayloadChangeEnum> get serializer =>
      _$groupChangedPayloadChangeEnumSerializer;

  const GroupChangedPayloadChangeEnum._(String name) : super(name);

  static BuiltSet<GroupChangedPayloadChangeEnum> get values =>
      _$groupChangedPayloadChangeEnumValues;
  static GroupChangedPayloadChangeEnum valueOf(String name) =>
      _$groupChangedPayloadChangeEnumValueOf(name);
}

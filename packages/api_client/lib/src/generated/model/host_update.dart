//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/host_session_user_assign.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_update.g.dart';

/// Members that are absent stay unchanged. To remove the assignee send `clear_assigned_user` (no JSON null, ADR 0053).
///
/// Properties:
/// * [name]
/// * [assignedUserId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [clearAssignedUser]
/// * [addSessionUsers] - New session user assignments (ruling T12).
/// * [revokeSessionUsers] - People whose session user assignment ends; their names stay taken on this host.
/// * [reason]
@BuiltValue()
abstract class HostUpdate implements Built<HostUpdate, HostUpdateBuilder> {
  @BuiltValueField(wireName: r'name')
  String? get name;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'assigned_user_id')
  String? get assignedUserId;

  @BuiltValueField(wireName: r'clear_assigned_user')
  bool? get clearAssignedUser;

  /// New session user assignments (ruling T12).
  @BuiltValueField(wireName: r'add_session_users')
  BuiltList<HostSessionUserAssign>? get addSessionUsers;

  /// People whose session user assignment ends; their names stay taken on this host.
  @BuiltValueField(wireName: r'revoke_session_users')
  BuiltList<String>? get revokeSessionUsers;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  HostUpdate._();

  factory HostUpdate([void updates(HostUpdateBuilder b)]) = _$HostUpdate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostUpdateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostUpdate> get serializer => _$HostUpdateSerializer();
}

class _$HostUpdateSerializer implements PrimitiveSerializer<HostUpdate> {
  @override
  final Iterable<Type> types = const [HostUpdate, _$HostUpdate];

  @override
  final String wireName = r'HostUpdate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostUpdate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.name != null) {
      yield r'name';
      yield serializers.serialize(
        object.name,
        specifiedType: const FullType(String),
      );
    }
    if (object.assignedUserId != null) {
      yield r'assigned_user_id';
      yield serializers.serialize(
        object.assignedUserId,
        specifiedType: const FullType(String),
      );
    }
    if (object.clearAssignedUser != null) {
      yield r'clear_assigned_user';
      yield serializers.serialize(
        object.clearAssignedUser,
        specifiedType: const FullType(bool),
      );
    }
    if (object.addSessionUsers != null) {
      yield r'add_session_users';
      yield serializers.serialize(
        object.addSessionUsers,
        specifiedType: const FullType(BuiltList, [
          FullType(HostSessionUserAssign),
        ]),
      );
    }
    if (object.revokeSessionUsers != null) {
      yield r'revoke_session_users';
      yield serializers.serialize(
        object.revokeSessionUsers,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostUpdate object, {
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
    required HostUpdateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.name = valueDes;
          break;
        case r'assigned_user_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.assignedUserId = valueDes;
          break;
        case r'clear_assigned_user':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.clearAssignedUser = valueDes;
          break;
        case r'add_session_users':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(HostSessionUserAssign),
            ]),
          ) as BuiltList<HostSessionUserAssign>?;
          if (valueDes == null) continue;
          result.addSessionUsers.replace(valueDes);
          break;
        case r'revoke_session_users':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.revokeSessionUsers.replace(valueDes);
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
  HostUpdate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostUpdateBuilder();
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

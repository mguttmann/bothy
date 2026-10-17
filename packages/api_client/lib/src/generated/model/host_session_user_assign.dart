//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_session_user_assign.g.dart';

/// Assigns a person their own session user on a host (ruling T12, 24 section 2.2). The name must be one the host reported (`reported_session_users`); any other name gives `400 validation_failed` with `errors[].code` `session_user_not_on_host`, and nothing is stored.
///
/// Properties:
/// * [userId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [osUser] - Operating system user name from `host_service.session_users`; at most `host.os_user_max_chars` characters; never a path.
@BuiltValue()
abstract class HostSessionUserAssign
    implements Built<HostSessionUserAssign, HostSessionUserAssignBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'user_id')
  String get userId;

  /// Operating system user name from `host_service.session_users`; at most `host.os_user_max_chars` characters; never a path.
  @BuiltValueField(wireName: r'os_user')
  String get osUser;

  HostSessionUserAssign._();

  factory HostSessionUserAssign([
    void updates(HostSessionUserAssignBuilder b),
  ]) = _$HostSessionUserAssign;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostSessionUserAssignBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostSessionUserAssign> get serializer =>
      _$HostSessionUserAssignSerializer();
}

class _$HostSessionUserAssignSerializer
    implements PrimitiveSerializer<HostSessionUserAssign> {
  @override
  final Iterable<Type> types = const [
    HostSessionUserAssign,
    _$HostSessionUserAssign,
  ];

  @override
  final String wireName = r'HostSessionUserAssign';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostSessionUserAssign object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'user_id';
    yield serializers.serialize(
      object.userId,
      specifiedType: const FullType(String),
    );
    yield r'os_user';
    yield serializers.serialize(
      object.osUser,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostSessionUserAssign object, {
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
    required HostSessionUserAssignBuilder result,
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
        case r'os_user':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.osUser = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostSessionUserAssign deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostSessionUserAssignBuilder();
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

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/terminal_close_reason.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_session_close.g.dart';

/// HostSessionClose
///
/// Properties:
/// * [sessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [closeReason]
@BuiltValue()
abstract class HostSessionClose
    implements Built<HostSessionClose, HostSessionCloseBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'session_id')
  String get sessionId;

  @BuiltValueField(wireName: r'close_reason')
  TerminalCloseReason get closeReason;
  // enum closeReasonEnum {  client_closed,  closed_by_owner,  process_exited,  idle_timeout,  max_duration,  detach_expired,  access_revoked,  i6_violation,  host_disabled,  host_retired,  host_rekeyed,  host_restarted,  user_disabled,  device_revoked,  session_revoked,  logout,  open_failed,  host_policy_violation,  };

  HostSessionClose._();

  factory HostSessionClose([void updates(HostSessionCloseBuilder b)]) =
      _$HostSessionClose;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostSessionCloseBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostSessionClose> get serializer =>
      _$HostSessionCloseSerializer();
}

class _$HostSessionCloseSerializer
    implements PrimitiveSerializer<HostSessionClose> {
  @override
  final Iterable<Type> types = const [HostSessionClose, _$HostSessionClose];

  @override
  final String wireName = r'HostSessionClose';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostSessionClose object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'session_id';
    yield serializers.serialize(
      object.sessionId,
      specifiedType: const FullType(String),
    );
    yield r'close_reason';
    yield serializers.serialize(
      object.closeReason,
      specifiedType: const FullType(TerminalCloseReason),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostSessionClose object, {
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
    required HostSessionCloseBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'session_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sessionId = valueDes;
          break;
        case r'close_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(TerminalCloseReason),
          ) as TerminalCloseReason;
          result.closeReason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostSessionClose deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostSessionCloseBuilder();
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

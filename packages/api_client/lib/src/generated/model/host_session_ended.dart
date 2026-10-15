//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/terminal_close_reason.dart';
import 'package:api_client/src/generated/model/terminal_close_detail.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_session_ended.g.dart';

/// HostSessionEnded
///
/// Properties:
/// * [sessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [closeReason]
/// * [closeDetail]
/// * [exitCode]
@BuiltValue()
abstract class HostSessionEnded
    implements Built<HostSessionEnded, HostSessionEndedBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'session_id')
  String get sessionId;

  @BuiltValueField(wireName: r'close_reason')
  TerminalCloseReason get closeReason;
  // enum closeReasonEnum {  client_closed,  closed_by_owner,  process_exited,  idle_timeout,  max_duration,  detach_expired,  access_revoked,  i6_violation,  host_disabled,  host_retired,  host_rekeyed,  host_restarted,  user_disabled,  device_revoked,  session_revoked,  logout,  open_failed,  host_policy_violation,  };

  @BuiltValueField(wireName: r'close_detail')
  TerminalCloseDetail? get closeDetail;
  // enum closeDetailEnum {  token_invalid,  token_expired,  token_replayed,  handshake_failed,  profile_unknown,  session_user_unknown,  session_user_mismatch,  server_host_owner_only,  local_confirm_denied,  subscription_login_detected,  helper_untrusted,  shared_location,  pty_failed,  open_timeout,  };

  @BuiltValueField(wireName: r'exit_code')
  int? get exitCode;

  HostSessionEnded._();

  factory HostSessionEnded([void updates(HostSessionEndedBuilder b)]) =
      _$HostSessionEnded;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostSessionEndedBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostSessionEnded> get serializer =>
      _$HostSessionEndedSerializer();
}

class _$HostSessionEndedSerializer
    implements PrimitiveSerializer<HostSessionEnded> {
  @override
  final Iterable<Type> types = const [HostSessionEnded, _$HostSessionEnded];

  @override
  final String wireName = r'HostSessionEnded';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostSessionEnded object, {
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
    if (object.closeDetail != null) {
      yield r'close_detail';
      yield serializers.serialize(
        object.closeDetail,
        specifiedType: const FullType(TerminalCloseDetail),
      );
    }
    if (object.exitCode != null) {
      yield r'exit_code';
      yield serializers.serialize(
        object.exitCode,
        specifiedType: const FullType(int),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    HostSessionEnded object, {
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
    required HostSessionEndedBuilder result,
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
        case r'close_detail':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(TerminalCloseDetail),
          ) as TerminalCloseDetail?;
          if (valueDes == null) continue;
          result.closeDetail = valueDes;
          break;
        case r'exit_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.exitCode = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostSessionEnded deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostSessionEndedBuilder();
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

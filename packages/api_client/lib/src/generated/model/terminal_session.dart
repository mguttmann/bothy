//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/terminal_close_reason.dart';
import 'package:api_client/src/generated/model/terminal_close_detail.dart';
import 'package:api_client/src/generated/model/terminal_session_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'terminal_session.g.dart';

/// Metadata of a terminal session. Never content.
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [hostId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [userId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [deviceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [profile]
/// * [state]
/// * [openedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [startedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [detachedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [closedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [closeReason]
/// * [closeDetail]
/// * [exitCode]
/// * [bytesIn]
/// * [bytesOut]
/// * [version]
@BuiltValue()
abstract class TerminalSession
    implements Built<TerminalSession, TerminalSessionBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'host_id')
  String get hostId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'user_id')
  String get userId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'device_id')
  String get deviceId;

  @BuiltValueField(wireName: r'profile')
  String get profile;

  @BuiltValueField(wireName: r'state')
  TerminalSessionState get state;
  // enum stateEnum {  opening,  active,  detached,  closed,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'opened_at')
  DateTime get openedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'started_at')
  DateTime? get startedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'detached_at')
  DateTime? get detachedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'closed_at')
  DateTime? get closedAt;

  @BuiltValueField(wireName: r'close_reason')
  TerminalCloseReason? get closeReason;
  // enum closeReasonEnum {  client_closed,  closed_by_owner,  process_exited,  idle_timeout,  max_duration,  detach_expired,  access_revoked,  i6_violation,  host_disabled,  host_retired,  host_rekeyed,  host_restarted,  user_disabled,  device_revoked,  session_revoked,  logout,  open_failed,  host_policy_violation,  };

  @BuiltValueField(wireName: r'close_detail')
  TerminalCloseDetail? get closeDetail;
  // enum closeDetailEnum {  token_invalid,  token_expired,  token_replayed,  handshake_failed,  profile_unknown,  session_user_unknown,  session_user_mismatch,  server_host_owner_only,  local_confirm_denied,  subscription_login_detected,  helper_untrusted,  shared_location,  pty_failed,  open_timeout,  };

  @BuiltValueField(wireName: r'exit_code')
  int? get exitCode;

  @BuiltValueField(wireName: r'bytes_in')
  int get bytesIn;

  @BuiltValueField(wireName: r'bytes_out')
  int get bytesOut;

  @BuiltValueField(wireName: r'version')
  int get version;

  TerminalSession._();

  factory TerminalSession([void updates(TerminalSessionBuilder b)]) =
      _$TerminalSession;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TerminalSessionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TerminalSession> get serializer =>
      _$TerminalSessionSerializer();
}

class _$TerminalSessionSerializer
    implements PrimitiveSerializer<TerminalSession> {
  @override
  final Iterable<Type> types = const [TerminalSession, _$TerminalSession];

  @override
  final String wireName = r'TerminalSession';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TerminalSession object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'host_id';
    yield serializers.serialize(
      object.hostId,
      specifiedType: const FullType(String),
    );
    yield r'user_id';
    yield serializers.serialize(
      object.userId,
      specifiedType: const FullType(String),
    );
    yield r'device_id';
    yield serializers.serialize(
      object.deviceId,
      specifiedType: const FullType(String),
    );
    yield r'profile';
    yield serializers.serialize(
      object.profile,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(TerminalSessionState),
    );
    yield r'opened_at';
    yield serializers.serialize(
      object.openedAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.startedAt != null) {
      yield r'started_at';
      yield serializers.serialize(
        object.startedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.detachedAt != null) {
      yield r'detached_at';
      yield serializers.serialize(
        object.detachedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.closedAt != null) {
      yield r'closed_at';
      yield serializers.serialize(
        object.closedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.closeReason != null) {
      yield r'close_reason';
      yield serializers.serialize(
        object.closeReason,
        specifiedType: const FullType(TerminalCloseReason),
      );
    }
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
    yield r'bytes_in';
    yield serializers.serialize(
      object.bytesIn,
      specifiedType: const FullType(int),
    );
    yield r'bytes_out';
    yield serializers.serialize(
      object.bytesOut,
      specifiedType: const FullType(int),
    );
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    TerminalSession object, {
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
    required TerminalSessionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'host_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.hostId = valueDes;
          break;
        case r'user_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.userId = valueDes;
          break;
        case r'device_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.deviceId = valueDes;
          break;
        case r'profile':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.profile = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(TerminalSessionState),
          ) as TerminalSessionState;
          result.state = valueDes;
          break;
        case r'opened_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.openedAt = valueDes;
          break;
        case r'started_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.startedAt = valueDes;
          break;
        case r'detached_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.detachedAt = valueDes;
          break;
        case r'closed_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.closedAt = valueDes;
          break;
        case r'close_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(TerminalCloseReason),
          ) as TerminalCloseReason?;
          if (valueDes == null) continue;
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
        case r'bytes_in':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.bytesIn = valueDes;
          break;
        case r'bytes_out':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.bytesOut = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  TerminalSession deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TerminalSessionBuilder();
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

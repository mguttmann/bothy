//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/computer_close_reason.dart';
import 'package:api_client/src/generated/model/computer_session_state.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/computer_close_detail.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'computer_session.g.dart';

/// Metadata of a computer session (03 `computer_sessions`) with the name of its desktop. Never an image, a coordinate, typed text, a key name, the attach token or its `jti`.
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [hostId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [desktop] - The name of the host (`hosts.name`).
/// * [userId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state]
/// * [pauseReason]
/// * [generation] - The enablement generation the session was opened with (claim `gen`).
/// * [keyGeneration]
/// * [attachedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [openedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [startedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [pausedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [pausedMs]
/// * [closedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [stoppedBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [closeReason]
/// * [closeDetail]
/// * [actionCount]
/// * [frontApp] - Identity of the frontmost app of the last `computer.state` (bundle ID or executable name); never a window title.
/// * [version]
@BuiltValue()
abstract class ComputerSession
    implements Built<ComputerSession, ComputerSessionBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'host_id')
  String get hostId;

  /// The name of the host (`hosts.name`).
  @BuiltValueField(wireName: r'desktop')
  String get desktop;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'user_id')
  String get userId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  @BuiltValueField(wireName: r'state')
  ComputerSessionState get state;
  // enum stateEnum {  opening,  active,  paused,  closed,  };

  @BuiltValueField(wireName: r'pause_reason')
  ComputerSessionPauseReasonEnum? get pauseReason;
  // enum pauseReasonEnum {  locked,  };

  /// The enablement generation the session was opened with (claim `gen`).
  @BuiltValueField(wireName: r'generation')
  int get generation;

  @BuiltValueField(wireName: r'key_generation')
  int get keyGeneration;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'attached_at')
  DateTime? get attachedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'opened_at')
  DateTime get openedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'started_at')
  DateTime? get startedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'paused_at')
  DateTime? get pausedAt;

  @BuiltValueField(wireName: r'paused_ms')
  int get pausedMs;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'closed_at')
  DateTime? get closedAt;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'stopped_by')
  String? get stoppedBy;

  @BuiltValueField(wireName: r'close_reason')
  ComputerCloseReason? get closeReason;
  // enum closeReasonEnum {  done,  stopped_by_user,  device_disabled,  run_ended,  access_revoked,  host_disabled,  host_retired,  host_rekeyed,  host_restarted,  connection_lost,  open_failed,  error,  };

  @BuiltValueField(wireName: r'close_detail')
  ComputerCloseDetail? get closeDetail;
  // enum closeDetailEnum {  local_escape,  indicator_stop,  client_stop,  token_invalid,  token_expired,  token_replayed,  handshake_failed,  helper_absent,  helper_untrusted,  not_enabled,  busy,  open_timeout,  stream_fault,  helper_crashed,  };

  @BuiltValueField(wireName: r'action_count')
  int get actionCount;

  /// Identity of the frontmost app of the last `computer.state` (bundle ID or executable name); never a window title.
  @BuiltValueField(wireName: r'front_app')
  String? get frontApp;

  @BuiltValueField(wireName: r'version')
  int get version;

  ComputerSession._();

  factory ComputerSession([void updates(ComputerSessionBuilder b)]) =
      _$ComputerSession;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ComputerSessionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ComputerSession> get serializer =>
      _$ComputerSessionSerializer();
}

class _$ComputerSessionSerializer
    implements PrimitiveSerializer<ComputerSession> {
  @override
  final Iterable<Type> types = const [ComputerSession, _$ComputerSession];

  @override
  final String wireName = r'ComputerSession';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ComputerSession object, {
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
    yield r'desktop';
    yield serializers.serialize(
      object.desktop,
      specifiedType: const FullType(String),
    );
    yield r'user_id';
    yield serializers.serialize(
      object.userId,
      specifiedType: const FullType(String),
    );
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    yield r'run_id';
    yield serializers.serialize(
      object.runId,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(ComputerSessionState),
    );
    if (object.pauseReason != null) {
      yield r'pause_reason';
      yield serializers.serialize(
        object.pauseReason,
        specifiedType: const FullType(ComputerSessionPauseReasonEnum),
      );
    }
    yield r'generation';
    yield serializers.serialize(
      object.generation,
      specifiedType: const FullType(int),
    );
    yield r'key_generation';
    yield serializers.serialize(
      object.keyGeneration,
      specifiedType: const FullType(int),
    );
    if (object.attachedAt != null) {
      yield r'attached_at';
      yield serializers.serialize(
        object.attachedAt,
        specifiedType: const FullType(DateTime),
      );
    }
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
    if (object.pausedAt != null) {
      yield r'paused_at';
      yield serializers.serialize(
        object.pausedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'paused_ms';
    yield serializers.serialize(
      object.pausedMs,
      specifiedType: const FullType(int),
    );
    if (object.closedAt != null) {
      yield r'closed_at';
      yield serializers.serialize(
        object.closedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.stoppedBy != null) {
      yield r'stopped_by';
      yield serializers.serialize(
        object.stoppedBy,
        specifiedType: const FullType(String),
      );
    }
    if (object.closeReason != null) {
      yield r'close_reason';
      yield serializers.serialize(
        object.closeReason,
        specifiedType: const FullType(ComputerCloseReason),
      );
    }
    if (object.closeDetail != null) {
      yield r'close_detail';
      yield serializers.serialize(
        object.closeDetail,
        specifiedType: const FullType(ComputerCloseDetail),
      );
    }
    yield r'action_count';
    yield serializers.serialize(
      object.actionCount,
      specifiedType: const FullType(int),
    );
    if (object.frontApp != null) {
      yield r'front_app';
      yield serializers.serialize(
        object.frontApp,
        specifiedType: const FullType(String),
      );
    }
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ComputerSession object, {
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
    required ComputerSessionBuilder result,
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
        case r'desktop':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.desktop = valueDes;
          break;
        case r'user_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.userId = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.agentId = valueDes;
          break;
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.runId = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ComputerSessionState),
          ) as ComputerSessionState;
          result.state = valueDes;
          break;
        case r'pause_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(
              ComputerSessionPauseReasonEnum,
            ),
          ) as ComputerSessionPauseReasonEnum?;
          if (valueDes == null) continue;
          result.pauseReason = valueDes;
          break;
        case r'generation':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.generation = valueDes;
          break;
        case r'key_generation':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.keyGeneration = valueDes;
          break;
        case r'attached_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.attachedAt = valueDes;
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
        case r'paused_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.pausedAt = valueDes;
          break;
        case r'paused_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.pausedMs = valueDes;
          break;
        case r'closed_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.closedAt = valueDes;
          break;
        case r'stopped_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.stoppedBy = valueDes;
          break;
        case r'close_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ComputerCloseReason),
          ) as ComputerCloseReason?;
          if (valueDes == null) continue;
          result.closeReason = valueDes;
          break;
        case r'close_detail':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ComputerCloseDetail),
          ) as ComputerCloseDetail?;
          if (valueDes == null) continue;
          result.closeDetail = valueDes;
          break;
        case r'action_count':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.actionCount = valueDes;
          break;
        case r'front_app':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.frontApp = valueDes;
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
  ComputerSession deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ComputerSessionBuilder();
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

class ComputerSessionPauseReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'locked')
  static const ComputerSessionPauseReasonEnum locked =
      _$computerSessionPauseReasonEnum_locked;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ComputerSessionPauseReasonEnum unknownDefaultOpenApi =
      _$computerSessionPauseReasonEnum_unknownDefaultOpenApi;

  static Serializer<ComputerSessionPauseReasonEnum> get serializer =>
      _$computerSessionPauseReasonEnumSerializer;

  const ComputerSessionPauseReasonEnum._(String name) : super(name);

  static BuiltSet<ComputerSessionPauseReasonEnum> get values =>
      _$computerSessionPauseReasonEnumValues;
  static ComputerSessionPauseReasonEnum valueOf(String name) =>
      _$computerSessionPauseReasonEnumValueOf(name);
}

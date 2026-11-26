//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/computer_close_detail.dart';
import 'package:api_client/src/generated/model/host_computer_app.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_computer_ended.g.dart';

/// HostComputerEnded
///
/// Properties:
/// * [sessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [closeReason] - The host service sends only these reasons (27 12.3).
/// * [closeDetail]
/// * [actionCounts] - A count per action kind of 27 6.4.
/// * [apps] - Identities of the frontmost apps; the server keeps at most `computer.audit_apps_max` in its audit entry.
/// * [pausedMs]
@BuiltValue()
abstract class HostComputerEnded
    implements Built<HostComputerEnded, HostComputerEndedBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'session_id')
  String get sessionId;

  /// The host service sends only these reasons (27 12.3).
  @BuiltValueField(wireName: r'close_reason')
  HostComputerEndedCloseReasonEnum get closeReason;
  // enum closeReasonEnum {  done,  stopped_by_user,  device_disabled,  connection_lost,  open_failed,  error,  };

  @BuiltValueField(wireName: r'close_detail')
  ComputerCloseDetail? get closeDetail;
  // enum closeDetailEnum {  local_escape,  indicator_stop,  client_stop,  token_invalid,  token_expired,  token_replayed,  handshake_failed,  helper_absent,  helper_untrusted,  not_enabled,  busy,  open_timeout,  stream_fault,  helper_crashed,  };

  /// A count per action kind of 27 6.4.
  @BuiltValueField(wireName: r'action_counts')
  BuiltMap<String, int> get actionCounts;

  /// Identities of the frontmost apps; the server keeps at most `computer.audit_apps_max` in its audit entry.
  @BuiltValueField(wireName: r'apps')
  BuiltList<HostComputerApp> get apps;

  @BuiltValueField(wireName: r'paused_ms')
  int get pausedMs;

  HostComputerEnded._();

  factory HostComputerEnded([void updates(HostComputerEndedBuilder b)]) =
      _$HostComputerEnded;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostComputerEndedBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostComputerEnded> get serializer =>
      _$HostComputerEndedSerializer();
}

class _$HostComputerEndedSerializer
    implements PrimitiveSerializer<HostComputerEnded> {
  @override
  final Iterable<Type> types = const [HostComputerEnded, _$HostComputerEnded];

  @override
  final String wireName = r'HostComputerEnded';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostComputerEnded object, {
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
      specifiedType: const FullType(HostComputerEndedCloseReasonEnum),
    );
    if (object.closeDetail != null) {
      yield r'close_detail';
      yield serializers.serialize(
        object.closeDetail,
        specifiedType: const FullType(ComputerCloseDetail),
      );
    }
    yield r'action_counts';
    yield serializers.serialize(
      object.actionCounts,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType(int),
      ]),
    );
    yield r'apps';
    yield serializers.serialize(
      object.apps,
      specifiedType: const FullType(BuiltList, [FullType(HostComputerApp)]),
    );
    yield r'paused_ms';
    yield serializers.serialize(
      object.pausedMs,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostComputerEnded object, {
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
    required HostComputerEndedBuilder result,
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
            specifiedType: const FullType(HostComputerEndedCloseReasonEnum),
          ) as HostComputerEndedCloseReasonEnum;
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
        case r'action_counts':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType(int),
            ]),
          ) as BuiltMap<String, int>;
          result.actionCounts.replace(valueDes);
          break;
        case r'apps':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(HostComputerApp),
            ]),
          ) as BuiltList<HostComputerApp>;
          result.apps.replace(valueDes);
          break;
        case r'paused_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.pausedMs = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostComputerEnded deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostComputerEndedBuilder();
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

/// The host service sends only these reasons (27 12.3).
class HostComputerEndedCloseReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'done')
  static const HostComputerEndedCloseReasonEnum done =
      _$hostComputerEndedCloseReasonEnum_done;
  @BuiltValueEnumConst(wireName: r'stopped_by_user')
  static const HostComputerEndedCloseReasonEnum stoppedByUser =
      _$hostComputerEndedCloseReasonEnum_stoppedByUser;
  @BuiltValueEnumConst(wireName: r'device_disabled')
  static const HostComputerEndedCloseReasonEnum deviceDisabled =
      _$hostComputerEndedCloseReasonEnum_deviceDisabled;
  @BuiltValueEnumConst(wireName: r'connection_lost')
  static const HostComputerEndedCloseReasonEnum connectionLost =
      _$hostComputerEndedCloseReasonEnum_connectionLost;
  @BuiltValueEnumConst(wireName: r'open_failed')
  static const HostComputerEndedCloseReasonEnum openFailed =
      _$hostComputerEndedCloseReasonEnum_openFailed;
  @BuiltValueEnumConst(wireName: r'error')
  static const HostComputerEndedCloseReasonEnum error =
      _$hostComputerEndedCloseReasonEnum_error;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostComputerEndedCloseReasonEnum unknownDefaultOpenApi =
      _$hostComputerEndedCloseReasonEnum_unknownDefaultOpenApi;

  static Serializer<HostComputerEndedCloseReasonEnum> get serializer =>
      _$hostComputerEndedCloseReasonEnumSerializer;

  const HostComputerEndedCloseReasonEnum._(String name) : super(name);

  static BuiltSet<HostComputerEndedCloseReasonEnum> get values =>
      _$hostComputerEndedCloseReasonEnumValues;
  static HostComputerEndedCloseReasonEnum valueOf(String name) =>
      _$hostComputerEndedCloseReasonEnumValueOf(name);
}

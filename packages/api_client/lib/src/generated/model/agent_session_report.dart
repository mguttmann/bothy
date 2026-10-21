//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_session_report.g.dart';

/// AgentSessionReport
///
/// Properties:
/// * [externalSessionRef] - The reference the runtime actually uses; a different one replaces the stored reference of the session (09, 9.5 item 3). At most `agent.session_ref_max_bytes` bytes of display text.
/// * [model] - The pattern of `default_model` (09, 2.2), at most `agent.session_ref_max_bytes` bytes.
/// * [started] - `true` once the runtime confirmed the external session (09, 9.1); `started` only moves forward, `false` never resets it.
/// * [bindFailed] - The bound session no longer exists in the runtime and the adapter started a new one (09, 9.5 item 4); only for a started session, otherwise 409 `conflict` with `reason` `session_not_bound`.
@BuiltValue()
abstract class AgentSessionReport
    implements Built<AgentSessionReport, AgentSessionReportBuilder> {
  /// The reference the runtime actually uses; a different one replaces the stored reference of the session (09, 9.5 item 3). At most `agent.session_ref_max_bytes` bytes of display text.
  @BuiltValueField(wireName: r'external_session_ref')
  String get externalSessionRef;

  /// The pattern of `default_model` (09, 2.2), at most `agent.session_ref_max_bytes` bytes.
  @BuiltValueField(wireName: r'model')
  String? get model;

  /// `true` once the runtime confirmed the external session (09, 9.1); `started` only moves forward, `false` never resets it.
  @BuiltValueField(wireName: r'started')
  bool? get started;

  /// The bound session no longer exists in the runtime and the adapter started a new one (09, 9.5 item 4); only for a started session, otherwise 409 `conflict` with `reason` `session_not_bound`.
  @BuiltValueField(wireName: r'bind_failed')
  AgentSessionReportBindFailedEnum? get bindFailed;
  // enum bindFailedEnum {  session_missing,  };

  AgentSessionReport._();

  factory AgentSessionReport([void updates(AgentSessionReportBuilder b)]) =
      _$AgentSessionReport;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentSessionReportBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentSessionReport> get serializer =>
      _$AgentSessionReportSerializer();
}

class _$AgentSessionReportSerializer
    implements PrimitiveSerializer<AgentSessionReport> {
  @override
  final Iterable<Type> types = const [AgentSessionReport, _$AgentSessionReport];

  @override
  final String wireName = r'AgentSessionReport';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentSessionReport object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'external_session_ref';
    yield serializers.serialize(
      object.externalSessionRef,
      specifiedType: const FullType(String),
    );
    if (object.model != null) {
      yield r'model';
      yield serializers.serialize(
        object.model,
        specifiedType: const FullType(String),
      );
    }
    if (object.started != null) {
      yield r'started';
      yield serializers.serialize(
        object.started,
        specifiedType: const FullType(bool),
      );
    }
    if (object.bindFailed != null) {
      yield r'bind_failed';
      yield serializers.serialize(
        object.bindFailed,
        specifiedType: const FullType(AgentSessionReportBindFailedEnum),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentSessionReport object, {
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
    required AgentSessionReportBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'external_session_ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.externalSessionRef = valueDes;
          break;
        case r'model':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.model = valueDes;
          break;
        case r'started':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.started = valueDes;
          break;
        case r'bind_failed':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(
              AgentSessionReportBindFailedEnum,
            ),
          ) as AgentSessionReportBindFailedEnum?;
          if (valueDes == null) continue;
          result.bindFailed = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentSessionReport deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentSessionReportBuilder();
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

/// The bound session no longer exists in the runtime and the adapter started a new one (09, 9.5 item 4); only for a started session, otherwise 409 `conflict` with `reason` `session_not_bound`.
class AgentSessionReportBindFailedEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'session_missing')
  static const AgentSessionReportBindFailedEnum sessionMissing =
      _$agentSessionReportBindFailedEnum_sessionMissing;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentSessionReportBindFailedEnum unknownDefaultOpenApi =
      _$agentSessionReportBindFailedEnum_unknownDefaultOpenApi;

  static Serializer<AgentSessionReportBindFailedEnum> get serializer =>
      _$agentSessionReportBindFailedEnumSerializer;

  const AgentSessionReportBindFailedEnum._(String name) : super(name);

  static BuiltSet<AgentSessionReportBindFailedEnum> get values =>
      _$agentSessionReportBindFailedEnumValues;
  static AgentSessionReportBindFailedEnum valueOf(String name) =>
      _$agentSessionReportBindFailedEnumValueOf(name);
}

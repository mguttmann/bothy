//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/command_option_value.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:api_client/src/generated/model/principal_ref.dart';
import 'package:api_client/src/generated/model/agent_session.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_invocation_payload.g.dart';

/// Sent to the agent; the envelope carries a fresh `invocation_token` per delivery.
///
/// Properties:
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [spaceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [triggerKind]
/// * [messageId] - Triggering message; the adapter reads its content with `run.context.read` (09, 4.6). Events outside resource `message` never carry message content (04, 2.2).
/// * [interactionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [commandInvocationId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [commandId] - Triggering command; set exactly for `trigger_kind = command`.
/// * [commandName] - Name of the triggering command; set exactly for `trigger_kind = command`.
/// * [options] - Parsed option values in the order of the command definition; set exactly for `trigger_kind = command`.
/// * [triggeredBy]
/// * [billingMode]
/// * [agentSession]
/// * [settingsVersion] - Channel settings version; reload `agent.config.read` when newer than cached.
@BuiltValue()
abstract class AgentInvocationPayload
    implements Built<AgentInvocationPayload, AgentInvocationPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'space_id')
  String? get spaceId;

  @BuiltValueField(wireName: r'trigger_kind')
  AgentInvocationPayloadTriggerKindEnum get triggerKind;
  // enum triggerKindEnum {  message,  interaction,  command,  };

  /// Triggering message; the adapter reads its content with `run.context.read` (09, 4.6). Events outside resource `message` never carry message content (04, 2.2).
  @BuiltValueField(wireName: r'message_id')
  String? get messageId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'interaction_id')
  String? get interactionId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'command_invocation_id')
  String? get commandInvocationId;

  /// Triggering command; set exactly for `trigger_kind = command`.
  @BuiltValueField(wireName: r'command_id')
  String? get commandId;

  /// Name of the triggering command; set exactly for `trigger_kind = command`.
  @BuiltValueField(wireName: r'command_name')
  String? get commandName;

  /// Parsed option values in the order of the command definition; set exactly for `trigger_kind = command`.
  @BuiltValueField(wireName: r'options')
  BuiltList<CommandOptionValue>? get options;

  @BuiltValueField(wireName: r'triggered_by')
  PrincipalRef get triggeredBy;

  @BuiltValueField(wireName: r'billing_mode')
  BillingMode get billingMode;
  // enum billingModeEnum {  personal,  organization,  };

  @BuiltValueField(wireName: r'agent_session')
  AgentSession? get agentSession;

  /// Channel settings version; reload `agent.config.read` when newer than cached.
  @BuiltValueField(wireName: r'settings_version')
  int? get settingsVersion;

  AgentInvocationPayload._();

  factory AgentInvocationPayload([
    void updates(AgentInvocationPayloadBuilder b),
  ]) = _$AgentInvocationPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentInvocationPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentInvocationPayload> get serializer =>
      _$AgentInvocationPayloadSerializer();
}

class _$AgentInvocationPayloadSerializer
    implements PrimitiveSerializer<AgentInvocationPayload> {
  @override
  final Iterable<Type> types = const [
    AgentInvocationPayload,
    _$AgentInvocationPayload,
  ];

  @override
  final String wireName = r'AgentInvocationPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentInvocationPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'run_id';
    yield serializers.serialize(
      object.runId,
      specifiedType: const FullType(String),
    );
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    if (object.spaceId != null) {
      yield r'space_id';
      yield serializers.serialize(
        object.spaceId,
        specifiedType: const FullType(String),
      );
    }
    yield r'trigger_kind';
    yield serializers.serialize(
      object.triggerKind,
      specifiedType: const FullType(AgentInvocationPayloadTriggerKindEnum),
    );
    if (object.messageId != null) {
      yield r'message_id';
      yield serializers.serialize(
        object.messageId,
        specifiedType: const FullType(String),
      );
    }
    if (object.interactionId != null) {
      yield r'interaction_id';
      yield serializers.serialize(
        object.interactionId,
        specifiedType: const FullType(String),
      );
    }
    if (object.commandInvocationId != null) {
      yield r'command_invocation_id';
      yield serializers.serialize(
        object.commandInvocationId,
        specifiedType: const FullType(String),
      );
    }
    if (object.commandId != null) {
      yield r'command_id';
      yield serializers.serialize(
        object.commandId,
        specifiedType: const FullType(String),
      );
    }
    if (object.commandName != null) {
      yield r'command_name';
      yield serializers.serialize(
        object.commandName,
        specifiedType: const FullType(String),
      );
    }
    if (object.options != null) {
      yield r'options';
      yield serializers.serialize(
        object.options,
        specifiedType: const FullType(BuiltList, [
          FullType(CommandOptionValue),
        ]),
      );
    }
    yield r'triggered_by';
    yield serializers.serialize(
      object.triggeredBy,
      specifiedType: const FullType(PrincipalRef),
    );
    yield r'billing_mode';
    yield serializers.serialize(
      object.billingMode,
      specifiedType: const FullType(BillingMode),
    );
    if (object.agentSession != null) {
      yield r'agent_session';
      yield serializers.serialize(
        object.agentSession,
        specifiedType: const FullType(AgentSession),
      );
    }
    if (object.settingsVersion != null) {
      yield r'settings_version';
      yield serializers.serialize(
        object.settingsVersion,
        specifiedType: const FullType(int),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentInvocationPayload object, {
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
    required AgentInvocationPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.runId = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'space_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.spaceId = valueDes;
          break;
        case r'trigger_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(
              AgentInvocationPayloadTriggerKindEnum,
            ),
          ) as AgentInvocationPayloadTriggerKindEnum;
          result.triggerKind = valueDes;
          break;
        case r'message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.messageId = valueDes;
          break;
        case r'interaction_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.interactionId = valueDes;
          break;
        case r'command_invocation_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.commandInvocationId = valueDes;
          break;
        case r'command_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.commandId = valueDes;
          break;
        case r'command_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.commandName = valueDes;
          break;
        case r'options':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(CommandOptionValue),
            ]),
          ) as BuiltList<CommandOptionValue>?;
          if (valueDes == null) continue;
          result.options.replace(valueDes);
          break;
        case r'triggered_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PrincipalRef),
          ) as PrincipalRef;
          result.triggeredBy.replace(valueDes);
          break;
        case r'billing_mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BillingMode),
          ) as BillingMode;
          result.billingMode = valueDes;
          break;
        case r'agent_session':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AgentSession),
          ) as AgentSession?;
          if (valueDes == null) continue;
          result.agentSession.replace(valueDes);
          break;
        case r'settings_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.settingsVersion = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentInvocationPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentInvocationPayloadBuilder();
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

class AgentInvocationPayloadTriggerKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'message')
  static const AgentInvocationPayloadTriggerKindEnum message =
      _$agentInvocationPayloadTriggerKindEnum_message;
  @BuiltValueEnumConst(wireName: r'interaction')
  static const AgentInvocationPayloadTriggerKindEnum interaction =
      _$agentInvocationPayloadTriggerKindEnum_interaction;
  @BuiltValueEnumConst(wireName: r'command')
  static const AgentInvocationPayloadTriggerKindEnum command =
      _$agentInvocationPayloadTriggerKindEnum_command;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentInvocationPayloadTriggerKindEnum unknownDefaultOpenApi =
      _$agentInvocationPayloadTriggerKindEnum_unknownDefaultOpenApi;

  static Serializer<AgentInvocationPayloadTriggerKindEnum> get serializer =>
      _$agentInvocationPayloadTriggerKindEnumSerializer;

  const AgentInvocationPayloadTriggerKindEnum._(String name) : super(name);

  static BuiltSet<AgentInvocationPayloadTriggerKindEnum> get values =>
      _$agentInvocationPayloadTriggerKindEnumValues;
  static AgentInvocationPayloadTriggerKindEnum valueOf(String name) =>
      _$agentInvocationPayloadTriggerKindEnumValueOf(name);
}

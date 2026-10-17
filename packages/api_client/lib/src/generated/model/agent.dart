//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/tool_class.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:api_client/src/generated/model/agent_runtime_status.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent.g.dart';

/// Agent
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [handle]
/// * [displayName]
/// * [avatarFileId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [status]
/// * [adapterKind]
/// * [description]
/// * [billingMode]
/// * [credentialHolderId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [billingModeChangedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [toolClass]
/// * [presence]
/// * [operatorConfig] - Schema `agent_operator_config.v1`. No secrets.
/// * [version]
@BuiltValue()
abstract class Agent implements Built<Agent, AgentBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'handle')
  String get handle;

  @BuiltValueField(wireName: r'display_name')
  String get displayName;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'avatar_file_id')
  String? get avatarFileId;

  @BuiltValueField(wireName: r'status')
  AgentStatusEnum get status;
  // enum statusEnum {  active,  disabled,  deleted,  };

  @BuiltValueField(wireName: r'adapter_kind')
  String get adapterKind;

  @BuiltValueField(wireName: r'description')
  String get description;

  @BuiltValueField(wireName: r'billing_mode')
  BillingMode get billingMode;
  // enum billingModeEnum {  personal,  organization,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'credential_holder_id')
  String? get credentialHolderId;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'billing_mode_changed_at')
  DateTime? get billingModeChangedAt;

  @BuiltValueField(wireName: r'tool_class')
  ToolClass get toolClass;
  // enum toolClassEnum {  chat,  scoped,  host,  };

  @BuiltValueField(wireName: r'presence')
  AgentRuntimeStatus get presence;

  /// Schema `agent_operator_config.v1`. No secrets.
  @BuiltValueField(wireName: r'operator_config')
  BuiltMap<String, JsonObject?> get operatorConfig;

  @BuiltValueField(wireName: r'version')
  int get version;

  Agent._();

  factory Agent([void updates(AgentBuilder b)]) = _$Agent;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Agent> get serializer => _$AgentSerializer();
}

class _$AgentSerializer implements PrimitiveSerializer<Agent> {
  @override
  final Iterable<Type> types = const [Agent, _$Agent];

  @override
  final String wireName = r'Agent';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Agent object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'handle';
    yield serializers.serialize(
      object.handle,
      specifiedType: const FullType(String),
    );
    yield r'display_name';
    yield serializers.serialize(
      object.displayName,
      specifiedType: const FullType(String),
    );
    if (object.avatarFileId != null) {
      yield r'avatar_file_id';
      yield serializers.serialize(
        object.avatarFileId,
        specifiedType: const FullType(String),
      );
    }
    yield r'status';
    yield serializers.serialize(
      object.status,
      specifiedType: const FullType(AgentStatusEnum),
    );
    yield r'adapter_kind';
    yield serializers.serialize(
      object.adapterKind,
      specifiedType: const FullType(String),
    );
    yield r'description';
    yield serializers.serialize(
      object.description,
      specifiedType: const FullType(String),
    );
    yield r'billing_mode';
    yield serializers.serialize(
      object.billingMode,
      specifiedType: const FullType(BillingMode),
    );
    if (object.credentialHolderId != null) {
      yield r'credential_holder_id';
      yield serializers.serialize(
        object.credentialHolderId,
        specifiedType: const FullType(String),
      );
    }
    if (object.billingModeChangedAt != null) {
      yield r'billing_mode_changed_at';
      yield serializers.serialize(
        object.billingModeChangedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'tool_class';
    yield serializers.serialize(
      object.toolClass,
      specifiedType: const FullType(ToolClass),
    );
    yield r'presence';
    yield serializers.serialize(
      object.presence,
      specifiedType: const FullType(AgentRuntimeStatus),
    );
    yield r'operator_config';
    yield serializers.serialize(
      object.operatorConfig,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
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
    Agent object, {
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
    required AgentBuilder result,
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
        case r'handle':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.handle = valueDes;
          break;
        case r'display_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.displayName = valueDes;
          break;
        case r'avatar_file_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.avatarFileId = valueDes;
          break;
        case r'status':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentStatusEnum),
          ) as AgentStatusEnum;
          result.status = valueDes;
          break;
        case r'adapter_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.adapterKind = valueDes;
          break;
        case r'description':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.description = valueDes;
          break;
        case r'billing_mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BillingMode),
          ) as BillingMode;
          result.billingMode = valueDes;
          break;
        case r'credential_holder_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.credentialHolderId = valueDes;
          break;
        case r'billing_mode_changed_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.billingModeChangedAt = valueDes;
          break;
        case r'tool_class':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ToolClass),
          ) as ToolClass;
          result.toolClass = valueDes;
          break;
        case r'presence':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentRuntimeStatus),
          ) as AgentRuntimeStatus;
          result.presence.replace(valueDes);
          break;
        case r'operator_config':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.operatorConfig.replace(valueDes);
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
  Agent deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentBuilder();
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

class AgentStatusEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'active')
  static const AgentStatusEnum active = _$agentStatusEnum_active;
  @BuiltValueEnumConst(wireName: r'disabled')
  static const AgentStatusEnum disabled = _$agentStatusEnum_disabled;
  @BuiltValueEnumConst(wireName: r'deleted')
  static const AgentStatusEnum deleted = _$agentStatusEnum_deleted;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentStatusEnum unknownDefaultOpenApi =
      _$agentStatusEnum_unknownDefaultOpenApi;

  static Serializer<AgentStatusEnum> get serializer =>
      _$agentStatusEnumSerializer;

  const AgentStatusEnum._(String name) : super(name);

  static BuiltSet<AgentStatusEnum> get values => _$agentStatusEnumValues;
  static AgentStatusEnum valueOf(String name) => _$agentStatusEnumValueOf(name);
}

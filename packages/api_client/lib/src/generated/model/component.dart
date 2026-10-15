//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/component_restriction.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/capability.dart';
import 'package:api_client/src/generated/model/component_type.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'component.g.dart';

/// Component
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [customId]
/// * [type]
/// * [definition] - Schema `component.v1` (label, style, options, form fields).
/// * [position]
/// * [singleUse]
/// * [requiredCapability]
/// * [restrictedTo]
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state]
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [usedBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [usedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [version]
@BuiltValue()
abstract class Component implements Built<Component, ComponentBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'custom_id')
  String get customId;

  @BuiltValueField(wireName: r'type')
  ComponentType get type;
  // enum typeEnum {  button,  select,  form,  };

  /// Schema `component.v1` (label, style, options, form fields).
  @BuiltValueField(wireName: r'definition')
  BuiltMap<String, JsonObject?> get definition;

  @BuiltValueField(wireName: r'position')
  int get position;

  @BuiltValueField(wireName: r'single_use')
  bool get singleUse;

  @BuiltValueField(wireName: r'required_capability')
  Capability? get requiredCapability;
  // enum requiredCapabilityEnum {  approve_actions,  upload,  export,  manage,  terminal,  };

  @BuiltValueField(wireName: r'restricted_to')
  ComponentRestriction get restrictedTo;
  // enum restrictedToEnum {  any,  run_starter,  owner,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String? get runId;

  @BuiltValueField(wireName: r'state')
  ComponentStateEnum get state;
  // enum stateEnum {  active,  used,  expired,  disabled,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime? get expiresAt;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'used_by')
  String? get usedBy;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'used_at')
  DateTime? get usedAt;

  @BuiltValueField(wireName: r'version')
  int get version;

  Component._();

  factory Component([void updates(ComponentBuilder b)]) = _$Component;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ComponentBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Component> get serializer => _$ComponentSerializer();
}

class _$ComponentSerializer implements PrimitiveSerializer<Component> {
  @override
  final Iterable<Type> types = const [Component, _$Component];

  @override
  final String wireName = r'Component';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Component object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'custom_id';
    yield serializers.serialize(
      object.customId,
      specifiedType: const FullType(String),
    );
    yield r'type';
    yield serializers.serialize(
      object.type,
      specifiedType: const FullType(ComponentType),
    );
    yield r'definition';
    yield serializers.serialize(
      object.definition,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
    yield r'position';
    yield serializers.serialize(
      object.position,
      specifiedType: const FullType(int),
    );
    yield r'single_use';
    yield serializers.serialize(
      object.singleUse,
      specifiedType: const FullType(bool),
    );
    if (object.requiredCapability != null) {
      yield r'required_capability';
      yield serializers.serialize(
        object.requiredCapability,
        specifiedType: const FullType(Capability),
      );
    }
    yield r'restricted_to';
    yield serializers.serialize(
      object.restrictedTo,
      specifiedType: const FullType(ComponentRestriction),
    );
    if (object.runId != null) {
      yield r'run_id';
      yield serializers.serialize(
        object.runId,
        specifiedType: const FullType(String),
      );
    }
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(ComponentStateEnum),
    );
    if (object.expiresAt != null) {
      yield r'expires_at';
      yield serializers.serialize(
        object.expiresAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.usedBy != null) {
      yield r'used_by';
      yield serializers.serialize(
        object.usedBy,
        specifiedType: const FullType(String),
      );
    }
    if (object.usedAt != null) {
      yield r'used_at';
      yield serializers.serialize(
        object.usedAt,
        specifiedType: const FullType(DateTime),
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
    Component object, {
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
    required ComponentBuilder result,
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
        case r'custom_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.customId = valueDes;
          break;
        case r'type':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ComponentType),
          ) as ComponentType;
          result.type = valueDes;
          break;
        case r'definition':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.definition.replace(valueDes);
          break;
        case r'position':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.position = valueDes;
          break;
        case r'single_use':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.singleUse = valueDes;
          break;
        case r'required_capability':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(Capability),
          ) as Capability?;
          if (valueDes == null) continue;
          result.requiredCapability = valueDes;
          break;
        case r'restricted_to':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ComponentRestriction),
          ) as ComponentRestriction;
          result.restrictedTo = valueDes;
          break;
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.runId = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ComponentStateEnum),
          ) as ComponentStateEnum;
          result.state = valueDes;
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.expiresAt = valueDes;
          break;
        case r'used_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.usedBy = valueDes;
          break;
        case r'used_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.usedAt = valueDes;
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
  Component deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ComponentBuilder();
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

class ComponentStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'active')
  static const ComponentStateEnum active = _$componentStateEnum_active;
  @BuiltValueEnumConst(wireName: r'used')
  static const ComponentStateEnum used = _$componentStateEnum_used;
  @BuiltValueEnumConst(wireName: r'expired')
  static const ComponentStateEnum expired = _$componentStateEnum_expired;
  @BuiltValueEnumConst(wireName: r'disabled')
  static const ComponentStateEnum disabled = _$componentStateEnum_disabled;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ComponentStateEnum unknownDefaultOpenApi =
      _$componentStateEnum_unknownDefaultOpenApi;

  static Serializer<ComponentStateEnum> get serializer =>
      _$componentStateEnumSerializer;

  const ComponentStateEnum._(String name) : super(name);

  static BuiltSet<ComponentStateEnum> get values => _$componentStateEnumValues;
  static ComponentStateEnum valueOf(String name) =>
      _$componentStateEnumValueOf(name);
}

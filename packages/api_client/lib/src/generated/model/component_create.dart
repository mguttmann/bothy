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

part 'component_create.g.dart';

/// ComponentCreate
///
/// Properties:
/// * [customId]
/// * [type]
/// * [definition] - Schema `component.v1` (09, 11.1), a closed member set per type; `400 validation_failed` with one entry per finding below `/components/<index>/definition` and the codes `required`, `invalid` or `limit_reached`.
/// * [singleUse]
/// * [requiredCapability]
/// * [restrictedTo]
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class ComponentCreate
    implements Built<ComponentCreate, ComponentCreateBuilder> {
  @BuiltValueField(wireName: r'custom_id')
  String get customId;

  @BuiltValueField(wireName: r'type')
  ComponentType get type;
  // enum typeEnum {  button,  select,  form,  };

  /// Schema `component.v1` (09, 11.1), a closed member set per type; `400 validation_failed` with one entry per finding below `/components/<index>/definition` and the codes `required`, `invalid` or `limit_reached`.
  @BuiltValueField(wireName: r'definition')
  BuiltMap<String, JsonObject?> get definition;

  @BuiltValueField(wireName: r'single_use')
  bool? get singleUse;

  @BuiltValueField(wireName: r'required_capability')
  Capability? get requiredCapability;
  // enum requiredCapabilityEnum {  approve_actions,  upload,  export,  manage,  terminal,  };

  @BuiltValueField(wireName: r'restricted_to')
  ComponentRestriction? get restrictedTo;
  // enum restrictedToEnum {  any,  run_starter,  owner,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime? get expiresAt;

  ComponentCreate._();

  factory ComponentCreate([void updates(ComponentCreateBuilder b)]) =
      _$ComponentCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ComponentCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ComponentCreate> get serializer =>
      _$ComponentCreateSerializer();
}

class _$ComponentCreateSerializer
    implements PrimitiveSerializer<ComponentCreate> {
  @override
  final Iterable<Type> types = const [ComponentCreate, _$ComponentCreate];

  @override
  final String wireName = r'ComponentCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ComponentCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
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
    if (object.singleUse != null) {
      yield r'single_use';
      yield serializers.serialize(
        object.singleUse,
        specifiedType: const FullType(bool),
      );
    }
    if (object.requiredCapability != null) {
      yield r'required_capability';
      yield serializers.serialize(
        object.requiredCapability,
        specifiedType: const FullType(Capability),
      );
    }
    if (object.restrictedTo != null) {
      yield r'restricted_to';
      yield serializers.serialize(
        object.restrictedTo,
        specifiedType: const FullType(ComponentRestriction),
      );
    }
    if (object.expiresAt != null) {
      yield r'expires_at';
      yield serializers.serialize(
        object.expiresAt,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ComponentCreate object, {
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
    required ComponentCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
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
        case r'single_use':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
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
            specifiedType: const FullType.nullable(ComponentRestriction),
          ) as ComponentRestriction?;
          if (valueDes == null) continue;
          result.restrictedTo = valueDes;
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.expiresAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ComponentCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ComponentCreateBuilder();
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

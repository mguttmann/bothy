//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'component_update.g.dart';

/// ComponentUpdate
///
/// Properties:
/// * [state]
/// * [definition] - A new `component.v1` object for the type of the component (09, 11.1); it may change in every state. Type, `custom_id`, `single_use`, `required_capability`, `restricted_to` and `run_id` cannot change.
@BuiltValue()
abstract class ComponentUpdate
    implements Built<ComponentUpdate, ComponentUpdateBuilder> {
  @BuiltValueField(wireName: r'state')
  ComponentUpdateStateEnum? get state;
  // enum stateEnum {  active,  disabled,  };

  /// A new `component.v1` object for the type of the component (09, 11.1); it may change in every state. Type, `custom_id`, `single_use`, `required_capability`, `restricted_to` and `run_id` cannot change.
  @BuiltValueField(wireName: r'definition')
  BuiltMap<String, JsonObject?>? get definition;

  ComponentUpdate._();

  factory ComponentUpdate([void updates(ComponentUpdateBuilder b)]) =
      _$ComponentUpdate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ComponentUpdateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ComponentUpdate> get serializer =>
      _$ComponentUpdateSerializer();
}

class _$ComponentUpdateSerializer
    implements PrimitiveSerializer<ComponentUpdate> {
  @override
  final Iterable<Type> types = const [ComponentUpdate, _$ComponentUpdate];

  @override
  final String wireName = r'ComponentUpdate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ComponentUpdate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.state != null) {
      yield r'state';
      yield serializers.serialize(
        object.state,
        specifiedType: const FullType(ComponentUpdateStateEnum),
      );
    }
    if (object.definition != null) {
      yield r'definition';
      yield serializers.serialize(
        object.definition,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ComponentUpdate object, {
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
    required ComponentUpdateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ComponentUpdateStateEnum),
          ) as ComponentUpdateStateEnum?;
          if (valueDes == null) continue;
          result.state = valueDes;
          break;
        case r'definition':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.definition.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ComponentUpdate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ComponentUpdateBuilder();
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

class ComponentUpdateStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'active')
  static const ComponentUpdateStateEnum active =
      _$componentUpdateStateEnum_active;
  @BuiltValueEnumConst(wireName: r'disabled')
  static const ComponentUpdateStateEnum disabled =
      _$componentUpdateStateEnum_disabled;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ComponentUpdateStateEnum unknownDefaultOpenApi =
      _$componentUpdateStateEnum_unknownDefaultOpenApi;

  static Serializer<ComponentUpdateStateEnum> get serializer =>
      _$componentUpdateStateEnumSerializer;

  const ComponentUpdateStateEnum._(String name) : super(name);

  static BuiltSet<ComponentUpdateStateEnum> get values =>
      _$componentUpdateStateEnumValues;
  static ComponentUpdateStateEnum valueOf(String name) =>
      _$componentUpdateStateEnumValueOf(name);
}

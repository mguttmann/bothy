//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_update.g.dart';

/// AgentUpdate
///
/// Properties:
/// * [displayName]
/// * [description]
/// * [operatorConfig] - Free-form JSON object whose schema is named in the field description.
@BuiltValue()
abstract class AgentUpdate implements Built<AgentUpdate, AgentUpdateBuilder> {
  @BuiltValueField(wireName: r'display_name')
  String? get displayName;

  @BuiltValueField(wireName: r'description')
  String? get description;

  /// Free-form JSON object whose schema is named in the field description.
  @BuiltValueField(wireName: r'operator_config')
  BuiltMap<String, JsonObject?>? get operatorConfig;

  AgentUpdate._();

  factory AgentUpdate([void updates(AgentUpdateBuilder b)]) = _$AgentUpdate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentUpdateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentUpdate> get serializer => _$AgentUpdateSerializer();
}

class _$AgentUpdateSerializer implements PrimitiveSerializer<AgentUpdate> {
  @override
  final Iterable<Type> types = const [AgentUpdate, _$AgentUpdate];

  @override
  final String wireName = r'AgentUpdate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentUpdate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.displayName != null) {
      yield r'display_name';
      yield serializers.serialize(
        object.displayName,
        specifiedType: const FullType(String),
      );
    }
    if (object.description != null) {
      yield r'description';
      yield serializers.serialize(
        object.description,
        specifiedType: const FullType(String),
      );
    }
    if (object.operatorConfig != null) {
      yield r'operator_config';
      yield serializers.serialize(
        object.operatorConfig,
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
    AgentUpdate object, {
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
    required AgentUpdateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'display_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.displayName = valueDes;
          break;
        case r'description':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.description = valueDes;
          break;
        case r'operator_config':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.operatorConfig.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentUpdate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentUpdateBuilder();
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

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_definition_changed_payload.g.dart';

/// The new definition version of the receiving built-in agent; never the definition (26 4.1).
///
/// Properties:
/// * [definitionVersion]
@BuiltValue()
abstract class AgentDefinitionChangedPayload
    implements
        Built<
          AgentDefinitionChangedPayload,
          AgentDefinitionChangedPayloadBuilder
        > {
  @BuiltValueField(wireName: r'definition_version')
  int get definitionVersion;

  AgentDefinitionChangedPayload._();

  factory AgentDefinitionChangedPayload([
    void updates(AgentDefinitionChangedPayloadBuilder b),
  ]) = _$AgentDefinitionChangedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentDefinitionChangedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentDefinitionChangedPayload> get serializer =>
      _$AgentDefinitionChangedPayloadSerializer();
}

class _$AgentDefinitionChangedPayloadSerializer
    implements PrimitiveSerializer<AgentDefinitionChangedPayload> {
  @override
  final Iterable<Type> types = const [
    AgentDefinitionChangedPayload,
    _$AgentDefinitionChangedPayload,
  ];

  @override
  final String wireName = r'AgentDefinitionChangedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentDefinitionChangedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'definition_version';
    yield serializers.serialize(
      object.definitionVersion,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinitionChangedPayload object, {
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
    required AgentDefinitionChangedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'definition_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.definitionVersion = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentDefinitionChangedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentDefinitionChangedPayloadBuilder();
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

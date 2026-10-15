//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_triggerers.g.dart';

/// Principals that may trigger the calling agent (allow list of the adapter).
///
/// Properties:
/// * [principalIds]
/// * [permissionVersion]
@BuiltValue()
abstract class AgentTriggerers
    implements Built<AgentTriggerers, AgentTriggerersBuilder> {
  @BuiltValueField(wireName: r'principal_ids')
  BuiltList<String> get principalIds;

  @BuiltValueField(wireName: r'permission_version')
  int get permissionVersion;

  AgentTriggerers._();

  factory AgentTriggerers([void updates(AgentTriggerersBuilder b)]) =
      _$AgentTriggerers;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentTriggerersBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentTriggerers> get serializer =>
      _$AgentTriggerersSerializer();
}

class _$AgentTriggerersSerializer
    implements PrimitiveSerializer<AgentTriggerers> {
  @override
  final Iterable<Type> types = const [AgentTriggerers, _$AgentTriggerers];

  @override
  final String wireName = r'AgentTriggerers';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentTriggerers object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'principal_ids';
    yield serializers.serialize(
      object.principalIds,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    yield r'permission_version';
    yield serializers.serialize(
      object.permissionVersion,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentTriggerers object, {
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
    required AgentTriggerersBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'principal_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.principalIds.replace(valueDes);
          break;
        case r'permission_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.permissionVersion = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentTriggerers deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentTriggerersBuilder();
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

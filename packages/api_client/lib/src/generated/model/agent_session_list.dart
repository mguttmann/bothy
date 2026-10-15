//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/agent_session.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_session_list.g.dart';

/// AgentSessionList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class AgentSessionList
    implements Built<AgentSessionList, AgentSessionListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<AgentSession> get items;

  AgentSessionList._();

  factory AgentSessionList([void updates(AgentSessionListBuilder b)]) =
      _$AgentSessionList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentSessionListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentSessionList> get serializer =>
      _$AgentSessionListSerializer();
}

class _$AgentSessionListSerializer
    implements PrimitiveSerializer<AgentSessionList> {
  @override
  final Iterable<Type> types = const [AgentSessionList, _$AgentSessionList];

  @override
  final String wireName = r'AgentSessionList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentSessionList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(AgentSession)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentSessionList object, {
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
    required AgentSessionListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AgentSession)]),
          ) as BuiltList<AgentSession>;
          result.items.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentSessionList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentSessionListBuilder();
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

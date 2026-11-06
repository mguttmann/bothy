//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/agent_definition_version.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_definition_version_page.g.dart';

/// One page of versions, newest first. `next_cursor` is absent on the last page.
///
/// Properties:
/// * [items]
/// * [nextCursor] - Opaque pagination cursor. Clients must not parse or build cursors.
@BuiltValue()
abstract class AgentDefinitionVersionPage
    implements
        Built<AgentDefinitionVersionPage, AgentDefinitionVersionPageBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<AgentDefinitionVersion> get items;

  /// Opaque pagination cursor. Clients must not parse or build cursors.
  @BuiltValueField(wireName: r'next_cursor')
  String? get nextCursor;

  AgentDefinitionVersionPage._();

  factory AgentDefinitionVersionPage([
    void updates(AgentDefinitionVersionPageBuilder b),
  ]) = _$AgentDefinitionVersionPage;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentDefinitionVersionPageBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentDefinitionVersionPage> get serializer =>
      _$AgentDefinitionVersionPageSerializer();
}

class _$AgentDefinitionVersionPageSerializer
    implements PrimitiveSerializer<AgentDefinitionVersionPage> {
  @override
  final Iterable<Type> types = const [
    AgentDefinitionVersionPage,
    _$AgentDefinitionVersionPage,
  ];

  @override
  final String wireName = r'AgentDefinitionVersionPage';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentDefinitionVersionPage object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [
        FullType(AgentDefinitionVersion),
      ]),
    );
    if (object.nextCursor != null) {
      yield r'next_cursor';
      yield serializers.serialize(
        object.nextCursor,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinitionVersionPage object, {
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
    required AgentDefinitionVersionPageBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(AgentDefinitionVersion),
            ]),
          ) as BuiltList<AgentDefinitionVersion>;
          result.items.replace(valueDes);
          break;
        case r'next_cursor':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.nextCursor = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentDefinitionVersionPage deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentDefinitionVersionPageBuilder();
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

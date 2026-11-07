//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/agent_template.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_template_list.g.dart';

/// The shipped templates by `slug`, then the private templates by `slug`.
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class AgentTemplateList
    implements Built<AgentTemplateList, AgentTemplateListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<AgentTemplate> get items;

  AgentTemplateList._();

  factory AgentTemplateList([void updates(AgentTemplateListBuilder b)]) =
      _$AgentTemplateList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentTemplateListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentTemplateList> get serializer =>
      _$AgentTemplateListSerializer();
}

class _$AgentTemplateListSerializer
    implements PrimitiveSerializer<AgentTemplateList> {
  @override
  final Iterable<Type> types = const [AgentTemplateList, _$AgentTemplateList];

  @override
  final String wireName = r'AgentTemplateList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentTemplateList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(AgentTemplate)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentTemplateList object, {
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
    required AgentTemplateListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AgentTemplate)]),
          ) as BuiltList<AgentTemplate>;
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
  AgentTemplateList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentTemplateListBuilder();
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

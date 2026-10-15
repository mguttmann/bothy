//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/template.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'template_list.g.dart';

/// TemplateList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class TemplateList
    implements Built<TemplateList, TemplateListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<Template> get items;

  TemplateList._();

  factory TemplateList([void updates(TemplateListBuilder b)]) = _$TemplateList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TemplateListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TemplateList> get serializer => _$TemplateListSerializer();
}

class _$TemplateListSerializer implements PrimitiveSerializer<TemplateList> {
  @override
  final Iterable<Type> types = const [TemplateList, _$TemplateList];

  @override
  final String wireName = r'TemplateList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TemplateList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(Template)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    TemplateList object, {
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
    required TemplateListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Template)]),
          ) as BuiltList<Template>;
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
  TemplateList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TemplateListBuilder();
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

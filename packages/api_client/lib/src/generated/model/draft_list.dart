//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/draft.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'draft_list.g.dart';

/// DraftList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class DraftList implements Built<DraftList, DraftListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<Draft> get items;

  DraftList._();

  factory DraftList([void updates(DraftListBuilder b)]) = _$DraftList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DraftListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<DraftList> get serializer => _$DraftListSerializer();
}

class _$DraftListSerializer implements PrimitiveSerializer<DraftList> {
  @override
  final Iterable<Type> types = const [DraftList, _$DraftList];

  @override
  final String wireName = r'DraftList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    DraftList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(Draft)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    DraftList object, {
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
    required DraftListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Draft)]),
          ) as BuiltList<Draft>;
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
  DraftList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DraftListBuilder();
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

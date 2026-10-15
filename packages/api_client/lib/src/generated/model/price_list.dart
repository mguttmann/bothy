//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/price.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'price_list.g.dart';

/// PriceList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class PriceList implements Built<PriceList, PriceListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<Price> get items;

  PriceList._();

  factory PriceList([void updates(PriceListBuilder b)]) = _$PriceList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PriceListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PriceList> get serializer => _$PriceListSerializer();
}

class _$PriceListSerializer implements PrimitiveSerializer<PriceList> {
  @override
  final Iterable<Type> types = const [PriceList, _$PriceList];

  @override
  final String wireName = r'PriceList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PriceList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(Price)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    PriceList object, {
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
    required PriceListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Price)]),
          ) as BuiltList<Price>;
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
  PriceList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PriceListBuilder();
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

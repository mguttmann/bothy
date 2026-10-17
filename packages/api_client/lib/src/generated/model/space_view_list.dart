//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/space_view.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'space_view_list.g.dart';

/// SpaceViewList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class SpaceViewList
    implements Built<SpaceViewList, SpaceViewListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<SpaceView> get items;

  SpaceViewList._();

  factory SpaceViewList([void updates(SpaceViewListBuilder b)]) =
      _$SpaceViewList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SpaceViewListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SpaceViewList> get serializer =>
      _$SpaceViewListSerializer();
}

class _$SpaceViewListSerializer implements PrimitiveSerializer<SpaceViewList> {
  @override
  final Iterable<Type> types = const [SpaceViewList, _$SpaceViewList];

  @override
  final String wireName = r'SpaceViewList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SpaceViewList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(SpaceView)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SpaceViewList object, {
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
    required SpaceViewListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(SpaceView)]),
          ) as BuiltList<SpaceView>;
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
  SpaceViewList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SpaceViewListBuilder();
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

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/channel_view.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel_view_list.g.dart';

/// ChannelViewList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class ChannelViewList
    implements Built<ChannelViewList, ChannelViewListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<ChannelView> get items;

  ChannelViewList._();

  factory ChannelViewList([void updates(ChannelViewListBuilder b)]) =
      _$ChannelViewList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ChannelViewListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ChannelViewList> get serializer =>
      _$ChannelViewListSerializer();
}

class _$ChannelViewListSerializer
    implements PrimitiveSerializer<ChannelViewList> {
  @override
  final Iterable<Type> types = const [ChannelViewList, _$ChannelViewList];

  @override
  final String wireName = r'ChannelViewList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ChannelViewList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(ChannelView)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ChannelViewList object, {
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
    required ChannelViewListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(ChannelView)]),
          ) as BuiltList<ChannelView>;
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
  ChannelViewList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ChannelViewListBuilder();
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

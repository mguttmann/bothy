//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/pin.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'pin_list.g.dart';

/// PinList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class PinList implements Built<PinList, PinListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<Pin> get items;

  PinList._();

  factory PinList([void updates(PinListBuilder b)]) = _$PinList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PinListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PinList> get serializer => _$PinListSerializer();
}

class _$PinListSerializer implements PrimitiveSerializer<PinList> {
  @override
  final Iterable<Type> types = const [PinList, _$PinList];

  @override
  final String wireName = r'PinList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PinList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(Pin)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    PinList object, {
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
    required PinListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Pin)]),
          ) as BuiltList<Pin>;
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
  PinList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PinListBuilder();
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

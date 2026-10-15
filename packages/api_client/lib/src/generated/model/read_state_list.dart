//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/read_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'read_state_list.g.dart';

/// ReadStateList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class ReadStateList
    implements Built<ReadStateList, ReadStateListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<ReadState> get items;

  ReadStateList._();

  factory ReadStateList([void updates(ReadStateListBuilder b)]) =
      _$ReadStateList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ReadStateListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ReadStateList> get serializer =>
      _$ReadStateListSerializer();
}

class _$ReadStateListSerializer implements PrimitiveSerializer<ReadStateList> {
  @override
  final Iterable<Type> types = const [ReadStateList, _$ReadStateList];

  @override
  final String wireName = r'ReadStateList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ReadStateList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(ReadState)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ReadStateList object, {
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
    required ReadStateListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(ReadState)]),
          ) as BuiltList<ReadState>;
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
  ReadStateList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ReadStateListBuilder();
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

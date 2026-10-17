//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/sidebar_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'sidebar_state_list.g.dart';

/// SidebarStateList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class SidebarStateList
    implements Built<SidebarStateList, SidebarStateListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<SidebarState> get items;

  SidebarStateList._();

  factory SidebarStateList([void updates(SidebarStateListBuilder b)]) =
      _$SidebarStateList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SidebarStateListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SidebarStateList> get serializer =>
      _$SidebarStateListSerializer();
}

class _$SidebarStateListSerializer
    implements PrimitiveSerializer<SidebarStateList> {
  @override
  final Iterable<Type> types = const [SidebarStateList, _$SidebarStateList];

  @override
  final String wireName = r'SidebarStateList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SidebarStateList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(SidebarState)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SidebarStateList object, {
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
    required SidebarStateListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(SidebarState)]),
          ) as BuiltList<SidebarState>;
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
  SidebarStateList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SidebarStateListBuilder();
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

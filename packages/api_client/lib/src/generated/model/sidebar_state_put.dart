//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'sidebar_state_put.g.dart';

/// SidebarStatePut
///
/// Properties:
/// * [collapsed]
/// * [sortKey]
/// * [useGlobalOrder] - True removes the personal order. Replaces sending null.
/// * [channelOrder] - New personal order of the channels and forums of the space, unique IDs; absent keeps the stored order, an empty list returns to the default order. At most `sidebar.order_max_items`.
@BuiltValue()
abstract class SidebarStatePut
    implements Built<SidebarStatePut, SidebarStatePutBuilder> {
  @BuiltValueField(wireName: r'collapsed')
  bool get collapsed;

  @BuiltValueField(wireName: r'sort_key')
  String? get sortKey;

  /// True removes the personal order. Replaces sending null.
  @BuiltValueField(wireName: r'use_global_order')
  bool? get useGlobalOrder;

  /// New personal order of the channels and forums of the space, unique IDs; absent keeps the stored order, an empty list returns to the default order. At most `sidebar.order_max_items`.
  @BuiltValueField(wireName: r'channel_order')
  BuiltList<String>? get channelOrder;

  SidebarStatePut._();

  factory SidebarStatePut([void updates(SidebarStatePutBuilder b)]) =
      _$SidebarStatePut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SidebarStatePutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SidebarStatePut> get serializer =>
      _$SidebarStatePutSerializer();
}

class _$SidebarStatePutSerializer
    implements PrimitiveSerializer<SidebarStatePut> {
  @override
  final Iterable<Type> types = const [SidebarStatePut, _$SidebarStatePut];

  @override
  final String wireName = r'SidebarStatePut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SidebarStatePut object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'collapsed';
    yield serializers.serialize(
      object.collapsed,
      specifiedType: const FullType(bool),
    );
    if (object.sortKey != null) {
      yield r'sort_key';
      yield serializers.serialize(
        object.sortKey,
        specifiedType: const FullType(String),
      );
    }
    if (object.useGlobalOrder != null) {
      yield r'use_global_order';
      yield serializers.serialize(
        object.useGlobalOrder,
        specifiedType: const FullType(bool),
      );
    }
    if (object.channelOrder != null) {
      yield r'channel_order';
      yield serializers.serialize(
        object.channelOrder,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    SidebarStatePut object, {
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
    required SidebarStatePutBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'collapsed':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.collapsed = valueDes;
          break;
        case r'sort_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.sortKey = valueDes;
          break;
        case r'use_global_order':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.useGlobalOrder = valueDes;
          break;
        case r'channel_order':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.channelOrder.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SidebarStatePut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SidebarStatePutBuilder();
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

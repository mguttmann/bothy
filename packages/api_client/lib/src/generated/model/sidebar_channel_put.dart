//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'sidebar_channel_put.g.dart';

/// SidebarChannelPut
///
/// Properties:
/// * [threadOrder] - New personal order of the posts of the channel, unique IDs; absent keeps the stored order, an empty list returns to the default order. At most `sidebar.order_max_items`.
/// * [hidden] - true archives the channel in the caller's own sidebar (sets `hidden_at`, kept when it is already set), false brings it back (clears `hidden_at`); absent keeps the stored state (01 6.22).
@BuiltValue()
abstract class SidebarChannelPut
    implements Built<SidebarChannelPut, SidebarChannelPutBuilder> {
  /// New personal order of the posts of the channel, unique IDs; absent keeps the stored order, an empty list returns to the default order. At most `sidebar.order_max_items`.
  @BuiltValueField(wireName: r'thread_order')
  BuiltList<String>? get threadOrder;

  /// true archives the channel in the caller's own sidebar (sets `hidden_at`, kept when it is already set), false brings it back (clears `hidden_at`); absent keeps the stored state (01 6.22).
  @BuiltValueField(wireName: r'hidden')
  bool? get hidden;

  SidebarChannelPut._();

  factory SidebarChannelPut([void updates(SidebarChannelPutBuilder b)]) =
      _$SidebarChannelPut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SidebarChannelPutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SidebarChannelPut> get serializer =>
      _$SidebarChannelPutSerializer();
}

class _$SidebarChannelPutSerializer
    implements PrimitiveSerializer<SidebarChannelPut> {
  @override
  final Iterable<Type> types = const [SidebarChannelPut, _$SidebarChannelPut];

  @override
  final String wireName = r'SidebarChannelPut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SidebarChannelPut object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.threadOrder != null) {
      yield r'thread_order';
      yield serializers.serialize(
        object.threadOrder,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.hidden != null) {
      yield r'hidden';
      yield serializers.serialize(
        object.hidden,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    SidebarChannelPut object, {
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
    required SidebarChannelPutBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'thread_order':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.threadOrder.replace(valueDes);
          break;
        case r'hidden':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.hidden = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SidebarChannelPut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SidebarChannelPutBuilder();
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

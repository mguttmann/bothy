//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'sidebar_channel_state.g.dart';

/// SidebarChannelState
///
/// Properties:
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadOrder] - Personal order of the open posts of the channel; empty uses the default order. At most `sidebar.order_max_items`.
/// * [version]
/// * [updatedByDeviceId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class SidebarChannelState
    implements Built<SidebarChannelState, SidebarChannelStateBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  /// Personal order of the open posts of the channel; empty uses the default order. At most `sidebar.order_max_items`.
  @BuiltValueField(wireName: r'thread_order')
  BuiltList<String> get threadOrder;

  @BuiltValueField(wireName: r'version')
  int get version;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'updated_by_device_id')
  String? get updatedByDeviceId;

  SidebarChannelState._();

  factory SidebarChannelState([void updates(SidebarChannelStateBuilder b)]) =
      _$SidebarChannelState;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SidebarChannelStateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SidebarChannelState> get serializer =>
      _$SidebarChannelStateSerializer();
}

class _$SidebarChannelStateSerializer
    implements PrimitiveSerializer<SidebarChannelState> {
  @override
  final Iterable<Type> types = const [
    SidebarChannelState,
    _$SidebarChannelState,
  ];

  @override
  final String wireName = r'SidebarChannelState';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SidebarChannelState object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    yield r'thread_order';
    yield serializers.serialize(
      object.threadOrder,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
    if (object.updatedByDeviceId != null) {
      yield r'updated_by_device_id';
      yield serializers.serialize(
        object.updatedByDeviceId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    SidebarChannelState object, {
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
    required SidebarChannelStateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'thread_order':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.threadOrder.replace(valueDes);
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        case r'updated_by_device_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.updatedByDeviceId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SidebarChannelState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SidebarChannelStateBuilder();
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

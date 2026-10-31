//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/sidebar_channel_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'sidebar_channel_payload.g.dart';

/// SidebarChannelPayload
///
/// Properties:
/// * [state]
@BuiltValue()
abstract class SidebarChannelPayload
    implements Built<SidebarChannelPayload, SidebarChannelPayloadBuilder> {
  @BuiltValueField(wireName: r'state')
  SidebarChannelState get state;

  SidebarChannelPayload._();

  factory SidebarChannelPayload([
    void updates(SidebarChannelPayloadBuilder b),
  ]) = _$SidebarChannelPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SidebarChannelPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SidebarChannelPayload> get serializer =>
      _$SidebarChannelPayloadSerializer();
}

class _$SidebarChannelPayloadSerializer
    implements PrimitiveSerializer<SidebarChannelPayload> {
  @override
  final Iterable<Type> types = const [
    SidebarChannelPayload,
    _$SidebarChannelPayload,
  ];

  @override
  final String wireName = r'SidebarChannelPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SidebarChannelPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(SidebarChannelState),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SidebarChannelPayload object, {
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
    required SidebarChannelPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(SidebarChannelState),
          ) as SidebarChannelState;
          result.state.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SidebarChannelPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SidebarChannelPayloadBuilder();
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

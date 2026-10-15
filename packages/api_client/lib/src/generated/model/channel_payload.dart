//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/channel.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel_payload.g.dart';

/// ChannelPayload
///
/// Properties:
/// * [channel]
@BuiltValue()
abstract class ChannelPayload
    implements Built<ChannelPayload, ChannelPayloadBuilder> {
  @BuiltValueField(wireName: r'channel')
  Channel get channel;

  ChannelPayload._();

  factory ChannelPayload([void updates(ChannelPayloadBuilder b)]) =
      _$ChannelPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ChannelPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ChannelPayload> get serializer =>
      _$ChannelPayloadSerializer();
}

class _$ChannelPayloadSerializer
    implements PrimitiveSerializer<ChannelPayload> {
  @override
  final Iterable<Type> types = const [ChannelPayload, _$ChannelPayload];

  @override
  final String wireName = r'ChannelPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ChannelPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channel';
    yield serializers.serialize(
      object.channel,
      specifiedType: const FullType(Channel),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ChannelPayload object, {
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
    required ChannelPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'channel':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Channel),
          ) as Channel;
          result.channel.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ChannelPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ChannelPayloadBuilder();
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

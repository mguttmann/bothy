//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/capability.dart';
import 'package:api_client/src/generated/model/level.dart';
import 'package:api_client/src/generated/model/channel.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel_view.g.dart';

/// ChannelView
///
/// Properties:
/// * [channel]
/// * [level]
/// * [capabilities]
@BuiltValue()
abstract class ChannelView implements Built<ChannelView, ChannelViewBuilder> {
  @BuiltValueField(wireName: r'channel')
  Channel get channel;

  @BuiltValueField(wireName: r'level')
  Level get level;
  // enum levelEnum {  read,  write,  trigger,  };

  @BuiltValueField(wireName: r'capabilities')
  BuiltList<Capability> get capabilities;

  ChannelView._();

  factory ChannelView([void updates(ChannelViewBuilder b)]) = _$ChannelView;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ChannelViewBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ChannelView> get serializer => _$ChannelViewSerializer();
}

class _$ChannelViewSerializer implements PrimitiveSerializer<ChannelView> {
  @override
  final Iterable<Type> types = const [ChannelView, _$ChannelView];

  @override
  final String wireName = r'ChannelView';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ChannelView object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channel';
    yield serializers.serialize(
      object.channel,
      specifiedType: const FullType(Channel),
    );
    yield r'level';
    yield serializers.serialize(
      object.level,
      specifiedType: const FullType(Level),
    );
    yield r'capabilities';
    yield serializers.serialize(
      object.capabilities,
      specifiedType: const FullType(BuiltList, [FullType(Capability)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ChannelView object, {
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
    required ChannelViewBuilder result,
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
        case r'level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Level),
          ) as Level;
          result.level = valueDes;
          break;
        case r'capabilities':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Capability)]),
          ) as BuiltList<Capability>;
          result.capabilities.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ChannelView deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ChannelViewBuilder();
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

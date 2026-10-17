//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel_settings_changed_payload.g.dart';

/// ChannelSettingsChangedPayload
///
/// Properties:
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [settingsVersion]
@BuiltValue()
abstract class ChannelSettingsChangedPayload
    implements
        Built<
          ChannelSettingsChangedPayload,
          ChannelSettingsChangedPayloadBuilder
        > {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  @BuiltValueField(wireName: r'settings_version')
  int get settingsVersion;

  ChannelSettingsChangedPayload._();

  factory ChannelSettingsChangedPayload([
    void updates(ChannelSettingsChangedPayloadBuilder b),
  ]) = _$ChannelSettingsChangedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ChannelSettingsChangedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ChannelSettingsChangedPayload> get serializer =>
      _$ChannelSettingsChangedPayloadSerializer();
}

class _$ChannelSettingsChangedPayloadSerializer
    implements PrimitiveSerializer<ChannelSettingsChangedPayload> {
  @override
  final Iterable<Type> types = const [
    ChannelSettingsChangedPayload,
    _$ChannelSettingsChangedPayload,
  ];

  @override
  final String wireName = r'ChannelSettingsChangedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ChannelSettingsChangedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    yield r'settings_version';
    yield serializers.serialize(
      object.settingsVersion,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ChannelSettingsChangedPayload object, {
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
    required ChannelSettingsChangedPayloadBuilder result,
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
        case r'settings_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.settingsVersion = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ChannelSettingsChangedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ChannelSettingsChangedPayloadBuilder();
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

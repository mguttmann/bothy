//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/forum_sort.dart';
import 'package:api_client/src/generated/model/notification_level.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel_update.g.dart';

/// ChannelUpdate
///
/// Properties:
/// * [name]
/// * [topic] - At most `channel.topic_max_chars` characters, no control or bidi characters.
/// * [forumDefaultSort]
/// * [defaultNotificationLevel]
/// * [clearDefaultNotificationLevel] - Removes the channel default; there is no JSON null (R-API-03).
@BuiltValue()
abstract class ChannelUpdate
    implements Built<ChannelUpdate, ChannelUpdateBuilder> {
  @BuiltValueField(wireName: r'name')
  String? get name;

  /// At most `channel.topic_max_chars` characters, no control or bidi characters.
  @BuiltValueField(wireName: r'topic')
  String? get topic;

  @BuiltValueField(wireName: r'forum_default_sort')
  ForumSort? get forumDefaultSort;
  // enum forumDefaultSortEnum {  last_activity,  created,  };

  @BuiltValueField(wireName: r'default_notification_level')
  NotificationLevel? get defaultNotificationLevel;
  // enum defaultNotificationLevelEnum {  all,  mentions,  mute,  };

  /// Removes the channel default; there is no JSON null (R-API-03).
  @BuiltValueField(wireName: r'clear_default_notification_level')
  bool? get clearDefaultNotificationLevel;

  ChannelUpdate._();

  factory ChannelUpdate([void updates(ChannelUpdateBuilder b)]) =
      _$ChannelUpdate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ChannelUpdateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ChannelUpdate> get serializer =>
      _$ChannelUpdateSerializer();
}

class _$ChannelUpdateSerializer implements PrimitiveSerializer<ChannelUpdate> {
  @override
  final Iterable<Type> types = const [ChannelUpdate, _$ChannelUpdate];

  @override
  final String wireName = r'ChannelUpdate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ChannelUpdate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.name != null) {
      yield r'name';
      yield serializers.serialize(
        object.name,
        specifiedType: const FullType(String),
      );
    }
    if (object.topic != null) {
      yield r'topic';
      yield serializers.serialize(
        object.topic,
        specifiedType: const FullType(String),
      );
    }
    if (object.forumDefaultSort != null) {
      yield r'forum_default_sort';
      yield serializers.serialize(
        object.forumDefaultSort,
        specifiedType: const FullType(ForumSort),
      );
    }
    if (object.defaultNotificationLevel != null) {
      yield r'default_notification_level';
      yield serializers.serialize(
        object.defaultNotificationLevel,
        specifiedType: const FullType(NotificationLevel),
      );
    }
    if (object.clearDefaultNotificationLevel != null) {
      yield r'clear_default_notification_level';
      yield serializers.serialize(
        object.clearDefaultNotificationLevel,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ChannelUpdate object, {
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
    required ChannelUpdateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.name = valueDes;
          break;
        case r'topic':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.topic = valueDes;
          break;
        case r'forum_default_sort':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ForumSort),
          ) as ForumSort?;
          if (valueDes == null) continue;
          result.forumDefaultSort = valueDes;
          break;
        case r'default_notification_level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(NotificationLevel),
          ) as NotificationLevel?;
          if (valueDes == null) continue;
          result.defaultNotificationLevel = valueDes;
          break;
        case r'clear_default_notification_level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.clearDefaultNotificationLevel = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ChannelUpdate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ChannelUpdateBuilder();
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

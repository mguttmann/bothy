//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/forum_sort.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel_create.g.dart';

/// ChannelCreate
///
/// Properties:
/// * [kind]
/// * [name]
/// * [topic] - At most `channel.topic_max_chars` characters, no control or bidi characters.
/// * [forumDefaultSort]
/// * [afterChannelId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class ChannelCreate
    implements Built<ChannelCreate, ChannelCreateBuilder> {
  @BuiltValueField(wireName: r'kind')
  ChannelCreateKindEnum get kind;
  // enum kindEnum {  text,  forum,  };

  @BuiltValueField(wireName: r'name')
  String get name;

  /// At most `channel.topic_max_chars` characters, no control or bidi characters.
  @BuiltValueField(wireName: r'topic')
  String? get topic;

  @BuiltValueField(wireName: r'forum_default_sort')
  ForumSort? get forumDefaultSort;
  // enum forumDefaultSortEnum {  last_activity,  created,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'after_channel_id')
  String? get afterChannelId;

  ChannelCreate._();

  factory ChannelCreate([void updates(ChannelCreateBuilder b)]) =
      _$ChannelCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ChannelCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ChannelCreate> get serializer =>
      _$ChannelCreateSerializer();
}

class _$ChannelCreateSerializer implements PrimitiveSerializer<ChannelCreate> {
  @override
  final Iterable<Type> types = const [ChannelCreate, _$ChannelCreate];

  @override
  final String wireName = r'ChannelCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ChannelCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(ChannelCreateKindEnum),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
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
    if (object.afterChannelId != null) {
      yield r'after_channel_id';
      yield serializers.serialize(
        object.afterChannelId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ChannelCreate object, {
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
    required ChannelCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ChannelCreateKindEnum),
          ) as ChannelCreateKindEnum;
          result.kind = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
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
        case r'after_channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.afterChannelId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ChannelCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ChannelCreateBuilder();
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

class ChannelCreateKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'text')
  static const ChannelCreateKindEnum text = _$channelCreateKindEnum_text;
  @BuiltValueEnumConst(wireName: r'forum')
  static const ChannelCreateKindEnum forum = _$channelCreateKindEnum_forum;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ChannelCreateKindEnum unknownDefaultOpenApi =
      _$channelCreateKindEnum_unknownDefaultOpenApi;

  static Serializer<ChannelCreateKindEnum> get serializer =>
      _$channelCreateKindEnumSerializer;

  const ChannelCreateKindEnum._(String name) : super(name);

  static BuiltSet<ChannelCreateKindEnum> get values =>
      _$channelCreateKindEnumValues;
  static ChannelCreateKindEnum valueOf(String name) =>
      _$channelCreateKindEnumValueOf(name);
}

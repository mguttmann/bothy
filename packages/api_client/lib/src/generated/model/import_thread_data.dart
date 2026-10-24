//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/import_earlier_session.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_thread_data.g.dart';

/// Data of a `thread` item (22, 5.1.1). The title is `name` cut to `thread.title_max_chars`, `title_source` is `auto` for a placeholder name and `manual` otherwise; the operator is the planned operator of the channel in the mapping; `parent_message_id` is the starter message only when its item is posted in the main thread of the same channel; the tags are the posted `forum_tag` items of the same channel. A posted thread only follows the mirror rules of 22 12.2 step 3.
///
/// Properties:
/// * [channelOriginKey]
/// * [name]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [lastActivityAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [closed] - The archive rule of the tool (22 5.1.1, R-IM-11).
/// * [closedAt] - Required exactly when `closed` is true.
/// * [starterMessageOriginKey]
/// * [tagOriginKeys]
/// * [earlierSession]
@BuiltValue()
abstract class ImportThreadData
    implements Built<ImportThreadData, ImportThreadDataBuilder> {
  @BuiltValueField(wireName: r'channel_origin_key')
  String get channelOriginKey;

  @BuiltValueField(wireName: r'name')
  String get name;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'last_activity_at')
  DateTime get lastActivityAt;

  /// The archive rule of the tool (22 5.1.1, R-IM-11).
  @BuiltValueField(wireName: r'closed')
  bool get closed;

  /// Required exactly when `closed` is true.
  @BuiltValueField(wireName: r'closed_at')
  DateTime? get closedAt;

  @BuiltValueField(wireName: r'starter_message_origin_key')
  String? get starterMessageOriginKey;

  @BuiltValueField(wireName: r'tag_origin_keys')
  BuiltList<String>? get tagOriginKeys;

  @BuiltValueField(wireName: r'earlier_session')
  ImportEarlierSession? get earlierSession;

  ImportThreadData._();

  factory ImportThreadData([void updates(ImportThreadDataBuilder b)]) =
      _$ImportThreadData;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportThreadDataBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportThreadData> get serializer =>
      _$ImportThreadDataSerializer();
}

class _$ImportThreadDataSerializer
    implements PrimitiveSerializer<ImportThreadData> {
  @override
  final Iterable<Type> types = const [ImportThreadData, _$ImportThreadData];

  @override
  final String wireName = r'ImportThreadData';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportThreadData object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channel_origin_key';
    yield serializers.serialize(
      object.channelOriginKey,
      specifiedType: const FullType(String),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'last_activity_at';
    yield serializers.serialize(
      object.lastActivityAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'closed';
    yield serializers.serialize(
      object.closed,
      specifiedType: const FullType(bool),
    );
    if (object.closedAt != null) {
      yield r'closed_at';
      yield serializers.serialize(
        object.closedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.starterMessageOriginKey != null) {
      yield r'starter_message_origin_key';
      yield serializers.serialize(
        object.starterMessageOriginKey,
        specifiedType: const FullType(String),
      );
    }
    if (object.tagOriginKeys != null) {
      yield r'tag_origin_keys';
      yield serializers.serialize(
        object.tagOriginKeys,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.earlierSession != null) {
      yield r'earlier_session';
      yield serializers.serialize(
        object.earlierSession,
        specifiedType: const FullType(ImportEarlierSession),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportThreadData object, {
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
    required ImportThreadDataBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'channel_origin_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelOriginKey = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'last_activity_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.lastActivityAt = valueDes;
          break;
        case r'closed':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.closed = valueDes;
          break;
        case r'closed_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.closedAt = valueDes;
          break;
        case r'starter_message_origin_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.starterMessageOriginKey = valueDes;
          break;
        case r'tag_origin_keys':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.tagOriginKeys.replace(valueDes);
          break;
        case r'earlier_session':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ImportEarlierSession),
          ) as ImportEarlierSession?;
          if (valueDes == null) continue;
          result.earlierSession.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportThreadData deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportThreadDataBuilder();
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

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/forum_sort.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/channel_operator.dart';
import 'package:api_client/src/generated/model/channel_kind.dart';
import 'package:api_client/src/generated/model/notification_level.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel.g.dart';

/// Channel
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [spaceId] - Absent for `dm`.
/// * [kind]
/// * [name]
/// * [topic]
/// * [sortKey]
/// * [operator_]
/// * [forumDefaultSort]
/// * [defaultNotificationLevel] - Default for people without an own setting (13, 3.1); absent means none.
/// * [readOnly] - Write-protected for every principal including owners; only a running import writes (05, rule 1.8).
/// * [projectBound] - True exactly when a project binding exists for the forum (03 `project_bindings`, R-DM-54); derived, never stored. Creating or deleting a binding raises `version` and writes `channel.updated`; changing a binding and a readiness report do not.
/// * [mainThreadId] - Thread with `kind = main` of text and DM channels.
/// * [dmParticipantIds]
/// * [archivedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [deletedAt] - Set while the item is deleted, pending or final (01 6.18); clients remove it and its local content, and a later version without it brings it back.
/// * [version]
/// * [settingsVersion]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class Channel implements Built<Channel, ChannelBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// Absent for `dm`.
  @BuiltValueField(wireName: r'space_id')
  String? get spaceId;

  @BuiltValueField(wireName: r'kind')
  ChannelKind get kind;
  // enum kindEnum {  text,  forum,  dm,  };

  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'topic')
  String get topic;

  @BuiltValueField(wireName: r'sort_key')
  String get sortKey;

  @BuiltValueField(wireName: r'operator')
  ChannelOperator get operator_;

  @BuiltValueField(wireName: r'forum_default_sort')
  ForumSort? get forumDefaultSort;
  // enum forumDefaultSortEnum {  last_activity,  created,  };

  /// Default for people without an own setting (13, 3.1); absent means none.
  @BuiltValueField(wireName: r'default_notification_level')
  NotificationLevel? get defaultNotificationLevel;
  // enum defaultNotificationLevelEnum {  all,  mentions,  mute,  };

  /// Write-protected for every principal including owners; only a running import writes (05, rule 1.8).
  @BuiltValueField(wireName: r'read_only')
  bool get readOnly;

  /// True exactly when a project binding exists for the forum (03 `project_bindings`, R-DM-54); derived, never stored. Creating or deleting a binding raises `version` and writes `channel.updated`; changing a binding and a readiness report do not.
  @BuiltValueField(wireName: r'project_bound')
  bool get projectBound;

  /// Thread with `kind = main` of text and DM channels.
  @BuiltValueField(wireName: r'main_thread_id')
  String? get mainThreadId;

  @BuiltValueField(wireName: r'dm_participant_ids')
  BuiltList<String>? get dmParticipantIds;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'archived_at')
  DateTime? get archivedAt;

  /// Set while the item is deleted, pending or final (01 6.18); clients remove it and its local content, and a later version without it brings it back.
  @BuiltValueField(wireName: r'deleted_at')
  DateTime? get deletedAt;

  @BuiltValueField(wireName: r'version')
  int get version;

  @BuiltValueField(wireName: r'settings_version')
  int get settingsVersion;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  Channel._();

  factory Channel([void updates(ChannelBuilder b)]) = _$Channel;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ChannelBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Channel> get serializer => _$ChannelSerializer();
}

class _$ChannelSerializer implements PrimitiveSerializer<Channel> {
  @override
  final Iterable<Type> types = const [Channel, _$Channel];

  @override
  final String wireName = r'Channel';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Channel object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    if (object.spaceId != null) {
      yield r'space_id';
      yield serializers.serialize(
        object.spaceId,
        specifiedType: const FullType(String),
      );
    }
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(ChannelKind),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'topic';
    yield serializers.serialize(
      object.topic,
      specifiedType: const FullType(String),
    );
    yield r'sort_key';
    yield serializers.serialize(
      object.sortKey,
      specifiedType: const FullType(String),
    );
    yield r'operator';
    yield serializers.serialize(
      object.operator_,
      specifiedType: const FullType(ChannelOperator),
    );
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
    yield r'read_only';
    yield serializers.serialize(
      object.readOnly,
      specifiedType: const FullType(bool),
    );
    yield r'project_bound';
    yield serializers.serialize(
      object.projectBound,
      specifiedType: const FullType(bool),
    );
    if (object.mainThreadId != null) {
      yield r'main_thread_id';
      yield serializers.serialize(
        object.mainThreadId,
        specifiedType: const FullType(String),
      );
    }
    if (object.dmParticipantIds != null) {
      yield r'dm_participant_ids';
      yield serializers.serialize(
        object.dmParticipantIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.archivedAt != null) {
      yield r'archived_at';
      yield serializers.serialize(
        object.archivedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.deletedAt != null) {
      yield r'deleted_at';
      yield serializers.serialize(
        object.deletedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
    yield r'settings_version';
    yield serializers.serialize(
      object.settingsVersion,
      specifiedType: const FullType(int),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Channel object, {
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
    required ChannelBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'space_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.spaceId = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ChannelKind),
          ) as ChannelKind;
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
            specifiedType: const FullType(String),
          ) as String;
          result.topic = valueDes;
          break;
        case r'sort_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sortKey = valueDes;
          break;
        case r'operator':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ChannelOperator),
          ) as ChannelOperator;
          result.operator_.replace(valueDes);
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
        case r'read_only':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.readOnly = valueDes;
          break;
        case r'project_bound':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.projectBound = valueDes;
          break;
        case r'main_thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.mainThreadId = valueDes;
          break;
        case r'dm_participant_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.dmParticipantIds.replace(valueDes);
          break;
        case r'archived_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.archivedAt = valueDes;
          break;
        case r'deleted_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.deletedAt = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        case r'settings_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.settingsVersion = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Channel deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ChannelBuilder();
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

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:api_client/src/generated/model/thread_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'thread.g.dart';

/// Thread
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [kind]
/// * [parentMessageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [title]
/// * [titleSource]
/// * [state]
/// * [closedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [closedBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [operatorAgentId] - Fixed operator of a thread of kind `thread` (R-DM-12). Absent for the main thread (`kind = main`, R-DM-21) and for threads without operator; clients then use `Channel.operator.effective_agent_id`.
/// * [billingMode] - Current policy of `operator_agent_id`, for the header badge; absent exactly when `operator_agent_id` is absent. Changes arrive with `agent.updated`.
/// * [lastMessageSeq]
/// * [unreadFloorSeq] - Messages up to this `thread_seq` never count as unread (import); absent means 0.
/// * [lastActivityAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [messageCount]
/// * [createdBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [movedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [version]
/// * [tagIds]
@BuiltValue()
abstract class Thread implements Built<Thread, ThreadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  @BuiltValueField(wireName: r'kind')
  ThreadKindEnum get kind;
  // enum kindEnum {  main,  thread,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'parent_message_id')
  String? get parentMessageId;

  @BuiltValueField(wireName: r'title')
  String get title;

  @BuiltValueField(wireName: r'title_source')
  ThreadTitleSourceEnum get titleSource;
  // enum titleSourceEnum {  auto,  manual,  };

  @BuiltValueField(wireName: r'state')
  ThreadState get state;
  // enum stateEnum {  open,  closed,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'closed_at')
  DateTime? get closedAt;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'closed_by')
  String? get closedBy;

  /// Fixed operator of a thread of kind `thread` (R-DM-12). Absent for the main thread (`kind = main`, R-DM-21) and for threads without operator; clients then use `Channel.operator.effective_agent_id`.
  @BuiltValueField(wireName: r'operator_agent_id')
  String? get operatorAgentId;

  /// Current policy of `operator_agent_id`, for the header badge; absent exactly when `operator_agent_id` is absent. Changes arrive with `agent.updated`.
  @BuiltValueField(wireName: r'billing_mode')
  BillingMode? get billingMode;
  // enum billingModeEnum {  personal,  organization,  };

  @BuiltValueField(wireName: r'last_message_seq')
  int get lastMessageSeq;

  /// Messages up to this `thread_seq` never count as unread (import); absent means 0.
  @BuiltValueField(wireName: r'unread_floor_seq')
  int? get unreadFloorSeq;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'last_activity_at')
  DateTime get lastActivityAt;

  @BuiltValueField(wireName: r'message_count')
  int get messageCount;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'created_by')
  String get createdBy;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'moved_at')
  DateTime? get movedAt;

  @BuiltValueField(wireName: r'version')
  int get version;

  @BuiltValueField(wireName: r'tag_ids')
  BuiltList<String> get tagIds;

  Thread._();

  factory Thread([void updates(ThreadBuilder b)]) = _$Thread;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ThreadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Thread> get serializer => _$ThreadSerializer();
}

class _$ThreadSerializer implements PrimitiveSerializer<Thread> {
  @override
  final Iterable<Type> types = const [Thread, _$Thread];

  @override
  final String wireName = r'Thread';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Thread object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(ThreadKindEnum),
    );
    if (object.parentMessageId != null) {
      yield r'parent_message_id';
      yield serializers.serialize(
        object.parentMessageId,
        specifiedType: const FullType(String),
      );
    }
    yield r'title';
    yield serializers.serialize(
      object.title,
      specifiedType: const FullType(String),
    );
    yield r'title_source';
    yield serializers.serialize(
      object.titleSource,
      specifiedType: const FullType(ThreadTitleSourceEnum),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(ThreadState),
    );
    if (object.closedAt != null) {
      yield r'closed_at';
      yield serializers.serialize(
        object.closedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.closedBy != null) {
      yield r'closed_by';
      yield serializers.serialize(
        object.closedBy,
        specifiedType: const FullType(String),
      );
    }
    if (object.operatorAgentId != null) {
      yield r'operator_agent_id';
      yield serializers.serialize(
        object.operatorAgentId,
        specifiedType: const FullType(String),
      );
    }
    if (object.billingMode != null) {
      yield r'billing_mode';
      yield serializers.serialize(
        object.billingMode,
        specifiedType: const FullType(BillingMode),
      );
    }
    yield r'last_message_seq';
    yield serializers.serialize(
      object.lastMessageSeq,
      specifiedType: const FullType(int),
    );
    if (object.unreadFloorSeq != null) {
      yield r'unread_floor_seq';
      yield serializers.serialize(
        object.unreadFloorSeq,
        specifiedType: const FullType(int),
      );
    }
    yield r'last_activity_at';
    yield serializers.serialize(
      object.lastActivityAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'message_count';
    yield serializers.serialize(
      object.messageCount,
      specifiedType: const FullType(int),
    );
    yield r'created_by';
    yield serializers.serialize(
      object.createdBy,
      specifiedType: const FullType(String),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.movedAt != null) {
      yield r'moved_at';
      yield serializers.serialize(
        object.movedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
    yield r'tag_ids';
    yield serializers.serialize(
      object.tagIds,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Thread object, {
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
    required ThreadBuilder result,
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
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ThreadKindEnum),
          ) as ThreadKindEnum;
          result.kind = valueDes;
          break;
        case r'parent_message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.parentMessageId = valueDes;
          break;
        case r'title':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.title = valueDes;
          break;
        case r'title_source':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ThreadTitleSourceEnum),
          ) as ThreadTitleSourceEnum;
          result.titleSource = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ThreadState),
          ) as ThreadState;
          result.state = valueDes;
          break;
        case r'closed_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.closedAt = valueDes;
          break;
        case r'closed_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.closedBy = valueDes;
          break;
        case r'operator_agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.operatorAgentId = valueDes;
          break;
        case r'billing_mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BillingMode),
          ) as BillingMode?;
          if (valueDes == null) continue;
          result.billingMode = valueDes;
          break;
        case r'last_message_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.lastMessageSeq = valueDes;
          break;
        case r'unread_floor_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.unreadFloorSeq = valueDes;
          break;
        case r'last_activity_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.lastActivityAt = valueDes;
          break;
        case r'message_count':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.messageCount = valueDes;
          break;
        case r'created_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.createdBy = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'moved_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.movedAt = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        case r'tag_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.tagIds.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Thread deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ThreadBuilder();
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

class ThreadKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'main')
  static const ThreadKindEnum main = _$threadKindEnum_main;
  @BuiltValueEnumConst(wireName: r'thread')
  static const ThreadKindEnum thread = _$threadKindEnum_thread;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ThreadKindEnum unknownDefaultOpenApi =
      _$threadKindEnum_unknownDefaultOpenApi;

  static Serializer<ThreadKindEnum> get serializer =>
      _$threadKindEnumSerializer;

  const ThreadKindEnum._(String name) : super(name);

  static BuiltSet<ThreadKindEnum> get values => _$threadKindEnumValues;
  static ThreadKindEnum valueOf(String name) => _$threadKindEnumValueOf(name);
}

class ThreadTitleSourceEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'auto')
  static const ThreadTitleSourceEnum auto = _$threadTitleSourceEnum_auto;
  @BuiltValueEnumConst(wireName: r'manual')
  static const ThreadTitleSourceEnum manual = _$threadTitleSourceEnum_manual;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ThreadTitleSourceEnum unknownDefaultOpenApi =
      _$threadTitleSourceEnum_unknownDefaultOpenApi;

  static Serializer<ThreadTitleSourceEnum> get serializer =>
      _$threadTitleSourceEnumSerializer;

  const ThreadTitleSourceEnum._(String name) : super(name);

  static BuiltSet<ThreadTitleSourceEnum> get values =>
      _$threadTitleSourceEnumValues;
  static ThreadTitleSourceEnum valueOf(String name) =>
      _$threadTitleSourceEnumValueOf(name);
}

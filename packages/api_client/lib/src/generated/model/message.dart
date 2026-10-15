//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/message_kind.dart';
import 'package:api_client/src/generated/model/trigger_state.dart';
import 'package:api_client/src/generated/model/attachment.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/message_origin.dart';
import 'package:api_client/src/generated/model/reaction_summary.dart';
import 'package:api_client/src/generated/model/principal_ref.dart';
import 'package:api_client/src/generated/model/component.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'message.g.dart';

/// Message
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadSeq]
/// * [author]
/// * [clientMsgId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [origin]
/// * [kind]
/// * [systemCode] - Set exactly for `kind = system`; clients render the localized line from it and `author`. `content_md` is empty then.
/// * [contentMd] - Markdown. In lists longer contents are cut at `message.list_content_max_bytes` with `content_truncated`.
/// * [contentFormat] - Version of the Markdown dialect; `1` is CommonMark 0.31.2 without raw HTML, with mention tokens (03 `messages`).
/// * [contentTruncated] - Set when `content_md` was cut (events above `sync.event_payload_max_bytes`, lists above `message.list_content_max_bytes`); `getMessage` returns the full content.
/// * [replyToMessageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [triggerState]
/// * [triggerStateAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [triggerStateReason] - Reason when `trigger_state` is `failed` or `cancelled`, e.g. `billing_mode_personal_not_holder`, `run_active_other_agent`, `user_cancelled`.
/// * [silent]
/// * [attachments]
/// * [mentionIds] - Principals resolved from the mention tokens of `content_md`, sorted. Clients render a token as a mention only for these.
/// * [reactions]
/// * [myReactions] - Emoji the caller reacted with. Only in REST responses, never in events.
/// * [pinned]
/// * [components]
/// * [version]
/// * [editedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [deletedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class Message implements Built<Message, MessageBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  @BuiltValueField(wireName: r'thread_seq')
  int get threadSeq;

  @BuiltValueField(wireName: r'author')
  PrincipalRef get author;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'client_msg_id')
  String get clientMsgId;

  @BuiltValueField(wireName: r'origin')
  MessageOrigin get origin;
  // enum originEnum {  human,  agent,  app,  import,  system,  };

  @BuiltValueField(wireName: r'kind')
  MessageKind get kind;
  // enum kindEnum {  text,  voice,  run,  card,  system,  };

  /// Set exactly for `kind = system`; clients render the localized line from it and `author`. `content_md` is empty then.
  @BuiltValueField(wireName: r'system_code')
  MessageSystemCodeEnum? get systemCode;
  // enum systemCodeEnum {  thread.reopened,  };

  /// Markdown. In lists longer contents are cut at `message.list_content_max_bytes` with `content_truncated`.
  @BuiltValueField(wireName: r'content_md')
  String get contentMd;

  /// Version of the Markdown dialect; `1` is CommonMark 0.31.2 without raw HTML, with mention tokens (03 `messages`).
  @BuiltValueField(wireName: r'content_format')
  int get contentFormat;

  /// Set when `content_md` was cut (events above `sync.event_payload_max_bytes`, lists above `message.list_content_max_bytes`); `getMessage` returns the full content.
  @BuiltValueField(wireName: r'content_truncated')
  bool? get contentTruncated;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'reply_to_message_id')
  String? get replyToMessageId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String? get runId;

  @BuiltValueField(wireName: r'trigger_state')
  TriggerState? get triggerState;
  // enum triggerStateEnum {  received,  queued,  working,  done,  failed,  cancelled,  followup_delivered,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'trigger_state_at')
  DateTime? get triggerStateAt;

  /// Reason when `trigger_state` is `failed` or `cancelled`, e.g. `billing_mode_personal_not_holder`, `run_active_other_agent`, `user_cancelled`.
  @BuiltValueField(wireName: r'trigger_state_reason')
  String? get triggerStateReason;

  @BuiltValueField(wireName: r'silent')
  bool get silent;

  @BuiltValueField(wireName: r'attachments')
  BuiltList<Attachment> get attachments;

  /// Principals resolved from the mention tokens of `content_md`, sorted. Clients render a token as a mention only for these.
  @BuiltValueField(wireName: r'mention_ids')
  BuiltList<String> get mentionIds;

  @BuiltValueField(wireName: r'reactions')
  BuiltList<ReactionSummary> get reactions;

  /// Emoji the caller reacted with. Only in REST responses, never in events.
  @BuiltValueField(wireName: r'my_reactions')
  BuiltList<String>? get myReactions;

  @BuiltValueField(wireName: r'pinned')
  bool get pinned;

  @BuiltValueField(wireName: r'components')
  BuiltList<Component> get components;

  @BuiltValueField(wireName: r'version')
  int get version;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'edited_at')
  DateTime? get editedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'deleted_at')
  DateTime? get deletedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  Message._();

  factory Message([void updates(MessageBuilder b)]) = _$Message;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(MessageBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Message> get serializer => _$MessageSerializer();
}

class _$MessageSerializer implements PrimitiveSerializer<Message> {
  @override
  final Iterable<Type> types = const [Message, _$Message];

  @override
  final String wireName = r'Message';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Message object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    yield r'thread_seq';
    yield serializers.serialize(
      object.threadSeq,
      specifiedType: const FullType(int),
    );
    yield r'author';
    yield serializers.serialize(
      object.author,
      specifiedType: const FullType(PrincipalRef),
    );
    yield r'client_msg_id';
    yield serializers.serialize(
      object.clientMsgId,
      specifiedType: const FullType(String),
    );
    yield r'origin';
    yield serializers.serialize(
      object.origin,
      specifiedType: const FullType(MessageOrigin),
    );
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(MessageKind),
    );
    if (object.systemCode != null) {
      yield r'system_code';
      yield serializers.serialize(
        object.systemCode,
        specifiedType: const FullType(MessageSystemCodeEnum),
      );
    }
    yield r'content_md';
    yield serializers.serialize(
      object.contentMd,
      specifiedType: const FullType(String),
    );
    yield r'content_format';
    yield serializers.serialize(
      object.contentFormat,
      specifiedType: const FullType(int),
    );
    if (object.contentTruncated != null) {
      yield r'content_truncated';
      yield serializers.serialize(
        object.contentTruncated,
        specifiedType: const FullType(bool),
      );
    }
    if (object.replyToMessageId != null) {
      yield r'reply_to_message_id';
      yield serializers.serialize(
        object.replyToMessageId,
        specifiedType: const FullType(String),
      );
    }
    if (object.runId != null) {
      yield r'run_id';
      yield serializers.serialize(
        object.runId,
        specifiedType: const FullType(String),
      );
    }
    if (object.triggerState != null) {
      yield r'trigger_state';
      yield serializers.serialize(
        object.triggerState,
        specifiedType: const FullType(TriggerState),
      );
    }
    if (object.triggerStateAt != null) {
      yield r'trigger_state_at';
      yield serializers.serialize(
        object.triggerStateAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.triggerStateReason != null) {
      yield r'trigger_state_reason';
      yield serializers.serialize(
        object.triggerStateReason,
        specifiedType: const FullType(String),
      );
    }
    yield r'silent';
    yield serializers.serialize(
      object.silent,
      specifiedType: const FullType(bool),
    );
    yield r'attachments';
    yield serializers.serialize(
      object.attachments,
      specifiedType: const FullType(BuiltList, [FullType(Attachment)]),
    );
    yield r'mention_ids';
    yield serializers.serialize(
      object.mentionIds,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    yield r'reactions';
    yield serializers.serialize(
      object.reactions,
      specifiedType: const FullType(BuiltList, [FullType(ReactionSummary)]),
    );
    if (object.myReactions != null) {
      yield r'my_reactions';
      yield serializers.serialize(
        object.myReactions,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    yield r'pinned';
    yield serializers.serialize(
      object.pinned,
      specifiedType: const FullType(bool),
    );
    yield r'components';
    yield serializers.serialize(
      object.components,
      specifiedType: const FullType(BuiltList, [FullType(Component)]),
    );
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
    if (object.editedAt != null) {
      yield r'edited_at';
      yield serializers.serialize(
        object.editedAt,
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
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Message object, {
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
    required MessageBuilder result,
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
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'thread_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.threadSeq = valueDes;
          break;
        case r'author':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PrincipalRef),
          ) as PrincipalRef;
          result.author.replace(valueDes);
          break;
        case r'client_msg_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.clientMsgId = valueDes;
          break;
        case r'origin':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(MessageOrigin),
          ) as MessageOrigin;
          result.origin = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(MessageKind),
          ) as MessageKind;
          result.kind = valueDes;
          break;
        case r'system_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(MessageSystemCodeEnum),
          ) as MessageSystemCodeEnum?;
          if (valueDes == null) continue;
          result.systemCode = valueDes;
          break;
        case r'content_md':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.contentMd = valueDes;
          break;
        case r'content_format':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.contentFormat = valueDes;
          break;
        case r'content_truncated':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.contentTruncated = valueDes;
          break;
        case r'reply_to_message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.replyToMessageId = valueDes;
          break;
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.runId = valueDes;
          break;
        case r'trigger_state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(TriggerState),
          ) as TriggerState?;
          if (valueDes == null) continue;
          result.triggerState = valueDes;
          break;
        case r'trigger_state_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.triggerStateAt = valueDes;
          break;
        case r'trigger_state_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.triggerStateReason = valueDes;
          break;
        case r'silent':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.silent = valueDes;
          break;
        case r'attachments':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Attachment)]),
          ) as BuiltList<Attachment>;
          result.attachments.replace(valueDes);
          break;
        case r'mention_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.mentionIds.replace(valueDes);
          break;
        case r'reactions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(ReactionSummary),
            ]),
          ) as BuiltList<ReactionSummary>;
          result.reactions.replace(valueDes);
          break;
        case r'my_reactions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.myReactions.replace(valueDes);
          break;
        case r'pinned':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.pinned = valueDes;
          break;
        case r'components':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Component)]),
          ) as BuiltList<Component>;
          result.components.replace(valueDes);
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        case r'edited_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.editedAt = valueDes;
          break;
        case r'deleted_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.deletedAt = valueDes;
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
  Message deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = MessageBuilder();
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

/// Set exactly for `kind = system`; clients render the localized line from it and `author`. `content_md` is empty then.
class MessageSystemCodeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'thread.reopened')
  static const MessageSystemCodeEnum threadPeriodReopened =
      _$messageSystemCodeEnum_threadPeriodReopened;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const MessageSystemCodeEnum unknownDefaultOpenApi =
      _$messageSystemCodeEnum_unknownDefaultOpenApi;

  static Serializer<MessageSystemCodeEnum> get serializer =>
      _$messageSystemCodeEnumSerializer;

  const MessageSystemCodeEnum._(String name) : super(name);

  static BuiltSet<MessageSystemCodeEnum> get values =>
      _$messageSystemCodeEnumValues;
  static MessageSystemCodeEnum valueOf(String name) =>
      _$messageSystemCodeEnumValueOf(name);
}

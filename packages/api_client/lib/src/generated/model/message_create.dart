//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/component_create.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'message_create.g.dart';

/// MessageCreate
///
/// Properties:
/// * [clientMsgId] - UUIDv7 created by the client before the first attempt; unique per author forever.
/// * [kind]
/// * [contentMd]
/// * [replyToMessageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [attachmentFileIds]
/// * [mentionIds] - Optional check. The server resolves mentions from `content_md` (03 `messages`, content format; 07 section 4.6); when present, the set must equal the resolved set, else `400 validation_failed` with code `mention_mismatch`.
/// * [silent]
/// * [clearDraft]
/// * [runId] - Agents only; the run this message belongs to. It must name an active run (`queued`, `running`, `waiting_input`) of the calling agent in this thread. Never stored on the message (`Message.run_id` stays absent). A person or app, an unknown run, a run of another thread or of another agent answer alike `400 validation_failed` with `errors[].code` `run_mismatch`; an ended own run answers `409 conflict` with `reason` `run_not_active`. With `kind = card` it also binds the components of the card to this run (`Component.run_id`, 09 11.4) and is part of the canonical request of `client_msg_id` (`bothy-msg-v2`); for every other kind it is not (09 4.7).
/// * [components] - Requires `kind = card`, and `kind = card` requires at least one component; at most `component.max_per_message`. Only agents send cards (people get `invalid`; apps `not_enabled` until app components are built). The whole `message.created` payload of the card must fit `sync.event_payload_max_bytes`, else `too_long` on `/components`. An agent whose owner switched `supports_components` off gets `403 feature_disabled` (09, 2.2 and 11.1).
@BuiltValue()
abstract class MessageCreate
    implements Built<MessageCreate, MessageCreateBuilder> {
  /// UUIDv7 created by the client before the first attempt; unique per author forever.
  @BuiltValueField(wireName: r'client_msg_id')
  String get clientMsgId;

  @BuiltValueField(wireName: r'kind')
  MessageCreateKindEnum? get kind;
  // enum kindEnum {  text,  voice,  card,  };

  @BuiltValueField(wireName: r'content_md')
  String? get contentMd;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'reply_to_message_id')
  String? get replyToMessageId;

  @BuiltValueField(wireName: r'attachment_file_ids')
  BuiltList<String>? get attachmentFileIds;

  /// Optional check. The server resolves mentions from `content_md` (03 `messages`, content format; 07 section 4.6); when present, the set must equal the resolved set, else `400 validation_failed` with code `mention_mismatch`.
  @BuiltValueField(wireName: r'mention_ids')
  BuiltList<String>? get mentionIds;

  @BuiltValueField(wireName: r'silent')
  bool? get silent;

  @BuiltValueField(wireName: r'clear_draft')
  bool? get clearDraft;

  /// Agents only; the run this message belongs to. It must name an active run (`queued`, `running`, `waiting_input`) of the calling agent in this thread. Never stored on the message (`Message.run_id` stays absent). A person or app, an unknown run, a run of another thread or of another agent answer alike `400 validation_failed` with `errors[].code` `run_mismatch`; an ended own run answers `409 conflict` with `reason` `run_not_active`. With `kind = card` it also binds the components of the card to this run (`Component.run_id`, 09 11.4) and is part of the canonical request of `client_msg_id` (`bothy-msg-v2`); for every other kind it is not (09 4.7).
  @BuiltValueField(wireName: r'run_id')
  String? get runId;

  /// Requires `kind = card`, and `kind = card` requires at least one component; at most `component.max_per_message`. Only agents send cards (people get `invalid`; apps `not_enabled` until app components are built). The whole `message.created` payload of the card must fit `sync.event_payload_max_bytes`, else `too_long` on `/components`. An agent whose owner switched `supports_components` off gets `403 feature_disabled` (09, 2.2 and 11.1).
  @BuiltValueField(wireName: r'components')
  BuiltList<ComponentCreate>? get components;

  MessageCreate._();

  factory MessageCreate([void updates(MessageCreateBuilder b)]) =
      _$MessageCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(MessageCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<MessageCreate> get serializer =>
      _$MessageCreateSerializer();
}

class _$MessageCreateSerializer implements PrimitiveSerializer<MessageCreate> {
  @override
  final Iterable<Type> types = const [MessageCreate, _$MessageCreate];

  @override
  final String wireName = r'MessageCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    MessageCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'client_msg_id';
    yield serializers.serialize(
      object.clientMsgId,
      specifiedType: const FullType(String),
    );
    if (object.kind != null) {
      yield r'kind';
      yield serializers.serialize(
        object.kind,
        specifiedType: const FullType(MessageCreateKindEnum),
      );
    }
    if (object.contentMd != null) {
      yield r'content_md';
      yield serializers.serialize(
        object.contentMd,
        specifiedType: const FullType(String),
      );
    }
    if (object.replyToMessageId != null) {
      yield r'reply_to_message_id';
      yield serializers.serialize(
        object.replyToMessageId,
        specifiedType: const FullType(String),
      );
    }
    if (object.attachmentFileIds != null) {
      yield r'attachment_file_ids';
      yield serializers.serialize(
        object.attachmentFileIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.mentionIds != null) {
      yield r'mention_ids';
      yield serializers.serialize(
        object.mentionIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.silent != null) {
      yield r'silent';
      yield serializers.serialize(
        object.silent,
        specifiedType: const FullType(bool),
      );
    }
    if (object.clearDraft != null) {
      yield r'clear_draft';
      yield serializers.serialize(
        object.clearDraft,
        specifiedType: const FullType(bool),
      );
    }
    if (object.runId != null) {
      yield r'run_id';
      yield serializers.serialize(
        object.runId,
        specifiedType: const FullType(String),
      );
    }
    if (object.components != null) {
      yield r'components';
      yield serializers.serialize(
        object.components,
        specifiedType: const FullType(BuiltList, [FullType(ComponentCreate)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    MessageCreate object, {
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
    required MessageCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'client_msg_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.clientMsgId = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(MessageCreateKindEnum),
          ) as MessageCreateKindEnum?;
          if (valueDes == null) continue;
          result.kind = valueDes;
          break;
        case r'content_md':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.contentMd = valueDes;
          break;
        case r'reply_to_message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.replyToMessageId = valueDes;
          break;
        case r'attachment_file_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.attachmentFileIds.replace(valueDes);
          break;
        case r'mention_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.mentionIds.replace(valueDes);
          break;
        case r'silent':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.silent = valueDes;
          break;
        case r'clear_draft':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.clearDraft = valueDes;
          break;
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.runId = valueDes;
          break;
        case r'components':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(ComponentCreate),
            ]),
          ) as BuiltList<ComponentCreate>?;
          if (valueDes == null) continue;
          result.components.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  MessageCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = MessageCreateBuilder();
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

class MessageCreateKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'text')
  static const MessageCreateKindEnum text = _$messageCreateKindEnum_text;
  @BuiltValueEnumConst(wireName: r'voice')
  static const MessageCreateKindEnum voice = _$messageCreateKindEnum_voice;
  @BuiltValueEnumConst(wireName: r'card')
  static const MessageCreateKindEnum card = _$messageCreateKindEnum_card;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const MessageCreateKindEnum unknownDefaultOpenApi =
      _$messageCreateKindEnum_unknownDefaultOpenApi;

  static Serializer<MessageCreateKindEnum> get serializer =>
      _$messageCreateKindEnumSerializer;

  const MessageCreateKindEnum._(String name) : super(name);

  static BuiltSet<MessageCreateKindEnum> get values =>
      _$messageCreateKindEnumValues;
  static MessageCreateKindEnum valueOf(String name) =>
      _$messageCreateKindEnumValueOf(name);
}

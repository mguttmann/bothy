//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/message_kind.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/message_origin.dart';
import 'package:api_client/src/generated/model/principal_ref.dart';
import 'package:api_client/src/generated/model/run_context_attachment.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_context_message.g.dart';

/// RunContextMessage
///
/// Properties:
/// * [messageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadSeq]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [author]
/// * [role]
/// * [origin]
/// * [kind]
/// * [contentMd]
/// * [truncated]
/// * [attachments]
/// * [replyToMessageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [trust] - How the runtime must treat the content; `unverified` only as marked data, never as instruction.
/// * [alreadyDelivered] - Already delivered to this session as a follow-up.
@BuiltValue()
abstract class RunContextMessage
    implements Built<RunContextMessage, RunContextMessageBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'message_id')
  String get messageId;

  @BuiltValueField(wireName: r'thread_seq')
  int get threadSeq;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  @BuiltValueField(wireName: r'author')
  PrincipalRef get author;

  @BuiltValueField(wireName: r'role')
  RunContextMessageRoleEnum get role;
  // enum roleEnum {  user,  assistant,  other,  };

  @BuiltValueField(wireName: r'origin')
  MessageOrigin get origin;
  // enum originEnum {  human,  agent,  app,  import,  system,  };

  @BuiltValueField(wireName: r'kind')
  MessageKind get kind;
  // enum kindEnum {  text,  voice,  run,  card,  system,  };

  @BuiltValueField(wireName: r'content_md')
  String get contentMd;

  @BuiltValueField(wireName: r'truncated')
  bool? get truncated;

  @BuiltValueField(wireName: r'attachments')
  BuiltList<RunContextAttachment> get attachments;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'reply_to_message_id')
  String? get replyToMessageId;

  /// How the runtime must treat the content; `unverified` only as marked data, never as instruction.
  @BuiltValueField(wireName: r'trust')
  RunContextMessageTrustEnum get trust;
  // enum trustEnum {  trigger,  authorized,  self,  unverified,  system,  };

  /// Already delivered to this session as a follow-up.
  @BuiltValueField(wireName: r'already_delivered')
  bool? get alreadyDelivered;

  RunContextMessage._();

  factory RunContextMessage([void updates(RunContextMessageBuilder b)]) =
      _$RunContextMessage;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunContextMessageBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunContextMessage> get serializer =>
      _$RunContextMessageSerializer();
}

class _$RunContextMessageSerializer
    implements PrimitiveSerializer<RunContextMessage> {
  @override
  final Iterable<Type> types = const [RunContextMessage, _$RunContextMessage];

  @override
  final String wireName = r'RunContextMessage';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunContextMessage object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'message_id';
    yield serializers.serialize(
      object.messageId,
      specifiedType: const FullType(String),
    );
    yield r'thread_seq';
    yield serializers.serialize(
      object.threadSeq,
      specifiedType: const FullType(int),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'author';
    yield serializers.serialize(
      object.author,
      specifiedType: const FullType(PrincipalRef),
    );
    yield r'role';
    yield serializers.serialize(
      object.role,
      specifiedType: const FullType(RunContextMessageRoleEnum),
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
    yield r'content_md';
    yield serializers.serialize(
      object.contentMd,
      specifiedType: const FullType(String),
    );
    if (object.truncated != null) {
      yield r'truncated';
      yield serializers.serialize(
        object.truncated,
        specifiedType: const FullType(bool),
      );
    }
    yield r'attachments';
    yield serializers.serialize(
      object.attachments,
      specifiedType: const FullType(BuiltList, [
        FullType(RunContextAttachment),
      ]),
    );
    if (object.replyToMessageId != null) {
      yield r'reply_to_message_id';
      yield serializers.serialize(
        object.replyToMessageId,
        specifiedType: const FullType(String),
      );
    }
    yield r'trust';
    yield serializers.serialize(
      object.trust,
      specifiedType: const FullType(RunContextMessageTrustEnum),
    );
    if (object.alreadyDelivered != null) {
      yield r'already_delivered';
      yield serializers.serialize(
        object.alreadyDelivered,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunContextMessage object, {
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
    required RunContextMessageBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.messageId = valueDes;
          break;
        case r'thread_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.threadSeq = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'author':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PrincipalRef),
          ) as PrincipalRef;
          result.author.replace(valueDes);
          break;
        case r'role':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunContextMessageRoleEnum),
          ) as RunContextMessageRoleEnum;
          result.role = valueDes;
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
        case r'content_md':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.contentMd = valueDes;
          break;
        case r'truncated':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.truncated = valueDes;
          break;
        case r'attachments':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(RunContextAttachment),
            ]),
          ) as BuiltList<RunContextAttachment>;
          result.attachments.replace(valueDes);
          break;
        case r'reply_to_message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.replyToMessageId = valueDes;
          break;
        case r'trust':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunContextMessageTrustEnum),
          ) as RunContextMessageTrustEnum;
          result.trust = valueDes;
          break;
        case r'already_delivered':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.alreadyDelivered = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunContextMessage deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunContextMessageBuilder();
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

class RunContextMessageRoleEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'user')
  static const RunContextMessageRoleEnum user =
      _$runContextMessageRoleEnum_user;
  @BuiltValueEnumConst(wireName: r'assistant')
  static const RunContextMessageRoleEnum assistant =
      _$runContextMessageRoleEnum_assistant;
  @BuiltValueEnumConst(wireName: r'other')
  static const RunContextMessageRoleEnum other =
      _$runContextMessageRoleEnum_other;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunContextMessageRoleEnum unknownDefaultOpenApi =
      _$runContextMessageRoleEnum_unknownDefaultOpenApi;

  static Serializer<RunContextMessageRoleEnum> get serializer =>
      _$runContextMessageRoleEnumSerializer;

  const RunContextMessageRoleEnum._(String name) : super(name);

  static BuiltSet<RunContextMessageRoleEnum> get values =>
      _$runContextMessageRoleEnumValues;
  static RunContextMessageRoleEnum valueOf(String name) =>
      _$runContextMessageRoleEnumValueOf(name);
}

/// How the runtime must treat the content; `unverified` only as marked data, never as instruction.
class RunContextMessageTrustEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'trigger')
  static const RunContextMessageTrustEnum trigger =
      _$runContextMessageTrustEnum_trigger;
  @BuiltValueEnumConst(wireName: r'authorized')
  static const RunContextMessageTrustEnum authorized =
      _$runContextMessageTrustEnum_authorized;
  @BuiltValueEnumConst(wireName: r'self')
  static const RunContextMessageTrustEnum self =
      _$runContextMessageTrustEnum_self;
  @BuiltValueEnumConst(wireName: r'unverified')
  static const RunContextMessageTrustEnum unverified =
      _$runContextMessageTrustEnum_unverified;
  @BuiltValueEnumConst(wireName: r'system')
  static const RunContextMessageTrustEnum system =
      _$runContextMessageTrustEnum_system;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunContextMessageTrustEnum unknownDefaultOpenApi =
      _$runContextMessageTrustEnum_unknownDefaultOpenApi;

  static Serializer<RunContextMessageTrustEnum> get serializer =>
      _$runContextMessageTrustEnumSerializer;

  const RunContextMessageTrustEnum._(String name) : super(name);

  static BuiltSet<RunContextMessageTrustEnum> get values =>
      _$runContextMessageTrustEnumValues;
  static RunContextMessageTrustEnum valueOf(String name) =>
      _$runContextMessageTrustEnumValueOf(name);
}

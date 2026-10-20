//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/alertmanager_alert.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'incoming_webhook_post.g.dart';

/// Body of an incoming webhook call in one of two shapes, recognised per request (11 8.2, docs/DECISIONS.md D-M2-27-5, D-M7-20-5). A body with at least one Alertmanager member (`version`, `groupKey`, `truncatedAlerts`, `status`, `receiver`, `groupLabels`, `commonLabels`, `commonAnnotations`, `externalURL`, `alerts`) is the unchanged webhook payload of Prometheus Alertmanager (format `alertmanager`) and must contain no native member; other members of such a payload, at the top and in each alert, are ignored, because Alertmanager adds members in new versions and drops a notification answered with `400`. Every other body is `native`, needs `content_md` and refuses every other member with `400 validation_failed` and `errors[].code` `unknown_member`.
///
/// Properties:
/// * [contentMd] - Native. Markdown content, at most `message.max_content_bytes` UTF-8 bytes.
/// * [clientMsgId] - Native. Optional UUIDv7; makes retries of the sender idempotent.
/// * [title] - Native. Title of a new thread when the webhook has no fixed thread; required in a forum.
/// * [threadId] - Native. Thread of the webhook's channel to write into when the webhook has no fixed thread.
/// * [version] - Alertmanager. Version of the webhook payload.
/// * [groupKey] - Alertmanager. Key of the alert group.
/// * [truncatedAlerts] - Alertmanager. Number of alerts left out of `alerts`.
/// * [status] - Alertmanager. State of the group.
/// * [receiver] - Alertmanager. Name of the receiver.
/// * [groupLabels] - Alertmanager. Labels of the group; every value is a string.
/// * [commonLabels] - Alertmanager. Labels all alerts share; every value is a string.
/// * [commonAnnotations] - Alertmanager. Annotations all alerts share; every value is a string.
/// * [externalURL] - Alertmanager. Address of the sending Alertmanager; written as text, never fetched.
/// * [alerts] - Alertmanager. The alerts of the notification.
@BuiltValue()
abstract class IncomingWebhookPost
    implements Built<IncomingWebhookPost, IncomingWebhookPostBuilder> {
  /// Native. Markdown content, at most `message.max_content_bytes` UTF-8 bytes.
  @BuiltValueField(wireName: r'content_md')
  String? get contentMd;

  /// Native. Optional UUIDv7; makes retries of the sender idempotent.
  @BuiltValueField(wireName: r'client_msg_id')
  String? get clientMsgId;

  /// Native. Title of a new thread when the webhook has no fixed thread; required in a forum.
  @BuiltValueField(wireName: r'title')
  String? get title;

  /// Native. Thread of the webhook's channel to write into when the webhook has no fixed thread.
  @BuiltValueField(wireName: r'thread_id')
  String? get threadId;

  /// Alertmanager. Version of the webhook payload.
  @BuiltValueField(wireName: r'version')
  IncomingWebhookPostVersionEnum? get version;
  // enum versionEnum {  4,  };

  /// Alertmanager. Key of the alert group.
  @BuiltValueField(wireName: r'groupKey')
  String? get groupKey;

  /// Alertmanager. Number of alerts left out of `alerts`.
  @BuiltValueField(wireName: r'truncatedAlerts')
  int? get truncatedAlerts;

  /// Alertmanager. State of the group.
  @BuiltValueField(wireName: r'status')
  IncomingWebhookPostStatusEnum? get status;
  // enum statusEnum {  firing,  resolved,  };

  /// Alertmanager. Name of the receiver.
  @BuiltValueField(wireName: r'receiver')
  String? get receiver;

  /// Alertmanager. Labels of the group; every value is a string.
  @BuiltValueField(wireName: r'groupLabels')
  BuiltMap<String, JsonObject?>? get groupLabels;

  /// Alertmanager. Labels all alerts share; every value is a string.
  @BuiltValueField(wireName: r'commonLabels')
  BuiltMap<String, JsonObject?>? get commonLabels;

  /// Alertmanager. Annotations all alerts share; every value is a string.
  @BuiltValueField(wireName: r'commonAnnotations')
  BuiltMap<String, JsonObject?>? get commonAnnotations;

  /// Alertmanager. Address of the sending Alertmanager; written as text, never fetched.
  @BuiltValueField(wireName: r'externalURL')
  String? get externalURL;

  /// Alertmanager. The alerts of the notification.
  @BuiltValueField(wireName: r'alerts')
  BuiltList<AlertmanagerAlert>? get alerts;

  IncomingWebhookPost._();

  factory IncomingWebhookPost([void updates(IncomingWebhookPostBuilder b)]) =
      _$IncomingWebhookPost;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(IncomingWebhookPostBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<IncomingWebhookPost> get serializer =>
      _$IncomingWebhookPostSerializer();
}

class _$IncomingWebhookPostSerializer
    implements PrimitiveSerializer<IncomingWebhookPost> {
  @override
  final Iterable<Type> types = const [
    IncomingWebhookPost,
    _$IncomingWebhookPost,
  ];

  @override
  final String wireName = r'IncomingWebhookPost';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    IncomingWebhookPost object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.contentMd != null) {
      yield r'content_md';
      yield serializers.serialize(
        object.contentMd,
        specifiedType: const FullType(String),
      );
    }
    if (object.clientMsgId != null) {
      yield r'client_msg_id';
      yield serializers.serialize(
        object.clientMsgId,
        specifiedType: const FullType(String),
      );
    }
    if (object.title != null) {
      yield r'title';
      yield serializers.serialize(
        object.title,
        specifiedType: const FullType(String),
      );
    }
    if (object.threadId != null) {
      yield r'thread_id';
      yield serializers.serialize(
        object.threadId,
        specifiedType: const FullType(String),
      );
    }
    if (object.version != null) {
      yield r'version';
      yield serializers.serialize(
        object.version,
        specifiedType: const FullType(IncomingWebhookPostVersionEnum),
      );
    }
    if (object.groupKey != null) {
      yield r'groupKey';
      yield serializers.serialize(
        object.groupKey,
        specifiedType: const FullType(String),
      );
    }
    if (object.truncatedAlerts != null) {
      yield r'truncatedAlerts';
      yield serializers.serialize(
        object.truncatedAlerts,
        specifiedType: const FullType(int),
      );
    }
    if (object.status != null) {
      yield r'status';
      yield serializers.serialize(
        object.status,
        specifiedType: const FullType(IncomingWebhookPostStatusEnum),
      );
    }
    if (object.receiver != null) {
      yield r'receiver';
      yield serializers.serialize(
        object.receiver,
        specifiedType: const FullType(String),
      );
    }
    if (object.groupLabels != null) {
      yield r'groupLabels';
      yield serializers.serialize(
        object.groupLabels,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
    if (object.commonLabels != null) {
      yield r'commonLabels';
      yield serializers.serialize(
        object.commonLabels,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
    if (object.commonAnnotations != null) {
      yield r'commonAnnotations';
      yield serializers.serialize(
        object.commonAnnotations,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
    if (object.externalURL != null) {
      yield r'externalURL';
      yield serializers.serialize(
        object.externalURL,
        specifiedType: const FullType(String),
      );
    }
    if (object.alerts != null) {
      yield r'alerts';
      yield serializers.serialize(
        object.alerts,
        specifiedType: const FullType(BuiltList, [FullType(AlertmanagerAlert)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    IncomingWebhookPost object, {
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
    required IncomingWebhookPostBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'content_md':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.contentMd = valueDes;
          break;
        case r'client_msg_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.clientMsgId = valueDes;
          break;
        case r'title':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.title = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.threadId = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(
              IncomingWebhookPostVersionEnum,
            ),
          ) as IncomingWebhookPostVersionEnum?;
          if (valueDes == null) continue;
          result.version = valueDes;
          break;
        case r'groupKey':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.groupKey = valueDes;
          break;
        case r'truncatedAlerts':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.truncatedAlerts = valueDes;
          break;
        case r'status':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(
              IncomingWebhookPostStatusEnum,
            ),
          ) as IncomingWebhookPostStatusEnum?;
          if (valueDes == null) continue;
          result.status = valueDes;
          break;
        case r'receiver':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.receiver = valueDes;
          break;
        case r'groupLabels':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.groupLabels.replace(valueDes);
          break;
        case r'commonLabels':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.commonLabels.replace(valueDes);
          break;
        case r'commonAnnotations':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.commonAnnotations.replace(valueDes);
          break;
        case r'externalURL':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.externalURL = valueDes;
          break;
        case r'alerts':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(AlertmanagerAlert),
            ]),
          ) as BuiltList<AlertmanagerAlert>?;
          if (valueDes == null) continue;
          result.alerts.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  IncomingWebhookPost deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = IncomingWebhookPostBuilder();
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

/// Alertmanager. Version of the webhook payload.
class IncomingWebhookPostVersionEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'4')
  static const IncomingWebhookPostVersionEnum n4 =
      _$incomingWebhookPostVersionEnum_n4;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const IncomingWebhookPostVersionEnum unknownDefaultOpenApi =
      _$incomingWebhookPostVersionEnum_unknownDefaultOpenApi;

  static Serializer<IncomingWebhookPostVersionEnum> get serializer =>
      _$incomingWebhookPostVersionEnumSerializer;

  const IncomingWebhookPostVersionEnum._(String name) : super(name);

  static BuiltSet<IncomingWebhookPostVersionEnum> get values =>
      _$incomingWebhookPostVersionEnumValues;
  static IncomingWebhookPostVersionEnum valueOf(String name) =>
      _$incomingWebhookPostVersionEnumValueOf(name);
}

/// Alertmanager. State of the group.
class IncomingWebhookPostStatusEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'firing')
  static const IncomingWebhookPostStatusEnum firing =
      _$incomingWebhookPostStatusEnum_firing;
  @BuiltValueEnumConst(wireName: r'resolved')
  static const IncomingWebhookPostStatusEnum resolved =
      _$incomingWebhookPostStatusEnum_resolved;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const IncomingWebhookPostStatusEnum unknownDefaultOpenApi =
      _$incomingWebhookPostStatusEnum_unknownDefaultOpenApi;

  static Serializer<IncomingWebhookPostStatusEnum> get serializer =>
      _$incomingWebhookPostStatusEnumSerializer;

  const IncomingWebhookPostStatusEnum._(String name) : super(name);

  static BuiltSet<IncomingWebhookPostStatusEnum> get values =>
      _$incomingWebhookPostStatusEnumValues;
  static IncomingWebhookPostStatusEnum valueOf(String name) =>
      _$incomingWebhookPostStatusEnumValueOf(name);
}

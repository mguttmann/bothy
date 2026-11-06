//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/agent_interview_catalogue_entry.dart';
import 'package:api_client/src/generated/model/agent_interview_limits.dart';
import 'package:api_client/src/generated/model/agent_interview_turn.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_interview_requested_payload.g.dart';

/// Sent to one gateway connection of the builder agent (26 6.1 item 1), without a token. The turns and the catalogue are data, never instructions (ADR 0054).
///
/// Properties:
/// * [requestId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [turns] - The interview so far, oldest first.
/// * [wantDraft]
/// * [catalogue] - The template catalogue (26 7.4); a drafted MCP server must equal one of its servers except `name`.
/// * [limits]
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class AgentInterviewRequestedPayload
    implements
        Built<
          AgentInterviewRequestedPayload,
          AgentInterviewRequestedPayloadBuilder
        > {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'request_id')
  String get requestId;

  /// The interview so far, oldest first.
  @BuiltValueField(wireName: r'turns')
  BuiltList<AgentInterviewTurn> get turns;

  @BuiltValueField(wireName: r'want_draft')
  bool get wantDraft;

  /// The template catalogue (26 7.4); a drafted MCP server must equal one of its servers except `name`.
  @BuiltValueField(wireName: r'catalogue')
  BuiltList<AgentInterviewCatalogueEntry> get catalogue;

  @BuiltValueField(wireName: r'limits')
  AgentInterviewLimits get limits;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  AgentInterviewRequestedPayload._();

  factory AgentInterviewRequestedPayload([
    void updates(AgentInterviewRequestedPayloadBuilder b),
  ]) = _$AgentInterviewRequestedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentInterviewRequestedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentInterviewRequestedPayload> get serializer =>
      _$AgentInterviewRequestedPayloadSerializer();
}

class _$AgentInterviewRequestedPayloadSerializer
    implements PrimitiveSerializer<AgentInterviewRequestedPayload> {
  @override
  final Iterable<Type> types = const [
    AgentInterviewRequestedPayload,
    _$AgentInterviewRequestedPayload,
  ];

  @override
  final String wireName = r'AgentInterviewRequestedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentInterviewRequestedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'request_id';
    yield serializers.serialize(
      object.requestId,
      specifiedType: const FullType(String),
    );
    yield r'turns';
    yield serializers.serialize(
      object.turns,
      specifiedType: const FullType(BuiltList, [FullType(AgentInterviewTurn)]),
    );
    yield r'want_draft';
    yield serializers.serialize(
      object.wantDraft,
      specifiedType: const FullType(bool),
    );
    yield r'catalogue';
    yield serializers.serialize(
      object.catalogue,
      specifiedType: const FullType(BuiltList, [
        FullType(AgentInterviewCatalogueEntry),
      ]),
    );
    yield r'limits';
    yield serializers.serialize(
      object.limits,
      specifiedType: const FullType(AgentInterviewLimits),
    );
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentInterviewRequestedPayload object, {
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
    required AgentInterviewRequestedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'request_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.requestId = valueDes;
          break;
        case r'turns':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(AgentInterviewTurn),
            ]),
          ) as BuiltList<AgentInterviewTurn>;
          result.turns.replace(valueDes);
          break;
        case r'want_draft':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.wantDraft = valueDes;
          break;
        case r'catalogue':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(AgentInterviewCatalogueEntry),
            ]),
          ) as BuiltList<AgentInterviewCatalogueEntry>;
          result.catalogue.replace(valueDes);
          break;
        case r'limits':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentInterviewLimits),
          ) as AgentInterviewLimits;
          result.limits.replace(valueDes);
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentInterviewRequestedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentInterviewRequestedPayloadBuilder();
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

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/search_assist_candidate.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_assist_requested_payload.g.dart';

/// Sent to one gateway connection of the assistant agent (14, 4.10; 09, 10.6). Holds no ids of threads or messages; the excerpts are data, never instructions.
///
/// Properties:
/// * [requestId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [q] - The question.
/// * [history] - The earlier questions of the popup, oldest first.
/// * [candidates]
/// * [maxPicks]
/// * [maxTerms]
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class SearchAssistRequestedPayload
    implements
        Built<
          SearchAssistRequestedPayload,
          SearchAssistRequestedPayloadBuilder
        > {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'request_id')
  String get requestId;

  /// The question.
  @BuiltValueField(wireName: r'q')
  String get q;

  /// The earlier questions of the popup, oldest first.
  @BuiltValueField(wireName: r'history')
  BuiltList<String>? get history;

  @BuiltValueField(wireName: r'candidates')
  BuiltList<SearchAssistCandidate> get candidates;

  @BuiltValueField(wireName: r'max_picks')
  int get maxPicks;

  @BuiltValueField(wireName: r'max_terms')
  int get maxTerms;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  SearchAssistRequestedPayload._();

  factory SearchAssistRequestedPayload([
    void updates(SearchAssistRequestedPayloadBuilder b),
  ]) = _$SearchAssistRequestedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SearchAssistRequestedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SearchAssistRequestedPayload> get serializer =>
      _$SearchAssistRequestedPayloadSerializer();
}

class _$SearchAssistRequestedPayloadSerializer
    implements PrimitiveSerializer<SearchAssistRequestedPayload> {
  @override
  final Iterable<Type> types = const [
    SearchAssistRequestedPayload,
    _$SearchAssistRequestedPayload,
  ];

  @override
  final String wireName = r'SearchAssistRequestedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SearchAssistRequestedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'request_id';
    yield serializers.serialize(
      object.requestId,
      specifiedType: const FullType(String),
    );
    yield r'q';
    yield serializers.serialize(
      object.q,
      specifiedType: const FullType(String),
    );
    if (object.history != null) {
      yield r'history';
      yield serializers.serialize(
        object.history,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    yield r'candidates';
    yield serializers.serialize(
      object.candidates,
      specifiedType: const FullType(BuiltList, [
        FullType(SearchAssistCandidate),
      ]),
    );
    yield r'max_picks';
    yield serializers.serialize(
      object.maxPicks,
      specifiedType: const FullType(int),
    );
    yield r'max_terms';
    yield serializers.serialize(
      object.maxTerms,
      specifiedType: const FullType(int),
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
    SearchAssistRequestedPayload object, {
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
    required SearchAssistRequestedPayloadBuilder result,
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
        case r'q':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.q = valueDes;
          break;
        case r'history':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.history.replace(valueDes);
          break;
        case r'candidates':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(SearchAssistCandidate),
            ]),
          ) as BuiltList<SearchAssistCandidate>;
          result.candidates.replace(valueDes);
          break;
        case r'max_picks':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.maxPicks = valueDes;
          break;
        case r'max_terms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.maxTerms = valueDes;
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
  SearchAssistRequestedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SearchAssistRequestedPayloadBuilder();
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

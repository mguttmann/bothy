//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'step_up_oidc_begin_request.g.dart';

/// StepUpOidcBeginRequest
///
/// Properties:
/// * [challengeId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [providerId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [appRedirectUri]
/// * [clientChallenge]
@BuiltValue()
abstract class StepUpOidcBeginRequest
    implements Built<StepUpOidcBeginRequest, StepUpOidcBeginRequestBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'challenge_id')
  String get challengeId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'provider_id')
  String get providerId;

  @BuiltValueField(wireName: r'app_redirect_uri')
  String get appRedirectUri;

  @BuiltValueField(wireName: r'client_challenge')
  String get clientChallenge;

  StepUpOidcBeginRequest._();

  factory StepUpOidcBeginRequest([
    void updates(StepUpOidcBeginRequestBuilder b),
  ]) = _$StepUpOidcBeginRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StepUpOidcBeginRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StepUpOidcBeginRequest> get serializer =>
      _$StepUpOidcBeginRequestSerializer();
}

class _$StepUpOidcBeginRequestSerializer
    implements PrimitiveSerializer<StepUpOidcBeginRequest> {
  @override
  final Iterable<Type> types = const [
    StepUpOidcBeginRequest,
    _$StepUpOidcBeginRequest,
  ];

  @override
  final String wireName = r'StepUpOidcBeginRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StepUpOidcBeginRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'challenge_id';
    yield serializers.serialize(
      object.challengeId,
      specifiedType: const FullType(String),
    );
    yield r'provider_id';
    yield serializers.serialize(
      object.providerId,
      specifiedType: const FullType(String),
    );
    yield r'app_redirect_uri';
    yield serializers.serialize(
      object.appRedirectUri,
      specifiedType: const FullType(String),
    );
    yield r'client_challenge';
    yield serializers.serialize(
      object.clientChallenge,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StepUpOidcBeginRequest object, {
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
    required StepUpOidcBeginRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'challenge_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.challengeId = valueDes;
          break;
        case r'provider_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.providerId = valueDes;
          break;
        case r'app_redirect_uri':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.appRedirectUri = valueDes;
          break;
        case r'client_challenge':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.clientChallenge = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StepUpOidcBeginRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StepUpOidcBeginRequestBuilder();
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

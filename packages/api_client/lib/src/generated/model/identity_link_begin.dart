//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'identity_link_begin.g.dart';

/// IdentityLinkBegin
///
/// Properties:
/// * [providerId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [appRedirectUri]
/// * [clientChallenge]
@BuiltValue()
abstract class IdentityLinkBegin
    implements Built<IdentityLinkBegin, IdentityLinkBeginBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'provider_id')
  String get providerId;

  @BuiltValueField(wireName: r'app_redirect_uri')
  String get appRedirectUri;

  @BuiltValueField(wireName: r'client_challenge')
  String get clientChallenge;

  IdentityLinkBegin._();

  factory IdentityLinkBegin([void updates(IdentityLinkBeginBuilder b)]) =
      _$IdentityLinkBegin;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(IdentityLinkBeginBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<IdentityLinkBegin> get serializer =>
      _$IdentityLinkBeginSerializer();
}

class _$IdentityLinkBeginSerializer
    implements PrimitiveSerializer<IdentityLinkBegin> {
  @override
  final Iterable<Type> types = const [IdentityLinkBegin, _$IdentityLinkBegin];

  @override
  final String wireName = r'IdentityLinkBegin';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    IdentityLinkBegin object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
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
    IdentityLinkBegin object, {
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
    required IdentityLinkBeginBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
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
  IdentityLinkBegin deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = IdentityLinkBeginBuilder();
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

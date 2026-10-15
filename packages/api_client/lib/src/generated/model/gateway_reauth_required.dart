//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_reauth_required.g.dart';

/// GatewayReauthRequired
///
/// Properties:
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [dpopNonce]
@BuiltValue()
abstract class GatewayReauthRequired
    implements Built<GatewayReauthRequired, GatewayReauthRequiredBuilder> {
  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  @BuiltValueField(wireName: r'dpop_nonce')
  String get dpopNonce;

  GatewayReauthRequired._();

  factory GatewayReauthRequired([
    void updates(GatewayReauthRequiredBuilder b),
  ]) = _$GatewayReauthRequired;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayReauthRequiredBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayReauthRequired> get serializer =>
      _$GatewayReauthRequiredSerializer();
}

class _$GatewayReauthRequiredSerializer
    implements PrimitiveSerializer<GatewayReauthRequired> {
  @override
  final Iterable<Type> types = const [
    GatewayReauthRequired,
    _$GatewayReauthRequired,
  ];

  @override
  final String wireName = r'GatewayReauthRequired';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayReauthRequired object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'dpop_nonce';
    yield serializers.serialize(
      object.dpopNonce,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayReauthRequired object, {
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
    required GatewayReauthRequiredBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        case r'dpop_nonce':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.dpopNonce = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayReauthRequired deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayReauthRequiredBuilder();
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

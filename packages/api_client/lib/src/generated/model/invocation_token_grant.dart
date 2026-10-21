//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'invocation_token_grant.g.dart';

/// InvocationTokenGrant
///
/// Properties:
/// * [invocationToken] - Fresh invocation token with a new `jti`; it replaces the current one of the run.
/// * [expiresAt]
@BuiltValue()
abstract class InvocationTokenGrant
    implements Built<InvocationTokenGrant, InvocationTokenGrantBuilder> {
  /// Fresh invocation token with a new `jti`; it replaces the current one of the run.
  @BuiltValueField(wireName: r'invocation_token')
  String get invocationToken;

  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  InvocationTokenGrant._();

  factory InvocationTokenGrant([void updates(InvocationTokenGrantBuilder b)]) =
      _$InvocationTokenGrant;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InvocationTokenGrantBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<InvocationTokenGrant> get serializer =>
      _$InvocationTokenGrantSerializer();
}

class _$InvocationTokenGrantSerializer
    implements PrimitiveSerializer<InvocationTokenGrant> {
  @override
  final Iterable<Type> types = const [
    InvocationTokenGrant,
    _$InvocationTokenGrant,
  ];

  @override
  final String wireName = r'InvocationTokenGrant';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    InvocationTokenGrant object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'invocation_token';
    yield serializers.serialize(
      object.invocationToken,
      specifiedType: const FullType(String),
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
    InvocationTokenGrant object, {
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
    required InvocationTokenGrantBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'invocation_token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.invocationToken = valueDes;
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
  InvocationTokenGrant deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InvocationTokenGrantBuilder();
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

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'identity_link_complete.g.dart';

/// IdentityLinkComplete
///
/// Properties:
/// * [loginCode]
/// * [clientVerifier]
@BuiltValue()
abstract class IdentityLinkComplete
    implements Built<IdentityLinkComplete, IdentityLinkCompleteBuilder> {
  @BuiltValueField(wireName: r'login_code')
  String get loginCode;

  @BuiltValueField(wireName: r'client_verifier')
  String get clientVerifier;

  IdentityLinkComplete._();

  factory IdentityLinkComplete([void updates(IdentityLinkCompleteBuilder b)]) =
      _$IdentityLinkComplete;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(IdentityLinkCompleteBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<IdentityLinkComplete> get serializer =>
      _$IdentityLinkCompleteSerializer();
}

class _$IdentityLinkCompleteSerializer
    implements PrimitiveSerializer<IdentityLinkComplete> {
  @override
  final Iterable<Type> types = const [
    IdentityLinkComplete,
    _$IdentityLinkComplete,
  ];

  @override
  final String wireName = r'IdentityLinkComplete';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    IdentityLinkComplete object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'login_code';
    yield serializers.serialize(
      object.loginCode,
      specifiedType: const FullType(String),
    );
    yield r'client_verifier';
    yield serializers.serialize(
      object.clientVerifier,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    IdentityLinkComplete object, {
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
    required IdentityLinkCompleteBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'login_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.loginCode = valueDes;
          break;
        case r'client_verifier':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.clientVerifier = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  IdentityLinkComplete deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = IdentityLinkCompleteBuilder();
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

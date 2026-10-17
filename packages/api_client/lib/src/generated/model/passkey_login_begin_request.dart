//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'passkey_login_begin_request.g.dart';

/// PasskeyLoginBeginRequest
///
/// Properties:
/// * [loginName] - Optional; without it the server offers discoverable credentials.
@BuiltValue()
abstract class PasskeyLoginBeginRequest
    implements
        Built<PasskeyLoginBeginRequest, PasskeyLoginBeginRequestBuilder> {
  /// Optional; without it the server offers discoverable credentials.
  @BuiltValueField(wireName: r'login_name')
  String? get loginName;

  PasskeyLoginBeginRequest._();

  factory PasskeyLoginBeginRequest([
    void updates(PasskeyLoginBeginRequestBuilder b),
  ]) = _$PasskeyLoginBeginRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PasskeyLoginBeginRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PasskeyLoginBeginRequest> get serializer =>
      _$PasskeyLoginBeginRequestSerializer();
}

class _$PasskeyLoginBeginRequestSerializer
    implements PrimitiveSerializer<PasskeyLoginBeginRequest> {
  @override
  final Iterable<Type> types = const [
    PasskeyLoginBeginRequest,
    _$PasskeyLoginBeginRequest,
  ];

  @override
  final String wireName = r'PasskeyLoginBeginRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PasskeyLoginBeginRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.loginName != null) {
      yield r'login_name';
      yield serializers.serialize(
        object.loginName,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    PasskeyLoginBeginRequest object, {
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
    required PasskeyLoginBeginRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'login_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.loginName = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PasskeyLoginBeginRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PasskeyLoginBeginRequestBuilder();
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

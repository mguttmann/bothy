//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/identity_provider_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'auth_provider_info.g.dart';

/// AuthProviderInfo
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [kind]
/// * [name]
@BuiltValue()
abstract class AuthProviderInfo
    implements Built<AuthProviderInfo, AuthProviderInfoBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'kind')
  IdentityProviderKind get kind;
  // enum kindEnum {  oidc,  trusted_header,  local,  };

  @BuiltValueField(wireName: r'name')
  String get name;

  AuthProviderInfo._();

  factory AuthProviderInfo([void updates(AuthProviderInfoBuilder b)]) =
      _$AuthProviderInfo;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AuthProviderInfoBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AuthProviderInfo> get serializer =>
      _$AuthProviderInfoSerializer();
}

class _$AuthProviderInfoSerializer
    implements PrimitiveSerializer<AuthProviderInfo> {
  @override
  final Iterable<Type> types = const [AuthProviderInfo, _$AuthProviderInfo];

  @override
  final String wireName = r'AuthProviderInfo';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AuthProviderInfo object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(IdentityProviderKind),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AuthProviderInfo object, {
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
    required AuthProviderInfoBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(IdentityProviderKind),
          ) as IdentityProviderKind;
          result.kind = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AuthProviderInfo deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AuthProviderInfoBuilder();
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

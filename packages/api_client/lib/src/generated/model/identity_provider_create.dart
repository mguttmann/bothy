//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/identity_provider_kind.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'identity_provider_create.g.dart';

/// IdentityProviderCreate
///
/// Properties:
/// * [kind]
/// * [name]
/// * [config] - Free-form JSON object whose schema is named in the field description.
/// * [enabled]
/// * [allowProvisioning]
@BuiltValue()
abstract class IdentityProviderCreate
    implements Built<IdentityProviderCreate, IdentityProviderCreateBuilder> {
  @BuiltValueField(wireName: r'kind')
  IdentityProviderKind get kind;
  // enum kindEnum {  oidc,  trusted_header,  local,  };

  @BuiltValueField(wireName: r'name')
  String get name;

  /// Free-form JSON object whose schema is named in the field description.
  @BuiltValueField(wireName: r'config')
  BuiltMap<String, JsonObject?> get config;

  @BuiltValueField(wireName: r'enabled')
  bool? get enabled;

  @BuiltValueField(wireName: r'allow_provisioning')
  bool? get allowProvisioning;

  IdentityProviderCreate._();

  factory IdentityProviderCreate([
    void updates(IdentityProviderCreateBuilder b),
  ]) = _$IdentityProviderCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(IdentityProviderCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<IdentityProviderCreate> get serializer =>
      _$IdentityProviderCreateSerializer();
}

class _$IdentityProviderCreateSerializer
    implements PrimitiveSerializer<IdentityProviderCreate> {
  @override
  final Iterable<Type> types = const [
    IdentityProviderCreate,
    _$IdentityProviderCreate,
  ];

  @override
  final String wireName = r'IdentityProviderCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    IdentityProviderCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
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
    yield r'config';
    yield serializers.serialize(
      object.config,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
    if (object.enabled != null) {
      yield r'enabled';
      yield serializers.serialize(
        object.enabled,
        specifiedType: const FullType(bool),
      );
    }
    if (object.allowProvisioning != null) {
      yield r'allow_provisioning';
      yield serializers.serialize(
        object.allowProvisioning,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    IdentityProviderCreate object, {
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
    required IdentityProviderCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
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
        case r'config':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.config.replace(valueDes);
          break;
        case r'enabled':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.enabled = valueDes;
          break;
        case r'allow_provisioning':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.allowProvisioning = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  IdentityProviderCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = IdentityProviderCreateBuilder();
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

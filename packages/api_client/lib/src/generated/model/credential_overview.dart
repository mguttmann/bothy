//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/identity.dart';
import 'package:api_client/src/generated/model/passkey.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'credential_overview.g.dart';

/// CredentialOverview
///
/// Properties:
/// * [identities]
/// * [passkeys]
/// * [localPasswordSet]
/// * [totpEnrolled]
@BuiltValue()
abstract class CredentialOverview
    implements Built<CredentialOverview, CredentialOverviewBuilder> {
  @BuiltValueField(wireName: r'identities')
  BuiltList<Identity> get identities;

  @BuiltValueField(wireName: r'passkeys')
  BuiltList<Passkey> get passkeys;

  @BuiltValueField(wireName: r'local_password_set')
  bool get localPasswordSet;

  @BuiltValueField(wireName: r'totp_enrolled')
  bool get totpEnrolled;

  CredentialOverview._();

  factory CredentialOverview([void updates(CredentialOverviewBuilder b)]) =
      _$CredentialOverview;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CredentialOverviewBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CredentialOverview> get serializer =>
      _$CredentialOverviewSerializer();
}

class _$CredentialOverviewSerializer
    implements PrimitiveSerializer<CredentialOverview> {
  @override
  final Iterable<Type> types = const [CredentialOverview, _$CredentialOverview];

  @override
  final String wireName = r'CredentialOverview';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CredentialOverview object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'identities';
    yield serializers.serialize(
      object.identities,
      specifiedType: const FullType(BuiltList, [FullType(Identity)]),
    );
    yield r'passkeys';
    yield serializers.serialize(
      object.passkeys,
      specifiedType: const FullType(BuiltList, [FullType(Passkey)]),
    );
    yield r'local_password_set';
    yield serializers.serialize(
      object.localPasswordSet,
      specifiedType: const FullType(bool),
    );
    yield r'totp_enrolled';
    yield serializers.serialize(
      object.totpEnrolled,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CredentialOverview object, {
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
    required CredentialOverviewBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'identities':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Identity)]),
          ) as BuiltList<Identity>;
          result.identities.replace(valueDes);
          break;
        case r'passkeys':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Passkey)]),
          ) as BuiltList<Passkey>;
          result.passkeys.replace(valueDes);
          break;
        case r'local_password_set':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.localPasswordSet = valueDes;
          break;
        case r'totp_enrolled':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.totpEnrolled = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CredentialOverview deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CredentialOverviewBuilder();
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

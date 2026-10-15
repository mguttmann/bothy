//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'credential_holder_set.g.dart';

/// CredentialHolderSet
///
/// Properties:
/// * [credentialHolderId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [reason]
@BuiltValue()
abstract class CredentialHolderSet
    implements Built<CredentialHolderSet, CredentialHolderSetBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'credential_holder_id')
  String get credentialHolderId;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  CredentialHolderSet._();

  factory CredentialHolderSet([void updates(CredentialHolderSetBuilder b)]) =
      _$CredentialHolderSet;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CredentialHolderSetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CredentialHolderSet> get serializer =>
      _$CredentialHolderSetSerializer();
}

class _$CredentialHolderSetSerializer
    implements PrimitiveSerializer<CredentialHolderSet> {
  @override
  final Iterable<Type> types = const [
    CredentialHolderSet,
    _$CredentialHolderSet,
  ];

  @override
  final String wireName = r'CredentialHolderSet';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CredentialHolderSet object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'credential_holder_id';
    yield serializers.serialize(
      object.credentialHolderId,
      specifiedType: const FullType(String),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CredentialHolderSet object, {
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
    required CredentialHolderSetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'credential_holder_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.credentialHolderId = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CredentialHolderSet deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CredentialHolderSetBuilder();
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

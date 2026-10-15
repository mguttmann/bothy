//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'billing_mode_set.g.dart';

/// BillingModeSet
///
/// Properties:
/// * [billingMode]
/// * [credentialHolderId] - Required for `personal`.
/// * [reason]
@BuiltValue()
abstract class BillingModeSet
    implements Built<BillingModeSet, BillingModeSetBuilder> {
  @BuiltValueField(wireName: r'billing_mode')
  BillingMode get billingMode;
  // enum billingModeEnum {  personal,  organization,  };

  /// Required for `personal`.
  @BuiltValueField(wireName: r'credential_holder_id')
  String? get credentialHolderId;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  BillingModeSet._();

  factory BillingModeSet([void updates(BillingModeSetBuilder b)]) =
      _$BillingModeSet;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(BillingModeSetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<BillingModeSet> get serializer =>
      _$BillingModeSetSerializer();
}

class _$BillingModeSetSerializer
    implements PrimitiveSerializer<BillingModeSet> {
  @override
  final Iterable<Type> types = const [BillingModeSet, _$BillingModeSet];

  @override
  final String wireName = r'BillingModeSet';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    BillingModeSet object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'billing_mode';
    yield serializers.serialize(
      object.billingMode,
      specifiedType: const FullType(BillingMode),
    );
    if (object.credentialHolderId != null) {
      yield r'credential_holder_id';
      yield serializers.serialize(
        object.credentialHolderId,
        specifiedType: const FullType(String),
      );
    }
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    BillingModeSet object, {
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
    required BillingModeSetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'billing_mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BillingMode),
          ) as BillingMode;
          result.billingMode = valueDes;
          break;
        case r'credential_holder_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
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
  BillingModeSet deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = BillingModeSetBuilder();
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

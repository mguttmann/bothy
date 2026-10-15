//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'billing_mode_set_all.g.dart';

/// Switches several agents in one transaction, all or nothing (10, 9.1).
///
/// Properties:
/// * [billingMode]
/// * [credentialHolderId] - Required for `personal`.
/// * [agentIds] - Required for `personal`. Without it `organization` affects every agent not yet on `organization`.
/// * [reason]
@BuiltValue()
abstract class BillingModeSetAll
    implements Built<BillingModeSetAll, BillingModeSetAllBuilder> {
  @BuiltValueField(wireName: r'billing_mode')
  BillingMode get billingMode;
  // enum billingModeEnum {  personal,  organization,  };

  /// Required for `personal`.
  @BuiltValueField(wireName: r'credential_holder_id')
  String? get credentialHolderId;

  /// Required for `personal`. Without it `organization` affects every agent not yet on `organization`.
  @BuiltValueField(wireName: r'agent_ids')
  BuiltList<String>? get agentIds;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  BillingModeSetAll._();

  factory BillingModeSetAll([void updates(BillingModeSetAllBuilder b)]) =
      _$BillingModeSetAll;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(BillingModeSetAllBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<BillingModeSetAll> get serializer =>
      _$BillingModeSetAllSerializer();
}

class _$BillingModeSetAllSerializer
    implements PrimitiveSerializer<BillingModeSetAll> {
  @override
  final Iterable<Type> types = const [BillingModeSetAll, _$BillingModeSetAll];

  @override
  final String wireName = r'BillingModeSetAll';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    BillingModeSetAll object, {
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
    if (object.agentIds != null) {
      yield r'agent_ids';
      yield serializers.serialize(
        object.agentIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
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
    BillingModeSetAll object, {
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
    required BillingModeSetAllBuilder result,
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
        case r'agent_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.agentIds.replace(valueDes);
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
  BillingModeSetAll deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = BillingModeSetAllBuilder();
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

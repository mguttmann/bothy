//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'billing_mode_set_all_result.g.dart';

/// BillingModeSetAllResult
///
/// Properties:
/// * [changedAgentIds]
@BuiltValue()
abstract class BillingModeSetAllResult
    implements Built<BillingModeSetAllResult, BillingModeSetAllResultBuilder> {
  @BuiltValueField(wireName: r'changed_agent_ids')
  BuiltList<String> get changedAgentIds;

  BillingModeSetAllResult._();

  factory BillingModeSetAllResult([
    void updates(BillingModeSetAllResultBuilder b),
  ]) = _$BillingModeSetAllResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(BillingModeSetAllResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<BillingModeSetAllResult> get serializer =>
      _$BillingModeSetAllResultSerializer();
}

class _$BillingModeSetAllResultSerializer
    implements PrimitiveSerializer<BillingModeSetAllResult> {
  @override
  final Iterable<Type> types = const [
    BillingModeSetAllResult,
    _$BillingModeSetAllResult,
  ];

  @override
  final String wireName = r'BillingModeSetAllResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    BillingModeSetAllResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'changed_agent_ids';
    yield serializers.serialize(
      object.changedAgentIds,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    BillingModeSetAllResult object, {
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
    required BillingModeSetAllResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'changed_agent_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.changedAgentIds.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  BillingModeSetAllResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = BillingModeSetAllResultBuilder();
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

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gdpr_decision.g.dart';

/// GdprDecision
///
/// Properties:
/// * [decision]
/// * [reason] - Communicated to the person and recorded in the audit entry of the decision; with `extend_months` the justification of the extension.
/// * [extendMonths] - Extends `due_at` once by one or two calendar months (Art. 12(3) GDPR). Accepted only before the original `due_at`; a second extension or one after the deadline is `409 conflict` with reason `extension_not_allowed`.
@BuiltValue()
abstract class GdprDecision
    implements Built<GdprDecision, GdprDecisionBuilder> {
  @BuiltValueField(wireName: r'decision')
  GdprDecisionDecisionEnum get decision;
  // enum decisionEnum {  approve,  reject,  };

  /// Communicated to the person and recorded in the audit entry of the decision; with `extend_months` the justification of the extension.
  @BuiltValueField(wireName: r'reason')
  String get reason;

  /// Extends `due_at` once by one or two calendar months (Art. 12(3) GDPR). Accepted only before the original `due_at`; a second extension or one after the deadline is `409 conflict` with reason `extension_not_allowed`.
  @BuiltValueField(wireName: r'extend_months')
  GdprDecisionExtendMonthsEnum? get extendMonths;
  // enum extendMonthsEnum {  1,  2,  };

  GdprDecision._();

  factory GdprDecision([void updates(GdprDecisionBuilder b)]) = _$GdprDecision;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GdprDecisionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GdprDecision> get serializer => _$GdprDecisionSerializer();
}

class _$GdprDecisionSerializer implements PrimitiveSerializer<GdprDecision> {
  @override
  final Iterable<Type> types = const [GdprDecision, _$GdprDecision];

  @override
  final String wireName = r'GdprDecision';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GdprDecision object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'decision';
    yield serializers.serialize(
      object.decision,
      specifiedType: const FullType(GdprDecisionDecisionEnum),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
    if (object.extendMonths != null) {
      yield r'extend_months';
      yield serializers.serialize(
        object.extendMonths,
        specifiedType: const FullType(GdprDecisionExtendMonthsEnum),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    GdprDecision object, {
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
    required GdprDecisionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'decision':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(GdprDecisionDecisionEnum),
          ) as GdprDecisionDecisionEnum;
          result.decision = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        case r'extend_months':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(
              GdprDecisionExtendMonthsEnum,
            ),
          ) as GdprDecisionExtendMonthsEnum?;
          if (valueDes == null) continue;
          result.extendMonths = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GdprDecision deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GdprDecisionBuilder();
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

class GdprDecisionDecisionEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'approve')
  static const GdprDecisionDecisionEnum approve =
      _$gdprDecisionDecisionEnum_approve;
  @BuiltValueEnumConst(wireName: r'reject')
  static const GdprDecisionDecisionEnum reject =
      _$gdprDecisionDecisionEnum_reject;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const GdprDecisionDecisionEnum unknownDefaultOpenApi =
      _$gdprDecisionDecisionEnum_unknownDefaultOpenApi;

  static Serializer<GdprDecisionDecisionEnum> get serializer =>
      _$gdprDecisionDecisionEnumSerializer;

  const GdprDecisionDecisionEnum._(String name) : super(name);

  static BuiltSet<GdprDecisionDecisionEnum> get values =>
      _$gdprDecisionDecisionEnumValues;
  static GdprDecisionDecisionEnum valueOf(String name) =>
      _$gdprDecisionDecisionEnumValueOf(name);
}

/// Extends `due_at` once by one or two calendar months (Art. 12(3) GDPR). Accepted only before the original `due_at`; a second extension or one after the deadline is `409 conflict` with reason `extension_not_allowed`.
class GdprDecisionExtendMonthsEnum extends EnumClass {
  @BuiltValueEnumConst(wireNumber: 1)
  static const GdprDecisionExtendMonthsEnum number1 =
      _$gdprDecisionExtendMonthsEnum_number1;
  @BuiltValueEnumConst(wireNumber: 2)
  static const GdprDecisionExtendMonthsEnum number2 =
      _$gdprDecisionExtendMonthsEnum_number2;
  @BuiltValueEnumConst(wireNumber: 11184809, fallback: true)
  static const GdprDecisionExtendMonthsEnum unknownDefaultOpenApi =
      _$gdprDecisionExtendMonthsEnum_unknownDefaultOpenApi;

  static Serializer<GdprDecisionExtendMonthsEnum> get serializer =>
      _$gdprDecisionExtendMonthsEnumSerializer;

  const GdprDecisionExtendMonthsEnum._(String name) : super(name);

  static BuiltSet<GdprDecisionExtendMonthsEnum> get values =>
      _$gdprDecisionExtendMonthsEnumValues;
  static GdprDecisionExtendMonthsEnum valueOf(String name) =>
      _$gdprDecisionExtendMonthsEnumValueOf(name);
}

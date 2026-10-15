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
/// * [reason]
@BuiltValue()
abstract class GdprDecision
    implements Built<GdprDecision, GdprDecisionBuilder> {
  @BuiltValueField(wireName: r'decision')
  GdprDecisionDecisionEnum get decision;
  // enum decisionEnum {  approve,  reject,  };

  @BuiltValueField(wireName: r'reason')
  String get reason;

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

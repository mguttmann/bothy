//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/tool_class.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:api_client/src/generated/model/billing_evidence.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_start.g.dart';

/// RunStart
///
/// Properties:
/// * [invocationToken] - JWS from the delivered invocation (I3).
/// * [billingModeUsed]
/// * [toolClassUsed]
/// * [externalSessionRef] - Absent when the runtime creates the reference only during the run; then the first `reportAgentSession` creates the session row.
/// * [model]
/// * [billingEvidence]
@BuiltValue()
abstract class RunStart implements Built<RunStart, RunStartBuilder> {
  /// JWS from the delivered invocation (I3).
  @BuiltValueField(wireName: r'invocation_token')
  String get invocationToken;

  @BuiltValueField(wireName: r'billing_mode_used')
  BillingMode get billingModeUsed;
  // enum billingModeUsedEnum {  personal,  organization,  };

  @BuiltValueField(wireName: r'tool_class_used')
  ToolClass get toolClassUsed;
  // enum toolClassUsedEnum {  chat,  scoped,  host,  };

  /// Absent when the runtime creates the reference only during the run; then the first `reportAgentSession` creates the session row.
  @BuiltValueField(wireName: r'external_session_ref')
  String? get externalSessionRef;

  @BuiltValueField(wireName: r'model')
  String? get model;

  @BuiltValueField(wireName: r'billing_evidence')
  BillingEvidence? get billingEvidence;

  RunStart._();

  factory RunStart([void updates(RunStartBuilder b)]) = _$RunStart;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunStartBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunStart> get serializer => _$RunStartSerializer();
}

class _$RunStartSerializer implements PrimitiveSerializer<RunStart> {
  @override
  final Iterable<Type> types = const [RunStart, _$RunStart];

  @override
  final String wireName = r'RunStart';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunStart object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'invocation_token';
    yield serializers.serialize(
      object.invocationToken,
      specifiedType: const FullType(String),
    );
    yield r'billing_mode_used';
    yield serializers.serialize(
      object.billingModeUsed,
      specifiedType: const FullType(BillingMode),
    );
    yield r'tool_class_used';
    yield serializers.serialize(
      object.toolClassUsed,
      specifiedType: const FullType(ToolClass),
    );
    if (object.externalSessionRef != null) {
      yield r'external_session_ref';
      yield serializers.serialize(
        object.externalSessionRef,
        specifiedType: const FullType(String),
      );
    }
    if (object.model != null) {
      yield r'model';
      yield serializers.serialize(
        object.model,
        specifiedType: const FullType(String),
      );
    }
    if (object.billingEvidence != null) {
      yield r'billing_evidence';
      yield serializers.serialize(
        object.billingEvidence,
        specifiedType: const FullType(BillingEvidence),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunStart object, {
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
    required RunStartBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'invocation_token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.invocationToken = valueDes;
          break;
        case r'billing_mode_used':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BillingMode),
          ) as BillingMode;
          result.billingModeUsed = valueDes;
          break;
        case r'tool_class_used':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ToolClass),
          ) as ToolClass;
          result.toolClassUsed = valueDes;
          break;
        case r'external_session_ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.externalSessionRef = valueDes;
          break;
        case r'model':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.model = valueDes;
          break;
        case r'billing_evidence':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BillingEvidence),
          ) as BillingEvidence?;
          if (valueDes == null) continue;
          result.billingEvidence.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunStart deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunStartBuilder();
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

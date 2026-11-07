//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/tool_class.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake_check_result.g.dart';

/// Allowed; the configured values the runner compares with its own (`compare_actual`).
///
/// Properties:
/// * [toolClass]
/// * [billingMode]
@BuiltValue()
abstract class RunWakeCheckResult
    implements Built<RunWakeCheckResult, RunWakeCheckResultBuilder> {
  @BuiltValueField(wireName: r'tool_class')
  ToolClass get toolClass;
  // enum toolClassEnum {  chat,  scoped,  host,  };

  @BuiltValueField(wireName: r'billing_mode')
  BillingMode get billingMode;
  // enum billingModeEnum {  personal,  organization,  };

  RunWakeCheckResult._();

  factory RunWakeCheckResult([void updates(RunWakeCheckResultBuilder b)]) =
      _$RunWakeCheckResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunWakeCheckResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunWakeCheckResult> get serializer =>
      _$RunWakeCheckResultSerializer();
}

class _$RunWakeCheckResultSerializer
    implements PrimitiveSerializer<RunWakeCheckResult> {
  @override
  final Iterable<Type> types = const [RunWakeCheckResult, _$RunWakeCheckResult];

  @override
  final String wireName = r'RunWakeCheckResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunWakeCheckResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'tool_class';
    yield serializers.serialize(
      object.toolClass,
      specifiedType: const FullType(ToolClass),
    );
    yield r'billing_mode';
    yield serializers.serialize(
      object.billingMode,
      specifiedType: const FullType(BillingMode),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    RunWakeCheckResult object, {
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
    required RunWakeCheckResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'tool_class':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ToolClass),
          ) as ToolClass;
          result.toolClass = valueDes;
          break;
        case r'billing_mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BillingMode),
          ) as BillingMode;
          result.billingMode = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunWakeCheckResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunWakeCheckResultBuilder();
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

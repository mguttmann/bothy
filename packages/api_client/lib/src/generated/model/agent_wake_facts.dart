//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/run_wake_reason.dart';
import 'package:api_client/src/generated/model/run_wake_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_wake_facts.g.dart';

/// The wake-up of a wake run or a wake input; the runner writes its label, reason and result as the input (09 5.8.3).
///
/// Properties:
/// * [kind]
/// * [reason]
/// * [label]
/// * [resultText]
@BuiltValue()
abstract class AgentWakeFacts
    implements Built<AgentWakeFacts, AgentWakeFactsBuilder> {
  @BuiltValueField(wireName: r'kind')
  RunWakeKind get kind;
  // enum kindEnum {  at,  pr_checks,  command,  };

  @BuiltValueField(wireName: r'reason')
  RunWakeReason get reason;
  // enum reasonEnum {  agent,  recovery,  usage_limit,  };

  @BuiltValueField(wireName: r'label')
  String get label;

  @BuiltValueField(wireName: r'result_text')
  String get resultText;

  AgentWakeFacts._();

  factory AgentWakeFacts([void updates(AgentWakeFactsBuilder b)]) =
      _$AgentWakeFacts;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentWakeFactsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentWakeFacts> get serializer =>
      _$AgentWakeFactsSerializer();
}

class _$AgentWakeFactsSerializer
    implements PrimitiveSerializer<AgentWakeFacts> {
  @override
  final Iterable<Type> types = const [AgentWakeFacts, _$AgentWakeFacts];

  @override
  final String wireName = r'AgentWakeFacts';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentWakeFacts object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(RunWakeKind),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(RunWakeReason),
    );
    yield r'label';
    yield serializers.serialize(
      object.label,
      specifiedType: const FullType(String),
    );
    yield r'result_text';
    yield serializers.serialize(
      object.resultText,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentWakeFacts object, {
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
    required AgentWakeFactsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunWakeKind),
          ) as RunWakeKind;
          result.kind = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunWakeReason),
          ) as RunWakeReason;
          result.reason = valueDes;
          break;
        case r'label':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.label = valueDes;
          break;
        case r'result_text':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.resultText = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentWakeFacts deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentWakeFactsBuilder();
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

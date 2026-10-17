//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/operator_mode.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'operator_set.g.dart';

/// Operator assignment. `inherit` is only valid for channels.
///
/// Properties:
/// * [mode]
/// * [agentId] - Required for mode `agent`.
/// * [reason]
@BuiltValue()
abstract class OperatorSet implements Built<OperatorSet, OperatorSetBuilder> {
  @BuiltValueField(wireName: r'mode')
  OperatorMode get mode;
  // enum modeEnum {  inherit,  none,  agent,  };

  /// Required for mode `agent`.
  @BuiltValueField(wireName: r'agent_id')
  String? get agentId;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  OperatorSet._();

  factory OperatorSet([void updates(OperatorSetBuilder b)]) = _$OperatorSet;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(OperatorSetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<OperatorSet> get serializer => _$OperatorSetSerializer();
}

class _$OperatorSetSerializer implements PrimitiveSerializer<OperatorSet> {
  @override
  final Iterable<Type> types = const [OperatorSet, _$OperatorSet];

  @override
  final String wireName = r'OperatorSet';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    OperatorSet object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'mode';
    yield serializers.serialize(
      object.mode,
      specifiedType: const FullType(OperatorMode),
    );
    if (object.agentId != null) {
      yield r'agent_id';
      yield serializers.serialize(
        object.agentId,
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
    OperatorSet object, {
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
    required OperatorSetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(OperatorMode),
          ) as OperatorMode;
          result.mode = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.agentId = valueDes;
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
  OperatorSet deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = OperatorSetBuilder();
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

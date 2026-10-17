//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/operator_mode.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel_operator.g.dart';

/// ChannelOperator
///
/// Properties:
/// * [mode]
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [effectiveAgentId] - Operator after resolving inheritance; absent means no operator.
@BuiltValue()
abstract class ChannelOperator
    implements Built<ChannelOperator, ChannelOperatorBuilder> {
  @BuiltValueField(wireName: r'mode')
  OperatorMode get mode;
  // enum modeEnum {  inherit,  none,  agent,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String? get agentId;

  /// Operator after resolving inheritance; absent means no operator.
  @BuiltValueField(wireName: r'effective_agent_id')
  String? get effectiveAgentId;

  ChannelOperator._();

  factory ChannelOperator([void updates(ChannelOperatorBuilder b)]) =
      _$ChannelOperator;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ChannelOperatorBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ChannelOperator> get serializer =>
      _$ChannelOperatorSerializer();
}

class _$ChannelOperatorSerializer
    implements PrimitiveSerializer<ChannelOperator> {
  @override
  final Iterable<Type> types = const [ChannelOperator, _$ChannelOperator];

  @override
  final String wireName = r'ChannelOperator';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ChannelOperator object, {
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
    if (object.effectiveAgentId != null) {
      yield r'effective_agent_id';
      yield serializers.serialize(
        object.effectiveAgentId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ChannelOperator object, {
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
    required ChannelOperatorBuilder result,
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
        case r'effective_agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.effectiveAgentId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ChannelOperator deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ChannelOperatorBuilder();
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

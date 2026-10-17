//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/status_component_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'status_component_input.g.dart';

/// StatusComponentInput
///
/// Properties:
/// * [name]
/// * [description]
/// * [state]
/// * [healthSource]
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [afterComponentId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class StatusComponentInput
    implements Built<StatusComponentInput, StatusComponentInputBuilder> {
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'description')
  String? get description;

  @BuiltValueField(wireName: r'state')
  StatusComponentState? get state;
  // enum stateEnum {  operational,  degraded,  partial_outage,  major_outage,  maintenance,  };

  @BuiltValueField(wireName: r'health_source')
  String? get healthSource;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String? get agentId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'after_component_id')
  String? get afterComponentId;

  StatusComponentInput._();

  factory StatusComponentInput([void updates(StatusComponentInputBuilder b)]) =
      _$StatusComponentInput;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatusComponentInputBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatusComponentInput> get serializer =>
      _$StatusComponentInputSerializer();
}

class _$StatusComponentInputSerializer
    implements PrimitiveSerializer<StatusComponentInput> {
  @override
  final Iterable<Type> types = const [
    StatusComponentInput,
    _$StatusComponentInput,
  ];

  @override
  final String wireName = r'StatusComponentInput';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatusComponentInput object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    if (object.description != null) {
      yield r'description';
      yield serializers.serialize(
        object.description,
        specifiedType: const FullType(String),
      );
    }
    if (object.state != null) {
      yield r'state';
      yield serializers.serialize(
        object.state,
        specifiedType: const FullType(StatusComponentState),
      );
    }
    if (object.healthSource != null) {
      yield r'health_source';
      yield serializers.serialize(
        object.healthSource,
        specifiedType: const FullType(String),
      );
    }
    if (object.agentId != null) {
      yield r'agent_id';
      yield serializers.serialize(
        object.agentId,
        specifiedType: const FullType(String),
      );
    }
    if (object.afterComponentId != null) {
      yield r'after_component_id';
      yield serializers.serialize(
        object.afterComponentId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    StatusComponentInput object, {
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
    required StatusComponentInputBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'description':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.description = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(StatusComponentState),
          ) as StatusComponentState?;
          if (valueDes == null) continue;
          result.state = valueDes;
          break;
        case r'health_source':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.healthSource = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.agentId = valueDes;
          break;
        case r'after_component_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.afterComponentId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatusComponentInput deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatusComponentInputBuilder();
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

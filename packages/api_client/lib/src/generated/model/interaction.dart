//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/interaction_response.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'interaction.g.dart';

/// Interaction
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [componentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [customId]
/// * [messageId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [principalId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [clientInteractionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [values] - Free-form JSON object whose schema is named in the field description.
/// * [state]
/// * [response]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class Interaction implements Built<Interaction, InteractionBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'component_id')
  String get componentId;

  @BuiltValueField(wireName: r'custom_id')
  String? get customId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'message_id')
  String get messageId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String? get threadId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String? get runId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'principal_id')
  String get principalId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'client_interaction_id')
  String get clientInteractionId;

  /// Free-form JSON object whose schema is named in the field description.
  @BuiltValueField(wireName: r'values')
  BuiltMap<String, JsonObject?> get values;

  @BuiltValueField(wireName: r'state')
  InteractionStateEnum get state;
  // enum stateEnum {  received,  dispatched,  acknowledged,  failed,  rejected,  };

  @BuiltValueField(wireName: r'response')
  InteractionResponse? get response;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  Interaction._();

  factory Interaction([void updates(InteractionBuilder b)]) = _$Interaction;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InteractionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Interaction> get serializer => _$InteractionSerializer();
}

class _$InteractionSerializer implements PrimitiveSerializer<Interaction> {
  @override
  final Iterable<Type> types = const [Interaction, _$Interaction];

  @override
  final String wireName = r'Interaction';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Interaction object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'component_id';
    yield serializers.serialize(
      object.componentId,
      specifiedType: const FullType(String),
    );
    if (object.customId != null) {
      yield r'custom_id';
      yield serializers.serialize(
        object.customId,
        specifiedType: const FullType(String),
      );
    }
    yield r'message_id';
    yield serializers.serialize(
      object.messageId,
      specifiedType: const FullType(String),
    );
    if (object.threadId != null) {
      yield r'thread_id';
      yield serializers.serialize(
        object.threadId,
        specifiedType: const FullType(String),
      );
    }
    if (object.runId != null) {
      yield r'run_id';
      yield serializers.serialize(
        object.runId,
        specifiedType: const FullType(String),
      );
    }
    yield r'principal_id';
    yield serializers.serialize(
      object.principalId,
      specifiedType: const FullType(String),
    );
    yield r'client_interaction_id';
    yield serializers.serialize(
      object.clientInteractionId,
      specifiedType: const FullType(String),
    );
    yield r'values';
    yield serializers.serialize(
      object.values,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(InteractionStateEnum),
    );
    if (object.response != null) {
      yield r'response';
      yield serializers.serialize(
        object.response,
        specifiedType: const FullType(InteractionResponse),
      );
    }
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Interaction object, {
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
    required InteractionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'component_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.componentId = valueDes;
          break;
        case r'custom_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.customId = valueDes;
          break;
        case r'message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.messageId = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.threadId = valueDes;
          break;
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.runId = valueDes;
          break;
        case r'principal_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.principalId = valueDes;
          break;
        case r'client_interaction_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.clientInteractionId = valueDes;
          break;
        case r'values':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.values.replace(valueDes);
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(InteractionStateEnum),
          ) as InteractionStateEnum;
          result.state = valueDes;
          break;
        case r'response':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(InteractionResponse),
          ) as InteractionResponse?;
          if (valueDes == null) continue;
          result.response.replace(valueDes);
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Interaction deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InteractionBuilder();
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

class InteractionStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'received')
  static const InteractionStateEnum received = _$interactionStateEnum_received;
  @BuiltValueEnumConst(wireName: r'dispatched')
  static const InteractionStateEnum dispatched =
      _$interactionStateEnum_dispatched;
  @BuiltValueEnumConst(wireName: r'acknowledged')
  static const InteractionStateEnum acknowledged =
      _$interactionStateEnum_acknowledged;
  @BuiltValueEnumConst(wireName: r'failed')
  static const InteractionStateEnum failed = _$interactionStateEnum_failed;
  @BuiltValueEnumConst(wireName: r'rejected')
  static const InteractionStateEnum rejected = _$interactionStateEnum_rejected;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const InteractionStateEnum unknownDefaultOpenApi =
      _$interactionStateEnum_unknownDefaultOpenApi;

  static Serializer<InteractionStateEnum> get serializer =>
      _$interactionStateEnumSerializer;

  const InteractionStateEnum._(String name) : super(name);

  static BuiltSet<InteractionStateEnum> get values =>
      _$interactionStateEnumValues;
  static InteractionStateEnum valueOf(String name) =>
      _$interactionStateEnumValueOf(name);
}

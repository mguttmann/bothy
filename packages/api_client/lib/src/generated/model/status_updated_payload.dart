//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/status_component_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'status_updated_payload.g.dart';

/// StatusUpdatedPayload
///
/// Properties:
/// * [overallState]
/// * [changedComponentIds]
/// * [changedIncidentIds]
@BuiltValue()
abstract class StatusUpdatedPayload
    implements Built<StatusUpdatedPayload, StatusUpdatedPayloadBuilder> {
  @BuiltValueField(wireName: r'overall_state')
  StatusComponentState get overallState;
  // enum overallStateEnum {  operational,  degraded,  partial_outage,  major_outage,  maintenance,  };

  @BuiltValueField(wireName: r'changed_component_ids')
  BuiltList<String>? get changedComponentIds;

  @BuiltValueField(wireName: r'changed_incident_ids')
  BuiltList<String>? get changedIncidentIds;

  StatusUpdatedPayload._();

  factory StatusUpdatedPayload([void updates(StatusUpdatedPayloadBuilder b)]) =
      _$StatusUpdatedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatusUpdatedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatusUpdatedPayload> get serializer =>
      _$StatusUpdatedPayloadSerializer();
}

class _$StatusUpdatedPayloadSerializer
    implements PrimitiveSerializer<StatusUpdatedPayload> {
  @override
  final Iterable<Type> types = const [
    StatusUpdatedPayload,
    _$StatusUpdatedPayload,
  ];

  @override
  final String wireName = r'StatusUpdatedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatusUpdatedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'overall_state';
    yield serializers.serialize(
      object.overallState,
      specifiedType: const FullType(StatusComponentState),
    );
    if (object.changedComponentIds != null) {
      yield r'changed_component_ids';
      yield serializers.serialize(
        object.changedComponentIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.changedIncidentIds != null) {
      yield r'changed_incident_ids';
      yield serializers.serialize(
        object.changedIncidentIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    StatusUpdatedPayload object, {
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
    required StatusUpdatedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'overall_state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatusComponentState),
          ) as StatusComponentState;
          result.overallState = valueDes;
          break;
        case r'changed_component_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.changedComponentIds.replace(valueDes);
          break;
        case r'changed_incident_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.changedIncidentIds.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatusUpdatedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatusUpdatedPayloadBuilder();
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

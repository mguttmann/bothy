//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/status_incident_state.dart';
import 'package:api_client/src/generated/model/status_incident_update.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/status_incident_impact.dart';
import 'package:api_client/src/generated/model/status_incident_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'status_incident.g.dart';

/// StatusIncident
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [title]
/// * [kind]
/// * [impact]
/// * [state]
/// * [startedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [resolvedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [componentIds]
/// * [updates]
@BuiltValue()
abstract class StatusIncident
    implements Built<StatusIncident, StatusIncidentBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'title')
  String get title;

  @BuiltValueField(wireName: r'kind')
  StatusIncidentKind get kind;
  // enum kindEnum {  incident,  maintenance,  };

  @BuiltValueField(wireName: r'impact')
  StatusIncidentImpact get impact;
  // enum impactEnum {  none,  minor,  major,  critical,  };

  @BuiltValueField(wireName: r'state')
  StatusIncidentState get state;
  // enum stateEnum {  scheduled,  investigating,  identified,  monitoring,  resolved,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'started_at')
  DateTime get startedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'resolved_at')
  DateTime? get resolvedAt;

  @BuiltValueField(wireName: r'component_ids')
  BuiltList<String> get componentIds;

  @BuiltValueField(wireName: r'updates')
  BuiltList<StatusIncidentUpdate> get updates;

  StatusIncident._();

  factory StatusIncident([void updates(StatusIncidentBuilder b)]) =
      _$StatusIncident;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatusIncidentBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatusIncident> get serializer =>
      _$StatusIncidentSerializer();
}

class _$StatusIncidentSerializer
    implements PrimitiveSerializer<StatusIncident> {
  @override
  final Iterable<Type> types = const [StatusIncident, _$StatusIncident];

  @override
  final String wireName = r'StatusIncident';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatusIncident object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'title';
    yield serializers.serialize(
      object.title,
      specifiedType: const FullType(String),
    );
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(StatusIncidentKind),
    );
    yield r'impact';
    yield serializers.serialize(
      object.impact,
      specifiedType: const FullType(StatusIncidentImpact),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(StatusIncidentState),
    );
    yield r'started_at';
    yield serializers.serialize(
      object.startedAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.resolvedAt != null) {
      yield r'resolved_at';
      yield serializers.serialize(
        object.resolvedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'component_ids';
    yield serializers.serialize(
      object.componentIds,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    yield r'updates';
    yield serializers.serialize(
      object.updates,
      specifiedType: const FullType(BuiltList, [
        FullType(StatusIncidentUpdate),
      ]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatusIncident object, {
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
    required StatusIncidentBuilder result,
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
        case r'title':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.title = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatusIncidentKind),
          ) as StatusIncidentKind;
          result.kind = valueDes;
          break;
        case r'impact':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatusIncidentImpact),
          ) as StatusIncidentImpact;
          result.impact = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatusIncidentState),
          ) as StatusIncidentState;
          result.state = valueDes;
          break;
        case r'started_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.startedAt = valueDes;
          break;
        case r'resolved_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.resolvedAt = valueDes;
          break;
        case r'component_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.componentIds.replace(valueDes);
          break;
        case r'updates':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(StatusIncidentUpdate),
            ]),
          ) as BuiltList<StatusIncidentUpdate>;
          result.updates.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatusIncident deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatusIncidentBuilder();
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

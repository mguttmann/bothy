//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/status_incident_state.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/status_incident_impact.dart';
import 'package:api_client/src/generated/model/status_incident_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'status_incident_create.g.dart';

/// StatusIncidentCreate
///
/// Properties:
/// * [title]
/// * [kind]
/// * [impact]
/// * [state]
/// * [startedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [componentIds]
/// * [bodyMd]
@BuiltValue()
abstract class StatusIncidentCreate
    implements Built<StatusIncidentCreate, StatusIncidentCreateBuilder> {
  @BuiltValueField(wireName: r'title')
  String get title;

  @BuiltValueField(wireName: r'kind')
  StatusIncidentKind get kind;
  // enum kindEnum {  incident,  maintenance,  };

  @BuiltValueField(wireName: r'impact')
  StatusIncidentImpact get impact;
  // enum impactEnum {  none,  minor,  major,  critical,  };

  @BuiltValueField(wireName: r'state')
  StatusIncidentState? get state;
  // enum stateEnum {  scheduled,  investigating,  identified,  monitoring,  resolved,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'started_at')
  DateTime get startedAt;

  @BuiltValueField(wireName: r'component_ids')
  BuiltList<String> get componentIds;

  @BuiltValueField(wireName: r'body_md')
  String get bodyMd;

  StatusIncidentCreate._();

  factory StatusIncidentCreate([void updates(StatusIncidentCreateBuilder b)]) =
      _$StatusIncidentCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatusIncidentCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatusIncidentCreate> get serializer =>
      _$StatusIncidentCreateSerializer();
}

class _$StatusIncidentCreateSerializer
    implements PrimitiveSerializer<StatusIncidentCreate> {
  @override
  final Iterable<Type> types = const [
    StatusIncidentCreate,
    _$StatusIncidentCreate,
  ];

  @override
  final String wireName = r'StatusIncidentCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatusIncidentCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
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
    if (object.state != null) {
      yield r'state';
      yield serializers.serialize(
        object.state,
        specifiedType: const FullType(StatusIncidentState),
      );
    }
    yield r'started_at';
    yield serializers.serialize(
      object.startedAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'component_ids';
    yield serializers.serialize(
      object.componentIds,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    yield r'body_md';
    yield serializers.serialize(
      object.bodyMd,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatusIncidentCreate object, {
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
    required StatusIncidentCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
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
            specifiedType: const FullType.nullable(StatusIncidentState),
          ) as StatusIncidentState?;
          if (valueDes == null) continue;
          result.state = valueDes;
          break;
        case r'started_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.startedAt = valueDes;
          break;
        case r'component_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.componentIds.replace(valueDes);
          break;
        case r'body_md':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.bodyMd = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatusIncidentCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatusIncidentCreateBuilder();
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

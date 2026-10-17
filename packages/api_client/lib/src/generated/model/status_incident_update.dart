//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/status_incident_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'status_incident_update.g.dart';

/// StatusIncidentUpdate
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state]
/// * [bodyMd]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class StatusIncidentUpdate
    implements Built<StatusIncidentUpdate, StatusIncidentUpdateBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'state')
  StatusIncidentState get state;
  // enum stateEnum {  scheduled,  investigating,  identified,  monitoring,  resolved,  };

  @BuiltValueField(wireName: r'body_md')
  String get bodyMd;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  StatusIncidentUpdate._();

  factory StatusIncidentUpdate([void updates(StatusIncidentUpdateBuilder b)]) =
      _$StatusIncidentUpdate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatusIncidentUpdateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatusIncidentUpdate> get serializer =>
      _$StatusIncidentUpdateSerializer();
}

class _$StatusIncidentUpdateSerializer
    implements PrimitiveSerializer<StatusIncidentUpdate> {
  @override
  final Iterable<Type> types = const [
    StatusIncidentUpdate,
    _$StatusIncidentUpdate,
  ];

  @override
  final String wireName = r'StatusIncidentUpdate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatusIncidentUpdate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(StatusIncidentState),
    );
    yield r'body_md';
    yield serializers.serialize(
      object.bodyMd,
      specifiedType: const FullType(String),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatusIncidentUpdate object, {
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
    required StatusIncidentUpdateBuilder result,
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
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatusIncidentState),
          ) as StatusIncidentState;
          result.state = valueDes;
          break;
        case r'body_md':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.bodyMd = valueDes;
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
  StatusIncidentUpdate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatusIncidentUpdateBuilder();
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

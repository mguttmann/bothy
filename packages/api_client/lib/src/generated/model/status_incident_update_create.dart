//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/status_incident_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'status_incident_update_create.g.dart';

/// StatusIncidentUpdateCreate
///
/// Properties:
/// * [state]
/// * [bodyMd]
@BuiltValue()
abstract class StatusIncidentUpdateCreate
    implements
        Built<StatusIncidentUpdateCreate, StatusIncidentUpdateCreateBuilder> {
  @BuiltValueField(wireName: r'state')
  StatusIncidentState get state;
  // enum stateEnum {  scheduled,  investigating,  identified,  monitoring,  resolved,  };

  @BuiltValueField(wireName: r'body_md')
  String get bodyMd;

  StatusIncidentUpdateCreate._();

  factory StatusIncidentUpdateCreate([
    void updates(StatusIncidentUpdateCreateBuilder b),
  ]) = _$StatusIncidentUpdateCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatusIncidentUpdateCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatusIncidentUpdateCreate> get serializer =>
      _$StatusIncidentUpdateCreateSerializer();
}

class _$StatusIncidentUpdateCreateSerializer
    implements PrimitiveSerializer<StatusIncidentUpdateCreate> {
  @override
  final Iterable<Type> types = const [
    StatusIncidentUpdateCreate,
    _$StatusIncidentUpdateCreate,
  ];

  @override
  final String wireName = r'StatusIncidentUpdateCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatusIncidentUpdateCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
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
  }

  @override
  Object serialize(
    Serializers serializers,
    StatusIncidentUpdateCreate object, {
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
    required StatusIncidentUpdateCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
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
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatusIncidentUpdateCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatusIncidentUpdateCreateBuilder();
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

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/status_component.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/status_component_state.dart';
import 'package:api_client/src/generated/model/status_incident.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'status_page.g.dart';

/// StatusPage
///
/// Properties:
/// * [overallState]
/// * [components]
/// * [incidents] - Open incidents and recent ones within `status.history_window`.
@BuiltValue()
abstract class StatusPage implements Built<StatusPage, StatusPageBuilder> {
  @BuiltValueField(wireName: r'overall_state')
  StatusComponentState get overallState;
  // enum overallStateEnum {  operational,  degraded,  partial_outage,  major_outage,  maintenance,  };

  @BuiltValueField(wireName: r'components')
  BuiltList<StatusComponent> get components;

  /// Open incidents and recent ones within `status.history_window`.
  @BuiltValueField(wireName: r'incidents')
  BuiltList<StatusIncident> get incidents;

  StatusPage._();

  factory StatusPage([void updates(StatusPageBuilder b)]) = _$StatusPage;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatusPageBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatusPage> get serializer => _$StatusPageSerializer();
}

class _$StatusPageSerializer implements PrimitiveSerializer<StatusPage> {
  @override
  final Iterable<Type> types = const [StatusPage, _$StatusPage];

  @override
  final String wireName = r'StatusPage';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatusPage object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'overall_state';
    yield serializers.serialize(
      object.overallState,
      specifiedType: const FullType(StatusComponentState),
    );
    yield r'components';
    yield serializers.serialize(
      object.components,
      specifiedType: const FullType(BuiltList, [FullType(StatusComponent)]),
    );
    yield r'incidents';
    yield serializers.serialize(
      object.incidents,
      specifiedType: const FullType(BuiltList, [FullType(StatusIncident)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatusPage object, {
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
    required StatusPageBuilder result,
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
        case r'components':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(StatusComponent),
            ]),
          ) as BuiltList<StatusComponent>;
          result.components.replace(valueDes);
          break;
        case r'incidents':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(StatusIncident),
            ]),
          ) as BuiltList<StatusIncident>;
          result.incidents.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatusPage deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatusPageBuilder();
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

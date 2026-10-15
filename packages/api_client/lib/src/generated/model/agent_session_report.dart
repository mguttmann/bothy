//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_session_report.g.dart';

/// AgentSessionReport
///
/// Properties:
/// * [externalSessionRef]
/// * [model]
@BuiltValue()
abstract class AgentSessionReport
    implements Built<AgentSessionReport, AgentSessionReportBuilder> {
  @BuiltValueField(wireName: r'external_session_ref')
  String get externalSessionRef;

  @BuiltValueField(wireName: r'model')
  String? get model;

  AgentSessionReport._();

  factory AgentSessionReport([void updates(AgentSessionReportBuilder b)]) =
      _$AgentSessionReport;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentSessionReportBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentSessionReport> get serializer =>
      _$AgentSessionReportSerializer();
}

class _$AgentSessionReportSerializer
    implements PrimitiveSerializer<AgentSessionReport> {
  @override
  final Iterable<Type> types = const [AgentSessionReport, _$AgentSessionReport];

  @override
  final String wireName = r'AgentSessionReport';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentSessionReport object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'external_session_ref';
    yield serializers.serialize(
      object.externalSessionRef,
      specifiedType: const FullType(String),
    );
    if (object.model != null) {
      yield r'model';
      yield serializers.serialize(
        object.model,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentSessionReport object, {
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
    required AgentSessionReportBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'external_session_ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.externalSessionRef = valueDes;
          break;
        case r'model':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.model = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentSessionReport deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentSessionReportBuilder();
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

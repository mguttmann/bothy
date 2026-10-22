//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'csp_report_legacy.g.dart';

/// Legacy report of the `report-uri` directive (CSP Level 2). The server reads only `effective-directive` (else `violated-directive`), `blocked-uri` and `disposition` of `csp-report`.
///
/// Properties:
/// * [cspReport] - Free-form JSON object whose schema is named in the field description.
@BuiltValue()
abstract class CspReportLegacy
    implements Built<CspReportLegacy, CspReportLegacyBuilder> {
  /// Free-form JSON object whose schema is named in the field description.
  @BuiltValueField(wireName: r'csp-report')
  BuiltMap<String, JsonObject?> get cspReport;

  CspReportLegacy._();

  factory CspReportLegacy([void updates(CspReportLegacyBuilder b)]) =
      _$CspReportLegacy;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CspReportLegacyBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CspReportLegacy> get serializer =>
      _$CspReportLegacySerializer();
}

class _$CspReportLegacySerializer
    implements PrimitiveSerializer<CspReportLegacy> {
  @override
  final Iterable<Type> types = const [CspReportLegacy, _$CspReportLegacy];

  @override
  final String wireName = r'CspReportLegacy';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CspReportLegacy object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'csp-report';
    yield serializers.serialize(
      object.cspReport,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CspReportLegacy object, {
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
    required CspReportLegacyBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'csp-report':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.cspReport.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CspReportLegacy deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CspReportLegacyBuilder();
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

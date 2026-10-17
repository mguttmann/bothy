//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'status_incident_impact.g.dart';

class StatusIncidentImpact extends EnumClass {
  @BuiltValueEnumConst(wireName: r'none')
  static const StatusIncidentImpact none = _$none;
  @BuiltValueEnumConst(wireName: r'minor')
  static const StatusIncidentImpact minor = _$minor;
  @BuiltValueEnumConst(wireName: r'major')
  static const StatusIncidentImpact major = _$major;
  @BuiltValueEnumConst(wireName: r'critical')
  static const StatusIncidentImpact critical = _$critical;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatusIncidentImpact unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<StatusIncidentImpact> get serializer =>
      _$statusIncidentImpactSerializer;

  const StatusIncidentImpact._(String name) : super(name);

  static BuiltSet<StatusIncidentImpact> get values => _$values;
  static StatusIncidentImpact valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class StatusIncidentImpactMixin = Object
    with _$StatusIncidentImpactMixin;

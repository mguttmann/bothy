//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'status_incident_kind.g.dart';

class StatusIncidentKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'incident')
  static const StatusIncidentKind incident = _$incident;
  @BuiltValueEnumConst(wireName: r'maintenance')
  static const StatusIncidentKind maintenance = _$maintenance;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatusIncidentKind unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<StatusIncidentKind> get serializer =>
      _$statusIncidentKindSerializer;

  const StatusIncidentKind._(String name) : super(name);

  static BuiltSet<StatusIncidentKind> get values => _$values;
  static StatusIncidentKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class StatusIncidentKindMixin = Object with _$StatusIncidentKindMixin;

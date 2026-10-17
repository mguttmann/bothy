//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'status_incident_state.g.dart';

class StatusIncidentState extends EnumClass {
  @BuiltValueEnumConst(wireName: r'scheduled')
  static const StatusIncidentState scheduled = _$scheduled;
  @BuiltValueEnumConst(wireName: r'investigating')
  static const StatusIncidentState investigating = _$investigating;
  @BuiltValueEnumConst(wireName: r'identified')
  static const StatusIncidentState identified = _$identified;
  @BuiltValueEnumConst(wireName: r'monitoring')
  static const StatusIncidentState monitoring = _$monitoring;
  @BuiltValueEnumConst(wireName: r'resolved')
  static const StatusIncidentState resolved = _$resolved;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatusIncidentState unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<StatusIncidentState> get serializer =>
      _$statusIncidentStateSerializer;

  const StatusIncidentState._(String name) : super(name);

  static BuiltSet<StatusIncidentState> get values => _$values;
  static StatusIncidentState valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class StatusIncidentStateMixin = Object
    with _$StatusIncidentStateMixin;

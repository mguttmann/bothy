//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'status_component_state.g.dart';

class StatusComponentState extends EnumClass {
  @BuiltValueEnumConst(wireName: r'operational')
  static const StatusComponentState operational = _$operational;
  @BuiltValueEnumConst(wireName: r'degraded')
  static const StatusComponentState degraded = _$degraded;
  @BuiltValueEnumConst(wireName: r'partial_outage')
  static const StatusComponentState partialOutage = _$partialOutage;
  @BuiltValueEnumConst(wireName: r'major_outage')
  static const StatusComponentState majorOutage = _$majorOutage;
  @BuiltValueEnumConst(wireName: r'maintenance')
  static const StatusComponentState maintenance = _$maintenance;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatusComponentState unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<StatusComponentState> get serializer =>
      _$statusComponentStateSerializer;

  const StatusComponentState._(String name) : super(name);

  static BuiltSet<StatusComponentState> get values => _$values;
  static StatusComponentState valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class StatusComponentStateMixin = Object
    with _$StatusComponentStateMixin;

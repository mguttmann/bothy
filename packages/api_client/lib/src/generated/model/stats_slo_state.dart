//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_slo_state.g.dart';

/// Traffic light of one SLO (docs/spec/25-visualization.md, section 6.4).
class StatsSloState extends EnumClass {
  @BuiltValueEnumConst(wireName: r'green')
  static const StatsSloState green = _$green;
  @BuiltValueEnumConst(wireName: r'amber')
  static const StatsSloState amber = _$amber;
  @BuiltValueEnumConst(wireName: r'red')
  static const StatsSloState red = _$red;
  @BuiltValueEnumConst(wireName: r'insufficient_data')
  static const StatsSloState insufficientData = _$insufficientData;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatsSloState unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<StatsSloState> get serializer => _$statsSloStateSerializer;

  const StatsSloState._(String name) : super(name);

  static BuiltSet<StatsSloState> get values => _$values;
  static StatsSloState valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class StatsSloStateMixin = Object with _$StatsSloStateMixin;

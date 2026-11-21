//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'team_pause_reason.g.dart';

/// Why member inputs are paused (09 3.9.6, 3.9.7); an owner message lifts every pause.
class TeamPauseReason extends EnumClass {
  @BuiltValueEnumConst(wireName: r'turn_budget')
  static const TeamPauseReason turnBudget = _$turnBudget;
  @BuiltValueEnumConst(wireName: r'pingpong')
  static const TeamPauseReason pingpong = _$pingpong;
  @BuiltValueEnumConst(wireName: r'stopped')
  static const TeamPauseReason stopped = _$stopped;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const TeamPauseReason unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<TeamPauseReason> get serializer =>
      _$teamPauseReasonSerializer;

  const TeamPauseReason._(String name) : super(name);

  static BuiltSet<TeamPauseReason> get values => _$values;
  static TeamPauseReason valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class TeamPauseReasonMixin = Object with _$TeamPauseReasonMixin;

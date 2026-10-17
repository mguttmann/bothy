//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_granularity.g.dart';

/// Size of a statistics bucket.
class StatsGranularity extends EnumClass {
  @BuiltValueEnumConst(wireName: r'hour')
  static const StatsGranularity hour = _$hour;
  @BuiltValueEnumConst(wireName: r'day')
  static const StatsGranularity day = _$day;
  @BuiltValueEnumConst(wireName: r'week')
  static const StatsGranularity week = _$week;
  @BuiltValueEnumConst(wireName: r'month')
  static const StatsGranularity month = _$month;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatsGranularity unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<StatsGranularity> get serializer =>
      _$statsGranularitySerializer;

  const StatsGranularity._(String name) : super(name);

  static BuiltSet<StatsGranularity> get values => _$values;
  static StatsGranularity valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class StatsGranularityMixin = Object with _$StatsGranularityMixin;

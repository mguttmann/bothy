//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake_kind.g.dart';

/// What makes a wake-up fire (09 5.8.1); `at` the server job `runs.wake_due`, `pr_checks` and `command` the runner after `run.wake.check`.
class RunWakeKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'at')
  static const RunWakeKind at = _$at;
  @BuiltValueEnumConst(wireName: r'pr_checks')
  static const RunWakeKind prChecks = _$prChecks;
  @BuiltValueEnumConst(wireName: r'command')
  static const RunWakeKind command = _$command;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunWakeKind unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<RunWakeKind> get serializer => _$runWakeKindSerializer;

  const RunWakeKind._(String name) : super(name);

  static BuiltSet<RunWakeKind> get values => _$values;
  static RunWakeKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class RunWakeKindMixin = Object with _$RunWakeKindMixin;

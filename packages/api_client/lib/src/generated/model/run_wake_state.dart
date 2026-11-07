//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake_state.g.dart';

/// State of a wake-up (09 5.8.1); every state other than `pending` is an end, and every end other than `fired` posts a note in the thread (09 5.8.4).
class RunWakeState extends EnumClass {
  @BuiltValueEnumConst(wireName: r'pending')
  static const RunWakeState pending = _$pending;
  @BuiltValueEnumConst(wireName: r'fired')
  static const RunWakeState fired = _$fired;
  @BuiltValueEnumConst(wireName: r'cancelled')
  static const RunWakeState cancelled = _$cancelled;
  @BuiltValueEnumConst(wireName: r'expired')
  static const RunWakeState expired = _$expired;
  @BuiltValueEnumConst(wireName: r'refused')
  static const RunWakeState refused = _$refused;
  @BuiltValueEnumConst(wireName: r'failed')
  static const RunWakeState failed = _$failed;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunWakeState unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<RunWakeState> get serializer => _$runWakeStateSerializer;

  const RunWakeState._(String name) : super(name);

  static BuiltSet<RunWakeState> get values => _$values;
  static RunWakeState valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class RunWakeStateMixin = Object with _$RunWakeStateMixin;

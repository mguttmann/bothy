//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_state.g.dart';

class RunState extends EnumClass {
  @BuiltValueEnumConst(wireName: r'queued')
  static const RunState queued = _$queued;
  @BuiltValueEnumConst(wireName: r'running')
  static const RunState running = _$running;
  @BuiltValueEnumConst(wireName: r'waiting_input')
  static const RunState waitingInput = _$waitingInput;
  @BuiltValueEnumConst(wireName: r'completed')
  static const RunState completed = _$completed;
  @BuiltValueEnumConst(wireName: r'failed')
  static const RunState failed = _$failed;
  @BuiltValueEnumConst(wireName: r'cancelled')
  static const RunState cancelled = _$cancelled;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunState unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<RunState> get serializer => _$runStateSerializer;

  const RunState._(String name) : super(name);

  static BuiltSet<RunState> get values => _$values;
  static RunState valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class RunStateMixin = Object with _$RunStateMixin;

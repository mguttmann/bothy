//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'trigger_state.g.dart';

class TriggerState extends EnumClass {
  @BuiltValueEnumConst(wireName: r'received')
  static const TriggerState received = _$received;
  @BuiltValueEnumConst(wireName: r'queued')
  static const TriggerState queued = _$queued;
  @BuiltValueEnumConst(wireName: r'working')
  static const TriggerState working = _$working;
  @BuiltValueEnumConst(wireName: r'done')
  static const TriggerState done = _$done;
  @BuiltValueEnumConst(wireName: r'failed')
  static const TriggerState failed = _$failed;
  @BuiltValueEnumConst(wireName: r'cancelled')
  static const TriggerState cancelled = _$cancelled;
  @BuiltValueEnumConst(wireName: r'followup_delivered')
  static const TriggerState followupDelivered = _$followupDelivered;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const TriggerState unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<TriggerState> get serializer => _$triggerStateSerializer;

  const TriggerState._(String name) : super(name);

  static BuiltSet<TriggerState> get values => _$values;
  static TriggerState valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class TriggerStateMixin = Object with _$TriggerStateMixin;

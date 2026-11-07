//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake_reason.g.dart';

/// Why the wake-up exists (09 5.8.1); `agent` from `bothy-wake`, `recovery` after lost background work, `usage_limit` at the reset of the account's usage window.
class RunWakeReason extends EnumClass {
  @BuiltValueEnumConst(wireName: r'agent')
  static const RunWakeReason agent = _$agent;
  @BuiltValueEnumConst(wireName: r'recovery')
  static const RunWakeReason recovery = _$recovery;
  @BuiltValueEnumConst(wireName: r'usage_limit')
  static const RunWakeReason usageLimit = _$usageLimit;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunWakeReason unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<RunWakeReason> get serializer => _$runWakeReasonSerializer;

  const RunWakeReason._(String name) : super(name);

  static BuiltSet<RunWakeReason> get values => _$values;
  static RunWakeReason valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class RunWakeReasonMixin = Object with _$RunWakeReasonMixin;

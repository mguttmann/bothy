//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_runtime_state.g.dart';

/// State derived from heartbeats.
class AgentRuntimeState extends EnumClass {
  @BuiltValueEnumConst(wireName: r'unknown')
  static const AgentRuntimeState unknown = _$unknown;
  @BuiltValueEnumConst(wireName: r'online')
  static const AgentRuntimeState online = _$online;
  @BuiltValueEnumConst(wireName: r'degraded')
  static const AgentRuntimeState degraded = _$degraded;
  @BuiltValueEnumConst(wireName: r'offline')
  static const AgentRuntimeState offline = _$offline;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentRuntimeState unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<AgentRuntimeState> get serializer =>
      _$agentRuntimeStateSerializer;

  const AgentRuntimeState._(String name) : super(name);

  static BuiltSet<AgentRuntimeState> get values => _$values;
  static AgentRuntimeState valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class AgentRuntimeStateMixin = Object with _$AgentRuntimeStateMixin;

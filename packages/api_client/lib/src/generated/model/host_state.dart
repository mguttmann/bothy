//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_state.g.dart';

/// Life cycle of a host (24, 3.1). Only `active` allows sessions.
class HostState extends EnumClass {
  @BuiltValueEnumConst(wireName: r'pairing')
  static const HostState pairing = _$pairing;
  @BuiltValueEnumConst(wireName: r'confirming')
  static const HostState confirming = _$confirming;
  @BuiltValueEnumConst(wireName: r'active')
  static const HostState active = _$active;
  @BuiltValueEnumConst(wireName: r'disabled')
  static const HostState disabled = _$disabled;
  @BuiltValueEnumConst(wireName: r'retired')
  static const HostState retired = _$retired;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostState unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<HostState> get serializer => _$hostStateSerializer;

  const HostState._(String name) : super(name);

  static BuiltSet<HostState> get values => _$values;
  static HostState valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class HostStateMixin = Object with _$HostStateMixin;

//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'computer_session_state.g.dart';

/// State of a computer session (docs/spec/27-computer-use.md 8.1).
class ComputerSessionState extends EnumClass {
  @BuiltValueEnumConst(wireName: r'opening')
  static const ComputerSessionState opening = _$opening;
  @BuiltValueEnumConst(wireName: r'active')
  static const ComputerSessionState active = _$active;
  @BuiltValueEnumConst(wireName: r'paused')
  static const ComputerSessionState paused = _$paused;
  @BuiltValueEnumConst(wireName: r'closed')
  static const ComputerSessionState closed = _$closed;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ComputerSessionState unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<ComputerSessionState> get serializer =>
      _$computerSessionStateSerializer;

  const ComputerSessionState._(String name) : super(name);

  static BuiltSet<ComputerSessionState> get values => _$values;
  static ComputerSessionState valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class ComputerSessionStateMixin = Object
    with _$ComputerSessionStateMixin;

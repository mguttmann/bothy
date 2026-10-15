//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'thread_state.g.dart';

class ThreadState extends EnumClass {
  @BuiltValueEnumConst(wireName: r'open')
  static const ThreadState open = _$open;
  @BuiltValueEnumConst(wireName: r'closed')
  static const ThreadState closed = _$closed;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ThreadState unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<ThreadState> get serializer => _$threadStateSerializer;

  const ThreadState._(String name) : super(name);

  static BuiltSet<ThreadState> get values => _$values;
  static ThreadState valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class ThreadStateMixin = Object with _$ThreadStateMixin;

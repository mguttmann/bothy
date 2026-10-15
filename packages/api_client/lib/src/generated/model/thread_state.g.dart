// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ThreadState _$open = const ThreadState._('open');
const ThreadState _$closed = const ThreadState._('closed');
const ThreadState _$unknownDefaultOpenApi = const ThreadState._(
  'unknownDefaultOpenApi',
);

ThreadState _$valueOf(String name) {
  switch (name) {
    case 'open':
      return _$open;
    case 'closed':
      return _$closed;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<ThreadState> _$values = BuiltSet<ThreadState>(
  const <ThreadState>[_$open, _$closed, _$unknownDefaultOpenApi],
);

class _$ThreadStateMeta {
  const _$ThreadStateMeta();
  ThreadState get open => _$open;
  ThreadState get closed => _$closed;
  ThreadState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  ThreadState valueOf(String name) => _$valueOf(name);
  BuiltSet<ThreadState> get values => _$values;
}

mixin _$ThreadStateMixin {
  // ignore: non_constant_identifier_names
  _$ThreadStateMeta get ThreadState => const _$ThreadStateMeta();
}

Serializer<ThreadState> _$threadStateSerializer = _$ThreadStateSerializer();

class _$ThreadStateSerializer implements PrimitiveSerializer<ThreadState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'open': 'open',
    'closed': 'closed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'open': 'open',
    'closed': 'closed',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ThreadState];
  @override
  final String wireName = 'ThreadState';

  @override
  Object serialize(
    Serializers serializers,
    ThreadState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ThreadState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ThreadState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

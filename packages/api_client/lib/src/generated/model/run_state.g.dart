// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunState _$queued = const RunState._('queued');
const RunState _$running = const RunState._('running');
const RunState _$waitingInput = const RunState._('waitingInput');
const RunState _$completed = const RunState._('completed');
const RunState _$failed = const RunState._('failed');
const RunState _$cancelled = const RunState._('cancelled');
const RunState _$unknownDefaultOpenApi = const RunState._(
  'unknownDefaultOpenApi',
);

RunState _$valueOf(String name) {
  switch (name) {
    case 'queued':
      return _$queued;
    case 'running':
      return _$running;
    case 'waitingInput':
      return _$waitingInput;
    case 'completed':
      return _$completed;
    case 'failed':
      return _$failed;
    case 'cancelled':
      return _$cancelled;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<RunState> _$values = BuiltSet<RunState>(const <RunState>[
  _$queued,
  _$running,
  _$waitingInput,
  _$completed,
  _$failed,
  _$cancelled,
  _$unknownDefaultOpenApi,
]);

class _$RunStateMeta {
  const _$RunStateMeta();
  RunState get queued => _$queued;
  RunState get running => _$running;
  RunState get waitingInput => _$waitingInput;
  RunState get completed => _$completed;
  RunState get failed => _$failed;
  RunState get cancelled => _$cancelled;
  RunState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  RunState valueOf(String name) => _$valueOf(name);
  BuiltSet<RunState> get values => _$values;
}

mixin _$RunStateMixin {
  // ignore: non_constant_identifier_names
  _$RunStateMeta get RunState => const _$RunStateMeta();
}

Serializer<RunState> _$runStateSerializer = _$RunStateSerializer();

class _$RunStateSerializer implements PrimitiveSerializer<RunState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'queued': 'queued',
    'running': 'running',
    'waitingInput': 'waiting_input',
    'completed': 'completed',
    'failed': 'failed',
    'cancelled': 'cancelled',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'queued': 'queued',
    'running': 'running',
    'waiting_input': 'waitingInput',
    'completed': 'completed',
    'failed': 'failed',
    'cancelled': 'cancelled',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunState];
  @override
  final String wireName = 'RunState';

  @override
  Object serialize(
    Serializers serializers,
    RunState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunWakeState _$pending = const RunWakeState._('pending');
const RunWakeState _$fired = const RunWakeState._('fired');
const RunWakeState _$cancelled = const RunWakeState._('cancelled');
const RunWakeState _$expired = const RunWakeState._('expired');
const RunWakeState _$refused = const RunWakeState._('refused');
const RunWakeState _$failed = const RunWakeState._('failed');
const RunWakeState _$unknownDefaultOpenApi = const RunWakeState._(
  'unknownDefaultOpenApi',
);

RunWakeState _$valueOf(String name) {
  switch (name) {
    case 'pending':
      return _$pending;
    case 'fired':
      return _$fired;
    case 'cancelled':
      return _$cancelled;
    case 'expired':
      return _$expired;
    case 'refused':
      return _$refused;
    case 'failed':
      return _$failed;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<RunWakeState> _$values = BuiltSet<RunWakeState>(
  const <RunWakeState>[
    _$pending,
    _$fired,
    _$cancelled,
    _$expired,
    _$refused,
    _$failed,
    _$unknownDefaultOpenApi,
  ],
);

class _$RunWakeStateMeta {
  const _$RunWakeStateMeta();
  RunWakeState get pending => _$pending;
  RunWakeState get fired => _$fired;
  RunWakeState get cancelled => _$cancelled;
  RunWakeState get expired => _$expired;
  RunWakeState get refused => _$refused;
  RunWakeState get failed => _$failed;
  RunWakeState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  RunWakeState valueOf(String name) => _$valueOf(name);
  BuiltSet<RunWakeState> get values => _$values;
}

mixin _$RunWakeStateMixin {
  // ignore: non_constant_identifier_names
  _$RunWakeStateMeta get RunWakeState => const _$RunWakeStateMeta();
}

Serializer<RunWakeState> _$runWakeStateSerializer = _$RunWakeStateSerializer();

class _$RunWakeStateSerializer implements PrimitiveSerializer<RunWakeState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'pending': 'pending',
    'fired': 'fired',
    'cancelled': 'cancelled',
    'expired': 'expired',
    'refused': 'refused',
    'failed': 'failed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'pending': 'pending',
    'fired': 'fired',
    'cancelled': 'cancelled',
    'expired': 'expired',
    'refused': 'refused',
    'failed': 'failed',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunWakeState];
  @override
  final String wireName = 'RunWakeState';

  @override
  Object serialize(
    Serializers serializers,
    RunWakeState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunWakeState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunWakeState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

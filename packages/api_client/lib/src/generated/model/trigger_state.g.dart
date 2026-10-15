// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'trigger_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const TriggerState _$received = const TriggerState._('received');
const TriggerState _$queued = const TriggerState._('queued');
const TriggerState _$working = const TriggerState._('working');
const TriggerState _$done = const TriggerState._('done');
const TriggerState _$failed = const TriggerState._('failed');
const TriggerState _$cancelled = const TriggerState._('cancelled');
const TriggerState _$followupDelivered = const TriggerState._(
  'followupDelivered',
);
const TriggerState _$unknownDefaultOpenApi = const TriggerState._(
  'unknownDefaultOpenApi',
);

TriggerState _$valueOf(String name) {
  switch (name) {
    case 'received':
      return _$received;
    case 'queued':
      return _$queued;
    case 'working':
      return _$working;
    case 'done':
      return _$done;
    case 'failed':
      return _$failed;
    case 'cancelled':
      return _$cancelled;
    case 'followupDelivered':
      return _$followupDelivered;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<TriggerState> _$values = BuiltSet<TriggerState>(
  const <TriggerState>[
    _$received,
    _$queued,
    _$working,
    _$done,
    _$failed,
    _$cancelled,
    _$followupDelivered,
    _$unknownDefaultOpenApi,
  ],
);

class _$TriggerStateMeta {
  const _$TriggerStateMeta();
  TriggerState get received => _$received;
  TriggerState get queued => _$queued;
  TriggerState get working => _$working;
  TriggerState get done => _$done;
  TriggerState get failed => _$failed;
  TriggerState get cancelled => _$cancelled;
  TriggerState get followupDelivered => _$followupDelivered;
  TriggerState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  TriggerState valueOf(String name) => _$valueOf(name);
  BuiltSet<TriggerState> get values => _$values;
}

mixin _$TriggerStateMixin {
  // ignore: non_constant_identifier_names
  _$TriggerStateMeta get TriggerState => const _$TriggerStateMeta();
}

Serializer<TriggerState> _$triggerStateSerializer = _$TriggerStateSerializer();

class _$TriggerStateSerializer implements PrimitiveSerializer<TriggerState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'received': 'received',
    'queued': 'queued',
    'working': 'working',
    'done': 'done',
    'failed': 'failed',
    'cancelled': 'cancelled',
    'followupDelivered': 'followup_delivered',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'received': 'received',
    'queued': 'queued',
    'working': 'working',
    'done': 'done',
    'failed': 'failed',
    'cancelled': 'cancelled',
    'followup_delivered': 'followupDelivered',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[TriggerState];
  @override
  final String wireName = 'TriggerState';

  @override
  Object serialize(
    Serializers serializers,
    TriggerState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  TriggerState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => TriggerState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

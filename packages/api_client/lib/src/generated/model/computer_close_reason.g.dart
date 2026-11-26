// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'computer_close_reason.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ComputerCloseReason _$done = const ComputerCloseReason._('done');
const ComputerCloseReason _$stoppedByUser = const ComputerCloseReason._(
  'stoppedByUser',
);
const ComputerCloseReason _$deviceDisabled = const ComputerCloseReason._(
  'deviceDisabled',
);
const ComputerCloseReason _$runEnded = const ComputerCloseReason._('runEnded');
const ComputerCloseReason _$accessRevoked = const ComputerCloseReason._(
  'accessRevoked',
);
const ComputerCloseReason _$hostDisabled = const ComputerCloseReason._(
  'hostDisabled',
);
const ComputerCloseReason _$hostRetired = const ComputerCloseReason._(
  'hostRetired',
);
const ComputerCloseReason _$hostRekeyed = const ComputerCloseReason._(
  'hostRekeyed',
);
const ComputerCloseReason _$hostRestarted = const ComputerCloseReason._(
  'hostRestarted',
);
const ComputerCloseReason _$connectionLost = const ComputerCloseReason._(
  'connectionLost',
);
const ComputerCloseReason _$openFailed = const ComputerCloseReason._(
  'openFailed',
);
const ComputerCloseReason _$error = const ComputerCloseReason._('error');
const ComputerCloseReason _$unknownDefaultOpenApi = const ComputerCloseReason._(
  'unknownDefaultOpenApi',
);

ComputerCloseReason _$valueOf(String name) {
  switch (name) {
    case 'done':
      return _$done;
    case 'stoppedByUser':
      return _$stoppedByUser;
    case 'deviceDisabled':
      return _$deviceDisabled;
    case 'runEnded':
      return _$runEnded;
    case 'accessRevoked':
      return _$accessRevoked;
    case 'hostDisabled':
      return _$hostDisabled;
    case 'hostRetired':
      return _$hostRetired;
    case 'hostRekeyed':
      return _$hostRekeyed;
    case 'hostRestarted':
      return _$hostRestarted;
    case 'connectionLost':
      return _$connectionLost;
    case 'openFailed':
      return _$openFailed;
    case 'error':
      return _$error;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<ComputerCloseReason> _$values = BuiltSet<ComputerCloseReason>(
  const <ComputerCloseReason>[
    _$done,
    _$stoppedByUser,
    _$deviceDisabled,
    _$runEnded,
    _$accessRevoked,
    _$hostDisabled,
    _$hostRetired,
    _$hostRekeyed,
    _$hostRestarted,
    _$connectionLost,
    _$openFailed,
    _$error,
    _$unknownDefaultOpenApi,
  ],
);

class _$ComputerCloseReasonMeta {
  const _$ComputerCloseReasonMeta();
  ComputerCloseReason get done => _$done;
  ComputerCloseReason get stoppedByUser => _$stoppedByUser;
  ComputerCloseReason get deviceDisabled => _$deviceDisabled;
  ComputerCloseReason get runEnded => _$runEnded;
  ComputerCloseReason get accessRevoked => _$accessRevoked;
  ComputerCloseReason get hostDisabled => _$hostDisabled;
  ComputerCloseReason get hostRetired => _$hostRetired;
  ComputerCloseReason get hostRekeyed => _$hostRekeyed;
  ComputerCloseReason get hostRestarted => _$hostRestarted;
  ComputerCloseReason get connectionLost => _$connectionLost;
  ComputerCloseReason get openFailed => _$openFailed;
  ComputerCloseReason get error => _$error;
  ComputerCloseReason get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  ComputerCloseReason valueOf(String name) => _$valueOf(name);
  BuiltSet<ComputerCloseReason> get values => _$values;
}

mixin _$ComputerCloseReasonMixin {
  // ignore: non_constant_identifier_names
  _$ComputerCloseReasonMeta get ComputerCloseReason =>
      const _$ComputerCloseReasonMeta();
}

Serializer<ComputerCloseReason> _$computerCloseReasonSerializer =
    _$ComputerCloseReasonSerializer();

class _$ComputerCloseReasonSerializer
    implements PrimitiveSerializer<ComputerCloseReason> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'done': 'done',
    'stoppedByUser': 'stopped_by_user',
    'deviceDisabled': 'device_disabled',
    'runEnded': 'run_ended',
    'accessRevoked': 'access_revoked',
    'hostDisabled': 'host_disabled',
    'hostRetired': 'host_retired',
    'hostRekeyed': 'host_rekeyed',
    'hostRestarted': 'host_restarted',
    'connectionLost': 'connection_lost',
    'openFailed': 'open_failed',
    'error': 'error',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'done': 'done',
    'stopped_by_user': 'stoppedByUser',
    'device_disabled': 'deviceDisabled',
    'run_ended': 'runEnded',
    'access_revoked': 'accessRevoked',
    'host_disabled': 'hostDisabled',
    'host_retired': 'hostRetired',
    'host_rekeyed': 'hostRekeyed',
    'host_restarted': 'hostRestarted',
    'connection_lost': 'connectionLost',
    'open_failed': 'openFailed',
    'error': 'error',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ComputerCloseReason];
  @override
  final String wireName = 'ComputerCloseReason';

  @override
  Object serialize(
    Serializers serializers,
    ComputerCloseReason object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ComputerCloseReason deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ComputerCloseReason.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

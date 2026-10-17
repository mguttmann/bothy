// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const HostState _$pairing = const HostState._('pairing');
const HostState _$confirming = const HostState._('confirming');
const HostState _$active = const HostState._('active');
const HostState _$disabled = const HostState._('disabled');
const HostState _$retired = const HostState._('retired');
const HostState _$unknownDefaultOpenApi = const HostState._(
  'unknownDefaultOpenApi',
);

HostState _$valueOf(String name) {
  switch (name) {
    case 'pairing':
      return _$pairing;
    case 'confirming':
      return _$confirming;
    case 'active':
      return _$active;
    case 'disabled':
      return _$disabled;
    case 'retired':
      return _$retired;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<HostState> _$values = BuiltSet<HostState>(const <HostState>[
  _$pairing,
  _$confirming,
  _$active,
  _$disabled,
  _$retired,
  _$unknownDefaultOpenApi,
]);

class _$HostStateMeta {
  const _$HostStateMeta();
  HostState get pairing => _$pairing;
  HostState get confirming => _$confirming;
  HostState get active => _$active;
  HostState get disabled => _$disabled;
  HostState get retired => _$retired;
  HostState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  HostState valueOf(String name) => _$valueOf(name);
  BuiltSet<HostState> get values => _$values;
}

mixin _$HostStateMixin {
  // ignore: non_constant_identifier_names
  _$HostStateMeta get HostState => const _$HostStateMeta();
}

Serializer<HostState> _$hostStateSerializer = _$HostStateSerializer();

class _$HostStateSerializer implements PrimitiveSerializer<HostState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'pairing': 'pairing',
    'confirming': 'confirming',
    'active': 'active',
    'disabled': 'disabled',
    'retired': 'retired',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'pairing': 'pairing',
    'confirming': 'confirming',
    'active': 'active',
    'disabled': 'disabled',
    'retired': 'retired',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostState];
  @override
  final String wireName = 'HostState';

  @override
  Object serialize(
    Serializers serializers,
    HostState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

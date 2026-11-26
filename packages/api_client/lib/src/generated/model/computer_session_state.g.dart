// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'computer_session_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ComputerSessionState _$opening = const ComputerSessionState._('opening');
const ComputerSessionState _$active = const ComputerSessionState._('active');
const ComputerSessionState _$paused = const ComputerSessionState._('paused');
const ComputerSessionState _$closed = const ComputerSessionState._('closed');
const ComputerSessionState _$unknownDefaultOpenApi =
    const ComputerSessionState._('unknownDefaultOpenApi');

ComputerSessionState _$valueOf(String name) {
  switch (name) {
    case 'opening':
      return _$opening;
    case 'active':
      return _$active;
    case 'paused':
      return _$paused;
    case 'closed':
      return _$closed;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<ComputerSessionState> _$values = BuiltSet<ComputerSessionState>(
  const <ComputerSessionState>[
    _$opening,
    _$active,
    _$paused,
    _$closed,
    _$unknownDefaultOpenApi,
  ],
);

class _$ComputerSessionStateMeta {
  const _$ComputerSessionStateMeta();
  ComputerSessionState get opening => _$opening;
  ComputerSessionState get active => _$active;
  ComputerSessionState get paused => _$paused;
  ComputerSessionState get closed => _$closed;
  ComputerSessionState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  ComputerSessionState valueOf(String name) => _$valueOf(name);
  BuiltSet<ComputerSessionState> get values => _$values;
}

mixin _$ComputerSessionStateMixin {
  // ignore: non_constant_identifier_names
  _$ComputerSessionStateMeta get ComputerSessionState =>
      const _$ComputerSessionStateMeta();
}

Serializer<ComputerSessionState> _$computerSessionStateSerializer =
    _$ComputerSessionStateSerializer();

class _$ComputerSessionStateSerializer
    implements PrimitiveSerializer<ComputerSessionState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'opening': 'opening',
    'active': 'active',
    'paused': 'paused',
    'closed': 'closed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'opening': 'opening',
    'active': 'active',
    'paused': 'paused',
    'closed': 'closed',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ComputerSessionState];
  @override
  final String wireName = 'ComputerSessionState';

  @override
  Object serialize(
    Serializers serializers,
    ComputerSessionState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ComputerSessionState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ComputerSessionState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

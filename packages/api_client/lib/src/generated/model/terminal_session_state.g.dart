// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'terminal_session_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const TerminalSessionState _$opening = const TerminalSessionState._('opening');
const TerminalSessionState _$active = const TerminalSessionState._('active');
const TerminalSessionState _$detached = const TerminalSessionState._(
  'detached',
);
const TerminalSessionState _$closed = const TerminalSessionState._('closed');
const TerminalSessionState _$unknownDefaultOpenApi =
    const TerminalSessionState._('unknownDefaultOpenApi');

TerminalSessionState _$valueOf(String name) {
  switch (name) {
    case 'opening':
      return _$opening;
    case 'active':
      return _$active;
    case 'detached':
      return _$detached;
    case 'closed':
      return _$closed;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<TerminalSessionState> _$values = BuiltSet<TerminalSessionState>(
  const <TerminalSessionState>[
    _$opening,
    _$active,
    _$detached,
    _$closed,
    _$unknownDefaultOpenApi,
  ],
);

class _$TerminalSessionStateMeta {
  const _$TerminalSessionStateMeta();
  TerminalSessionState get opening => _$opening;
  TerminalSessionState get active => _$active;
  TerminalSessionState get detached => _$detached;
  TerminalSessionState get closed => _$closed;
  TerminalSessionState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  TerminalSessionState valueOf(String name) => _$valueOf(name);
  BuiltSet<TerminalSessionState> get values => _$values;
}

mixin _$TerminalSessionStateMixin {
  // ignore: non_constant_identifier_names
  _$TerminalSessionStateMeta get TerminalSessionState =>
      const _$TerminalSessionStateMeta();
}

Serializer<TerminalSessionState> _$terminalSessionStateSerializer =
    _$TerminalSessionStateSerializer();

class _$TerminalSessionStateSerializer
    implements PrimitiveSerializer<TerminalSessionState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'opening': 'opening',
    'active': 'active',
    'detached': 'detached',
    'closed': 'closed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'opening': 'opening',
    'active': 'active',
    'detached': 'detached',
    'closed': 'closed',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[TerminalSessionState];
  @override
  final String wireName = 'TerminalSessionState';

  @override
  Object serialize(
    Serializers serializers,
    TerminalSessionState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  TerminalSessionState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => TerminalSessionState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

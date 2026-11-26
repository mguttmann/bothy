// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_computer_started.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const HostComputerStartedStateEnum _$hostComputerStartedStateEnum_active =
    const HostComputerStartedStateEnum._('active');
const HostComputerStartedStateEnum _$hostComputerStartedStateEnum_paused =
    const HostComputerStartedStateEnum._('paused');
const HostComputerStartedStateEnum
_$hostComputerStartedStateEnum_unknownDefaultOpenApi =
    const HostComputerStartedStateEnum._('unknownDefaultOpenApi');

HostComputerStartedStateEnum _$hostComputerStartedStateEnumValueOf(
  String name,
) {
  switch (name) {
    case 'active':
      return _$hostComputerStartedStateEnum_active;
    case 'paused':
      return _$hostComputerStartedStateEnum_paused;
    case 'unknownDefaultOpenApi':
      return _$hostComputerStartedStateEnum_unknownDefaultOpenApi;
    default:
      return _$hostComputerStartedStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HostComputerStartedStateEnum>
_$hostComputerStartedStateEnumValues = BuiltSet<HostComputerStartedStateEnum>(
  const <HostComputerStartedStateEnum>[
    _$hostComputerStartedStateEnum_active,
    _$hostComputerStartedStateEnum_paused,
    _$hostComputerStartedStateEnum_unknownDefaultOpenApi,
  ],
);

Serializer<HostComputerStartedStateEnum>
_$hostComputerStartedStateEnumSerializer =
    _$HostComputerStartedStateEnumSerializer();

class _$HostComputerStartedStateEnumSerializer
    implements PrimitiveSerializer<HostComputerStartedStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'active': 'active',
    'paused': 'paused',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'active': 'active',
    'paused': 'paused',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostComputerStartedStateEnum];
  @override
  final String wireName = 'HostComputerStartedStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    HostComputerStartedStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostComputerStartedStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostComputerStartedStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$HostComputerStarted extends HostComputerStarted {
  @override
  final String sessionId;
  @override
  final HostComputerStartedStateEnum state;

  factory _$HostComputerStarted([
    void Function(HostComputerStartedBuilder)? updates,
  ]) => (HostComputerStartedBuilder()..update(updates))._build();

  _$HostComputerStarted._({required this.sessionId, required this.state})
    : super._();
  @override
  HostComputerStarted rebuild(
    void Function(HostComputerStartedBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  HostComputerStartedBuilder toBuilder() =>
      HostComputerStartedBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostComputerStarted &&
        sessionId == other.sessionId &&
        state == other.state;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostComputerStarted')
          ..add('sessionId', sessionId)
          ..add('state', state))
        .toString();
  }
}

class HostComputerStartedBuilder
    implements Builder<HostComputerStarted, HostComputerStartedBuilder> {
  _$HostComputerStarted? _$v;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  HostComputerStartedStateEnum? _state;
  HostComputerStartedStateEnum? get state => _$this._state;
  set state(HostComputerStartedStateEnum? state) => _$this._state = state;

  HostComputerStartedBuilder() {
    HostComputerStarted._defaults(this);
  }

  HostComputerStartedBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _sessionId = $v.sessionId;
      _state = $v.state;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostComputerStarted other) {
    _$v = other as _$HostComputerStarted;
  }

  @override
  void update(void Function(HostComputerStartedBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostComputerStarted build() => _build();

  _$HostComputerStarted _build() {
    final _$result =
        _$v ??
        _$HostComputerStarted._(
          sessionId: BuiltValueNullFieldError.checkNotNull(
            sessionId,
            r'HostComputerStarted',
            'sessionId',
          ),
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'HostComputerStarted',
            'state',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

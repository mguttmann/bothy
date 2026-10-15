// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_heartbeat.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayHeartbeat extends GatewayHeartbeat {
  @override
  final int lastSeq;
  @override
  final Heartbeat? agent;

  factory _$GatewayHeartbeat([
    void Function(GatewayHeartbeatBuilder)? updates,
  ]) => (GatewayHeartbeatBuilder()..update(updates))._build();

  _$GatewayHeartbeat._({required this.lastSeq, this.agent}) : super._();
  @override
  GatewayHeartbeat rebuild(void Function(GatewayHeartbeatBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayHeartbeatBuilder toBuilder() =>
      GatewayHeartbeatBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayHeartbeat &&
        lastSeq == other.lastSeq &&
        agent == other.agent;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, lastSeq.hashCode);
    _$hash = $jc(_$hash, agent.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayHeartbeat')
          ..add('lastSeq', lastSeq)
          ..add('agent', agent))
        .toString();
  }
}

class GatewayHeartbeatBuilder
    implements Builder<GatewayHeartbeat, GatewayHeartbeatBuilder> {
  _$GatewayHeartbeat? _$v;

  int? _lastSeq;
  int? get lastSeq => _$this._lastSeq;
  set lastSeq(int? lastSeq) => _$this._lastSeq = lastSeq;

  HeartbeatBuilder? _agent;
  HeartbeatBuilder get agent => _$this._agent ??= HeartbeatBuilder();
  set agent(HeartbeatBuilder? agent) => _$this._agent = agent;

  GatewayHeartbeatBuilder() {
    GatewayHeartbeat._defaults(this);
  }

  GatewayHeartbeatBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _lastSeq = $v.lastSeq;
      _agent = $v.agent?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayHeartbeat other) {
    _$v = other as _$GatewayHeartbeat;
  }

  @override
  void update(void Function(GatewayHeartbeatBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayHeartbeat build() => _build();

  _$GatewayHeartbeat _build() {
    _$GatewayHeartbeat _$result;
    try {
      _$result =
          _$v ??
          _$GatewayHeartbeat._(
            lastSeq: BuiltValueNullFieldError.checkNotNull(
              lastSeq,
              r'GatewayHeartbeat',
              'lastSeq',
            ),
            agent: _agent?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'agent';
        _agent?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GatewayHeartbeat',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

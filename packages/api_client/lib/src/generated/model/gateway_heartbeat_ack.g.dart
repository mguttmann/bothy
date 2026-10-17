// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_heartbeat_ack.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayHeartbeatAck extends GatewayHeartbeatAck {
  @override
  final DateTime serverTime;

  factory _$GatewayHeartbeatAck([
    void Function(GatewayHeartbeatAckBuilder)? updates,
  ]) => (GatewayHeartbeatAckBuilder()..update(updates))._build();

  _$GatewayHeartbeatAck._({required this.serverTime}) : super._();
  @override
  GatewayHeartbeatAck rebuild(
    void Function(GatewayHeartbeatAckBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  GatewayHeartbeatAckBuilder toBuilder() =>
      GatewayHeartbeatAckBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayHeartbeatAck && serverTime == other.serverTime;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, serverTime.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'GatewayHeartbeatAck',
    )..add('serverTime', serverTime)).toString();
  }
}

class GatewayHeartbeatAckBuilder
    implements Builder<GatewayHeartbeatAck, GatewayHeartbeatAckBuilder> {
  _$GatewayHeartbeatAck? _$v;

  DateTime? _serverTime;
  DateTime? get serverTime => _$this._serverTime;
  set serverTime(DateTime? serverTime) => _$this._serverTime = serverTime;

  GatewayHeartbeatAckBuilder() {
    GatewayHeartbeatAck._defaults(this);
  }

  GatewayHeartbeatAckBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _serverTime = $v.serverTime;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayHeartbeatAck other) {
    _$v = other as _$GatewayHeartbeatAck;
  }

  @override
  void update(void Function(GatewayHeartbeatAckBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayHeartbeatAck build() => _build();

  _$GatewayHeartbeatAck _build() {
    final _$result =
        _$v ??
        _$GatewayHeartbeatAck._(
          serverTime: BuiltValueNullFieldError.checkNotNull(
            serverTime,
            r'GatewayHeartbeatAck',
            'serverTime',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

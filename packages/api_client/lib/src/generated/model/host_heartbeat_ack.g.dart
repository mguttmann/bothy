// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_heartbeat_ack.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostHeartbeatAck extends HostHeartbeatAck {
  @override
  final DateTime? serverTime;
  @override
  final Jwks? jwks;

  factory _$HostHeartbeatAck([
    void Function(HostHeartbeatAckBuilder)? updates,
  ]) => (HostHeartbeatAckBuilder()..update(updates))._build();

  _$HostHeartbeatAck._({this.serverTime, this.jwks}) : super._();
  @override
  HostHeartbeatAck rebuild(void Function(HostHeartbeatAckBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostHeartbeatAckBuilder toBuilder() =>
      HostHeartbeatAckBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostHeartbeatAck &&
        serverTime == other.serverTime &&
        jwks == other.jwks;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, serverTime.hashCode);
    _$hash = $jc(_$hash, jwks.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostHeartbeatAck')
          ..add('serverTime', serverTime)
          ..add('jwks', jwks))
        .toString();
  }
}

class HostHeartbeatAckBuilder
    implements Builder<HostHeartbeatAck, HostHeartbeatAckBuilder> {
  _$HostHeartbeatAck? _$v;

  DateTime? _serverTime;
  DateTime? get serverTime => _$this._serverTime;
  set serverTime(DateTime? serverTime) => _$this._serverTime = serverTime;

  JwksBuilder? _jwks;
  JwksBuilder get jwks => _$this._jwks ??= JwksBuilder();
  set jwks(JwksBuilder? jwks) => _$this._jwks = jwks;

  HostHeartbeatAckBuilder() {
    HostHeartbeatAck._defaults(this);
  }

  HostHeartbeatAckBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _serverTime = $v.serverTime;
      _jwks = $v.jwks?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostHeartbeatAck other) {
    _$v = other as _$HostHeartbeatAck;
  }

  @override
  void update(void Function(HostHeartbeatAckBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostHeartbeatAck build() => _build();

  _$HostHeartbeatAck _build() {
    _$HostHeartbeatAck _$result;
    try {
      _$result =
          _$v ??
          _$HostHeartbeatAck._(serverTime: serverTime, jwks: _jwks?.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'jwks';
        _jwks?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostHeartbeatAck',
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

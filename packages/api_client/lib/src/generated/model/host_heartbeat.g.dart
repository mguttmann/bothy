// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_heartbeat.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostHeartbeat extends HostHeartbeat {
  @override
  final int activeSessions;

  factory _$HostHeartbeat([void Function(HostHeartbeatBuilder)? updates]) =>
      (HostHeartbeatBuilder()..update(updates))._build();

  _$HostHeartbeat._({required this.activeSessions}) : super._();
  @override
  HostHeartbeat rebuild(void Function(HostHeartbeatBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostHeartbeatBuilder toBuilder() => HostHeartbeatBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostHeartbeat && activeSessions == other.activeSessions;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, activeSessions.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'HostHeartbeat',
    )..add('activeSessions', activeSessions)).toString();
  }
}

class HostHeartbeatBuilder
    implements Builder<HostHeartbeat, HostHeartbeatBuilder> {
  _$HostHeartbeat? _$v;

  int? _activeSessions;
  int? get activeSessions => _$this._activeSessions;
  set activeSessions(int? activeSessions) =>
      _$this._activeSessions = activeSessions;

  HostHeartbeatBuilder() {
    HostHeartbeat._defaults(this);
  }

  HostHeartbeatBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _activeSessions = $v.activeSessions;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostHeartbeat other) {
    _$v = other as _$HostHeartbeat;
  }

  @override
  void update(void Function(HostHeartbeatBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostHeartbeat build() => _build();

  _$HostHeartbeat _build() {
    final _$result =
        _$v ??
        _$HostHeartbeat._(
          activeSessions: BuiltValueNullFieldError.checkNotNull(
            activeSessions,
            r'HostHeartbeat',
            'activeSessions',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

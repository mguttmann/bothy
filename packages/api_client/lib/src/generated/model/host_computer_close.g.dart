// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_computer_close.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostComputerClose extends HostComputerClose {
  @override
  final String sessionId;
  @override
  final ComputerCloseReason reason;

  factory _$HostComputerClose([
    void Function(HostComputerCloseBuilder)? updates,
  ]) => (HostComputerCloseBuilder()..update(updates))._build();

  _$HostComputerClose._({required this.sessionId, required this.reason})
    : super._();
  @override
  HostComputerClose rebuild(void Function(HostComputerCloseBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostComputerCloseBuilder toBuilder() =>
      HostComputerCloseBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostComputerClose &&
        sessionId == other.sessionId &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostComputerClose')
          ..add('sessionId', sessionId)
          ..add('reason', reason))
        .toString();
  }
}

class HostComputerCloseBuilder
    implements Builder<HostComputerClose, HostComputerCloseBuilder> {
  _$HostComputerClose? _$v;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  ComputerCloseReason? _reason;
  ComputerCloseReason? get reason => _$this._reason;
  set reason(ComputerCloseReason? reason) => _$this._reason = reason;

  HostComputerCloseBuilder() {
    HostComputerClose._defaults(this);
  }

  HostComputerCloseBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _sessionId = $v.sessionId;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostComputerClose other) {
    _$v = other as _$HostComputerClose;
  }

  @override
  void update(void Function(HostComputerCloseBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostComputerClose build() => _build();

  _$HostComputerClose _build() {
    final _$result =
        _$v ??
        _$HostComputerClose._(
          sessionId: BuiltValueNullFieldError.checkNotNull(
            sessionId,
            r'HostComputerClose',
            'sessionId',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'HostComputerClose',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

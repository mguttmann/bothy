// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_session_close.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostSessionClose extends HostSessionClose {
  @override
  final String sessionId;
  @override
  final TerminalCloseReason closeReason;

  factory _$HostSessionClose([
    void Function(HostSessionCloseBuilder)? updates,
  ]) => (HostSessionCloseBuilder()..update(updates))._build();

  _$HostSessionClose._({required this.sessionId, required this.closeReason})
    : super._();
  @override
  HostSessionClose rebuild(void Function(HostSessionCloseBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostSessionCloseBuilder toBuilder() =>
      HostSessionCloseBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostSessionClose &&
        sessionId == other.sessionId &&
        closeReason == other.closeReason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jc(_$hash, closeReason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostSessionClose')
          ..add('sessionId', sessionId)
          ..add('closeReason', closeReason))
        .toString();
  }
}

class HostSessionCloseBuilder
    implements Builder<HostSessionClose, HostSessionCloseBuilder> {
  _$HostSessionClose? _$v;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  TerminalCloseReason? _closeReason;
  TerminalCloseReason? get closeReason => _$this._closeReason;
  set closeReason(TerminalCloseReason? closeReason) =>
      _$this._closeReason = closeReason;

  HostSessionCloseBuilder() {
    HostSessionClose._defaults(this);
  }

  HostSessionCloseBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _sessionId = $v.sessionId;
      _closeReason = $v.closeReason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostSessionClose other) {
    _$v = other as _$HostSessionClose;
  }

  @override
  void update(void Function(HostSessionCloseBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostSessionClose build() => _build();

  _$HostSessionClose _build() {
    final _$result =
        _$v ??
        _$HostSessionClose._(
          sessionId: BuiltValueNullFieldError.checkNotNull(
            sessionId,
            r'HostSessionClose',
            'sessionId',
          ),
          closeReason: BuiltValueNullFieldError.checkNotNull(
            closeReason,
            r'HostSessionClose',
            'closeReason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_session_ended.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostSessionEnded extends HostSessionEnded {
  @override
  final String sessionId;
  @override
  final TerminalCloseReason closeReason;
  @override
  final TerminalCloseDetail? closeDetail;
  @override
  final int? exitCode;

  factory _$HostSessionEnded([
    void Function(HostSessionEndedBuilder)? updates,
  ]) => (HostSessionEndedBuilder()..update(updates))._build();

  _$HostSessionEnded._({
    required this.sessionId,
    required this.closeReason,
    this.closeDetail,
    this.exitCode,
  }) : super._();
  @override
  HostSessionEnded rebuild(void Function(HostSessionEndedBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostSessionEndedBuilder toBuilder() =>
      HostSessionEndedBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostSessionEnded &&
        sessionId == other.sessionId &&
        closeReason == other.closeReason &&
        closeDetail == other.closeDetail &&
        exitCode == other.exitCode;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jc(_$hash, closeReason.hashCode);
    _$hash = $jc(_$hash, closeDetail.hashCode);
    _$hash = $jc(_$hash, exitCode.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostSessionEnded')
          ..add('sessionId', sessionId)
          ..add('closeReason', closeReason)
          ..add('closeDetail', closeDetail)
          ..add('exitCode', exitCode))
        .toString();
  }
}

class HostSessionEndedBuilder
    implements Builder<HostSessionEnded, HostSessionEndedBuilder> {
  _$HostSessionEnded? _$v;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  TerminalCloseReason? _closeReason;
  TerminalCloseReason? get closeReason => _$this._closeReason;
  set closeReason(TerminalCloseReason? closeReason) =>
      _$this._closeReason = closeReason;

  TerminalCloseDetail? _closeDetail;
  TerminalCloseDetail? get closeDetail => _$this._closeDetail;
  set closeDetail(TerminalCloseDetail? closeDetail) =>
      _$this._closeDetail = closeDetail;

  int? _exitCode;
  int? get exitCode => _$this._exitCode;
  set exitCode(int? exitCode) => _$this._exitCode = exitCode;

  HostSessionEndedBuilder() {
    HostSessionEnded._defaults(this);
  }

  HostSessionEndedBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _sessionId = $v.sessionId;
      _closeReason = $v.closeReason;
      _closeDetail = $v.closeDetail;
      _exitCode = $v.exitCode;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostSessionEnded other) {
    _$v = other as _$HostSessionEnded;
  }

  @override
  void update(void Function(HostSessionEndedBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostSessionEnded build() => _build();

  _$HostSessionEnded _build() {
    final _$result =
        _$v ??
        _$HostSessionEnded._(
          sessionId: BuiltValueNullFieldError.checkNotNull(
            sessionId,
            r'HostSessionEnded',
            'sessionId',
          ),
          closeReason: BuiltValueNullFieldError.checkNotNull(
            closeReason,
            r'HostSessionEnded',
            'closeReason',
          ),
          closeDetail: closeDetail,
          exitCode: exitCode,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

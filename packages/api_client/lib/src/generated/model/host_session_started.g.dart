// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_session_started.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostSessionStarted extends HostSessionStarted {
  @override
  final String sessionId;

  factory _$HostSessionStarted([
    void Function(HostSessionStartedBuilder)? updates,
  ]) => (HostSessionStartedBuilder()..update(updates))._build();

  _$HostSessionStarted._({required this.sessionId}) : super._();
  @override
  HostSessionStarted rebuild(
    void Function(HostSessionStartedBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  HostSessionStartedBuilder toBuilder() =>
      HostSessionStartedBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostSessionStarted && sessionId == other.sessionId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'HostSessionStarted',
    )..add('sessionId', sessionId)).toString();
  }
}

class HostSessionStartedBuilder
    implements Builder<HostSessionStarted, HostSessionStartedBuilder> {
  _$HostSessionStarted? _$v;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  HostSessionStartedBuilder() {
    HostSessionStarted._defaults(this);
  }

  HostSessionStartedBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _sessionId = $v.sessionId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostSessionStarted other) {
    _$v = other as _$HostSessionStarted;
  }

  @override
  void update(void Function(HostSessionStartedBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostSessionStarted build() => _build();

  _$HostSessionStarted _build() {
    final _$result =
        _$v ??
        _$HostSessionStarted._(
          sessionId: BuiltValueNullFieldError.checkNotNull(
            sessionId,
            r'HostSessionStarted',
            'sessionId',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

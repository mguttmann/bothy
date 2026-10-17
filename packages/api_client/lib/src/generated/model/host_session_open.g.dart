// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_session_open.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostSessionOpen extends HostSessionOpen {
  @override
  final String sessionId;
  @override
  final String dataPath;
  @override
  final String profile;
  @override
  final DateTime expiresAt;

  factory _$HostSessionOpen([void Function(HostSessionOpenBuilder)? updates]) =>
      (HostSessionOpenBuilder()..update(updates))._build();

  _$HostSessionOpen._({
    required this.sessionId,
    required this.dataPath,
    required this.profile,
    required this.expiresAt,
  }) : super._();
  @override
  HostSessionOpen rebuild(void Function(HostSessionOpenBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostSessionOpenBuilder toBuilder() => HostSessionOpenBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostSessionOpen &&
        sessionId == other.sessionId &&
        dataPath == other.dataPath &&
        profile == other.profile &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jc(_$hash, dataPath.hashCode);
    _$hash = $jc(_$hash, profile.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostSessionOpen')
          ..add('sessionId', sessionId)
          ..add('dataPath', dataPath)
          ..add('profile', profile)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class HostSessionOpenBuilder
    implements Builder<HostSessionOpen, HostSessionOpenBuilder> {
  _$HostSessionOpen? _$v;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  String? _dataPath;
  String? get dataPath => _$this._dataPath;
  set dataPath(String? dataPath) => _$this._dataPath = dataPath;

  String? _profile;
  String? get profile => _$this._profile;
  set profile(String? profile) => _$this._profile = profile;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  HostSessionOpenBuilder() {
    HostSessionOpen._defaults(this);
  }

  HostSessionOpenBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _sessionId = $v.sessionId;
      _dataPath = $v.dataPath;
      _profile = $v.profile;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostSessionOpen other) {
    _$v = other as _$HostSessionOpen;
  }

  @override
  void update(void Function(HostSessionOpenBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostSessionOpen build() => _build();

  _$HostSessionOpen _build() {
    final _$result =
        _$v ??
        _$HostSessionOpen._(
          sessionId: BuiltValueNullFieldError.checkNotNull(
            sessionId,
            r'HostSessionOpen',
            'sessionId',
          ),
          dataPath: BuiltValueNullFieldError.checkNotNull(
            dataPath,
            r'HostSessionOpen',
            'dataPath',
          ),
          profile: BuiltValueNullFieldError.checkNotNull(
            profile,
            r'HostSessionOpen',
            'profile',
          ),
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'HostSessionOpen',
            'expiresAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

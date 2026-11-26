// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'computer_session_grant.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ComputerSessionGrant extends ComputerSessionGrant {
  @override
  final String sessionId;
  @override
  final String token;
  @override
  final String attachPath;
  @override
  final String noisePublicKey;
  @override
  final String keyFingerprint;
  @override
  final int keyGeneration;
  @override
  final DateTime expiresAt;

  factory _$ComputerSessionGrant([
    void Function(ComputerSessionGrantBuilder)? updates,
  ]) => (ComputerSessionGrantBuilder()..update(updates))._build();

  _$ComputerSessionGrant._({
    required this.sessionId,
    required this.token,
    required this.attachPath,
    required this.noisePublicKey,
    required this.keyFingerprint,
    required this.keyGeneration,
    required this.expiresAt,
  }) : super._();
  @override
  ComputerSessionGrant rebuild(
    void Function(ComputerSessionGrantBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ComputerSessionGrantBuilder toBuilder() =>
      ComputerSessionGrantBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ComputerSessionGrant &&
        sessionId == other.sessionId &&
        token == other.token &&
        attachPath == other.attachPath &&
        noisePublicKey == other.noisePublicKey &&
        keyFingerprint == other.keyFingerprint &&
        keyGeneration == other.keyGeneration &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jc(_$hash, token.hashCode);
    _$hash = $jc(_$hash, attachPath.hashCode);
    _$hash = $jc(_$hash, noisePublicKey.hashCode);
    _$hash = $jc(_$hash, keyFingerprint.hashCode);
    _$hash = $jc(_$hash, keyGeneration.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ComputerSessionGrant')
          ..add('sessionId', sessionId)
          ..add('token', token)
          ..add('attachPath', attachPath)
          ..add('noisePublicKey', noisePublicKey)
          ..add('keyFingerprint', keyFingerprint)
          ..add('keyGeneration', keyGeneration)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class ComputerSessionGrantBuilder
    implements Builder<ComputerSessionGrant, ComputerSessionGrantBuilder> {
  _$ComputerSessionGrant? _$v;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  String? _token;
  String? get token => _$this._token;
  set token(String? token) => _$this._token = token;

  String? _attachPath;
  String? get attachPath => _$this._attachPath;
  set attachPath(String? attachPath) => _$this._attachPath = attachPath;

  String? _noisePublicKey;
  String? get noisePublicKey => _$this._noisePublicKey;
  set noisePublicKey(String? noisePublicKey) =>
      _$this._noisePublicKey = noisePublicKey;

  String? _keyFingerprint;
  String? get keyFingerprint => _$this._keyFingerprint;
  set keyFingerprint(String? keyFingerprint) =>
      _$this._keyFingerprint = keyFingerprint;

  int? _keyGeneration;
  int? get keyGeneration => _$this._keyGeneration;
  set keyGeneration(int? keyGeneration) =>
      _$this._keyGeneration = keyGeneration;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  ComputerSessionGrantBuilder() {
    ComputerSessionGrant._defaults(this);
  }

  ComputerSessionGrantBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _sessionId = $v.sessionId;
      _token = $v.token;
      _attachPath = $v.attachPath;
      _noisePublicKey = $v.noisePublicKey;
      _keyFingerprint = $v.keyFingerprint;
      _keyGeneration = $v.keyGeneration;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ComputerSessionGrant other) {
    _$v = other as _$ComputerSessionGrant;
  }

  @override
  void update(void Function(ComputerSessionGrantBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ComputerSessionGrant build() => _build();

  _$ComputerSessionGrant _build() {
    final _$result =
        _$v ??
        _$ComputerSessionGrant._(
          sessionId: BuiltValueNullFieldError.checkNotNull(
            sessionId,
            r'ComputerSessionGrant',
            'sessionId',
          ),
          token: BuiltValueNullFieldError.checkNotNull(
            token,
            r'ComputerSessionGrant',
            'token',
          ),
          attachPath: BuiltValueNullFieldError.checkNotNull(
            attachPath,
            r'ComputerSessionGrant',
            'attachPath',
          ),
          noisePublicKey: BuiltValueNullFieldError.checkNotNull(
            noisePublicKey,
            r'ComputerSessionGrant',
            'noisePublicKey',
          ),
          keyFingerprint: BuiltValueNullFieldError.checkNotNull(
            keyFingerprint,
            r'ComputerSessionGrant',
            'keyFingerprint',
          ),
          keyGeneration: BuiltValueNullFieldError.checkNotNull(
            keyGeneration,
            r'ComputerSessionGrant',
            'keyGeneration',
          ),
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'ComputerSessionGrant',
            'expiresAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

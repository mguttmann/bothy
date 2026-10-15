// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'oidc_begin_response.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$OidcBeginResponse extends OidcBeginResponse {
  @override
  final String loginAttemptId;
  @override
  final String authorizationUrl;
  @override
  final DateTime expiresAt;

  factory _$OidcBeginResponse([
    void Function(OidcBeginResponseBuilder)? updates,
  ]) => (OidcBeginResponseBuilder()..update(updates))._build();

  _$OidcBeginResponse._({
    required this.loginAttemptId,
    required this.authorizationUrl,
    required this.expiresAt,
  }) : super._();
  @override
  OidcBeginResponse rebuild(void Function(OidcBeginResponseBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  OidcBeginResponseBuilder toBuilder() =>
      OidcBeginResponseBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is OidcBeginResponse &&
        loginAttemptId == other.loginAttemptId &&
        authorizationUrl == other.authorizationUrl &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, loginAttemptId.hashCode);
    _$hash = $jc(_$hash, authorizationUrl.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'OidcBeginResponse')
          ..add('loginAttemptId', loginAttemptId)
          ..add('authorizationUrl', authorizationUrl)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class OidcBeginResponseBuilder
    implements Builder<OidcBeginResponse, OidcBeginResponseBuilder> {
  _$OidcBeginResponse? _$v;

  String? _loginAttemptId;
  String? get loginAttemptId => _$this._loginAttemptId;
  set loginAttemptId(String? loginAttemptId) =>
      _$this._loginAttemptId = loginAttemptId;

  String? _authorizationUrl;
  String? get authorizationUrl => _$this._authorizationUrl;
  set authorizationUrl(String? authorizationUrl) =>
      _$this._authorizationUrl = authorizationUrl;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  OidcBeginResponseBuilder() {
    OidcBeginResponse._defaults(this);
  }

  OidcBeginResponseBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _loginAttemptId = $v.loginAttemptId;
      _authorizationUrl = $v.authorizationUrl;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(OidcBeginResponse other) {
    _$v = other as _$OidcBeginResponse;
  }

  @override
  void update(void Function(OidcBeginResponseBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  OidcBeginResponse build() => _build();

  _$OidcBeginResponse _build() {
    final _$result =
        _$v ??
        _$OidcBeginResponse._(
          loginAttemptId: BuiltValueNullFieldError.checkNotNull(
            loginAttemptId,
            r'OidcBeginResponse',
            'loginAttemptId',
          ),
          authorizationUrl: BuiltValueNullFieldError.checkNotNull(
            authorizationUrl,
            r'OidcBeginResponse',
            'authorizationUrl',
          ),
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'OidcBeginResponse',
            'expiresAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'oidc_begin_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$OidcBeginRequest extends OidcBeginRequest {
  @override
  final LoginClient? client;
  @override
  final String providerId;
  @override
  final String appRedirectUri;
  @override
  final String clientChallenge;
  @override
  final String? enrollmentCode;

  factory _$OidcBeginRequest([
    void Function(OidcBeginRequestBuilder)? updates,
  ]) => (OidcBeginRequestBuilder()..update(updates))._build();

  _$OidcBeginRequest._({
    this.client,
    required this.providerId,
    required this.appRedirectUri,
    required this.clientChallenge,
    this.enrollmentCode,
  }) : super._();
  @override
  OidcBeginRequest rebuild(void Function(OidcBeginRequestBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  OidcBeginRequestBuilder toBuilder() =>
      OidcBeginRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is OidcBeginRequest &&
        client == other.client &&
        providerId == other.providerId &&
        appRedirectUri == other.appRedirectUri &&
        clientChallenge == other.clientChallenge &&
        enrollmentCode == other.enrollmentCode;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, client.hashCode);
    _$hash = $jc(_$hash, providerId.hashCode);
    _$hash = $jc(_$hash, appRedirectUri.hashCode);
    _$hash = $jc(_$hash, clientChallenge.hashCode);
    _$hash = $jc(_$hash, enrollmentCode.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'OidcBeginRequest')
          ..add('client', client)
          ..add('providerId', providerId)
          ..add('appRedirectUri', appRedirectUri)
          ..add('clientChallenge', clientChallenge)
          ..add('enrollmentCode', enrollmentCode))
        .toString();
  }
}

class OidcBeginRequestBuilder
    implements Builder<OidcBeginRequest, OidcBeginRequestBuilder> {
  _$OidcBeginRequest? _$v;

  LoginClient? _client;
  LoginClient? get client => _$this._client;
  set client(LoginClient? client) => _$this._client = client;

  String? _providerId;
  String? get providerId => _$this._providerId;
  set providerId(String? providerId) => _$this._providerId = providerId;

  String? _appRedirectUri;
  String? get appRedirectUri => _$this._appRedirectUri;
  set appRedirectUri(String? appRedirectUri) =>
      _$this._appRedirectUri = appRedirectUri;

  String? _clientChallenge;
  String? get clientChallenge => _$this._clientChallenge;
  set clientChallenge(String? clientChallenge) =>
      _$this._clientChallenge = clientChallenge;

  String? _enrollmentCode;
  String? get enrollmentCode => _$this._enrollmentCode;
  set enrollmentCode(String? enrollmentCode) =>
      _$this._enrollmentCode = enrollmentCode;

  OidcBeginRequestBuilder() {
    OidcBeginRequest._defaults(this);
  }

  OidcBeginRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _client = $v.client;
      _providerId = $v.providerId;
      _appRedirectUri = $v.appRedirectUri;
      _clientChallenge = $v.clientChallenge;
      _enrollmentCode = $v.enrollmentCode;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(OidcBeginRequest other) {
    _$v = other as _$OidcBeginRequest;
  }

  @override
  void update(void Function(OidcBeginRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  OidcBeginRequest build() => _build();

  _$OidcBeginRequest _build() {
    final _$result =
        _$v ??
        _$OidcBeginRequest._(
          client: client,
          providerId: BuiltValueNullFieldError.checkNotNull(
            providerId,
            r'OidcBeginRequest',
            'providerId',
          ),
          appRedirectUri: BuiltValueNullFieldError.checkNotNull(
            appRedirectUri,
            r'OidcBeginRequest',
            'appRedirectUri',
          ),
          clientChallenge: BuiltValueNullFieldError.checkNotNull(
            clientChallenge,
            r'OidcBeginRequest',
            'clientChallenge',
          ),
          enrollmentCode: enrollmentCode,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

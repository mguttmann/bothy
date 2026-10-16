// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'step_up_oidc_begin_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StepUpOidcBeginRequest extends StepUpOidcBeginRequest {
  @override
  final String challengeId;
  @override
  final String providerId;
  @override
  final String appRedirectUri;
  @override
  final String clientChallenge;

  factory _$StepUpOidcBeginRequest([
    void Function(StepUpOidcBeginRequestBuilder)? updates,
  ]) => (StepUpOidcBeginRequestBuilder()..update(updates))._build();

  _$StepUpOidcBeginRequest._({
    required this.challengeId,
    required this.providerId,
    required this.appRedirectUri,
    required this.clientChallenge,
  }) : super._();
  @override
  StepUpOidcBeginRequest rebuild(
    void Function(StepUpOidcBeginRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StepUpOidcBeginRequestBuilder toBuilder() =>
      StepUpOidcBeginRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StepUpOidcBeginRequest &&
        challengeId == other.challengeId &&
        providerId == other.providerId &&
        appRedirectUri == other.appRedirectUri &&
        clientChallenge == other.clientChallenge;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, challengeId.hashCode);
    _$hash = $jc(_$hash, providerId.hashCode);
    _$hash = $jc(_$hash, appRedirectUri.hashCode);
    _$hash = $jc(_$hash, clientChallenge.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StepUpOidcBeginRequest')
          ..add('challengeId', challengeId)
          ..add('providerId', providerId)
          ..add('appRedirectUri', appRedirectUri)
          ..add('clientChallenge', clientChallenge))
        .toString();
  }
}

class StepUpOidcBeginRequestBuilder
    implements Builder<StepUpOidcBeginRequest, StepUpOidcBeginRequestBuilder> {
  _$StepUpOidcBeginRequest? _$v;

  String? _challengeId;
  String? get challengeId => _$this._challengeId;
  set challengeId(String? challengeId) => _$this._challengeId = challengeId;

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

  StepUpOidcBeginRequestBuilder() {
    StepUpOidcBeginRequest._defaults(this);
  }

  StepUpOidcBeginRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _challengeId = $v.challengeId;
      _providerId = $v.providerId;
      _appRedirectUri = $v.appRedirectUri;
      _clientChallenge = $v.clientChallenge;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StepUpOidcBeginRequest other) {
    _$v = other as _$StepUpOidcBeginRequest;
  }

  @override
  void update(void Function(StepUpOidcBeginRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StepUpOidcBeginRequest build() => _build();

  _$StepUpOidcBeginRequest _build() {
    final _$result =
        _$v ??
        _$StepUpOidcBeginRequest._(
          challengeId: BuiltValueNullFieldError.checkNotNull(
            challengeId,
            r'StepUpOidcBeginRequest',
            'challengeId',
          ),
          providerId: BuiltValueNullFieldError.checkNotNull(
            providerId,
            r'StepUpOidcBeginRequest',
            'providerId',
          ),
          appRedirectUri: BuiltValueNullFieldError.checkNotNull(
            appRedirectUri,
            r'StepUpOidcBeginRequest',
            'appRedirectUri',
          ),
          clientChallenge: BuiltValueNullFieldError.checkNotNull(
            clientChallenge,
            r'StepUpOidcBeginRequest',
            'clientChallenge',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

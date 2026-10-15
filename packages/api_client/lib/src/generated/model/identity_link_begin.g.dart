// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'identity_link_begin.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$IdentityLinkBegin extends IdentityLinkBegin {
  @override
  final String providerId;
  @override
  final String appRedirectUri;
  @override
  final String clientChallenge;

  factory _$IdentityLinkBegin([
    void Function(IdentityLinkBeginBuilder)? updates,
  ]) => (IdentityLinkBeginBuilder()..update(updates))._build();

  _$IdentityLinkBegin._({
    required this.providerId,
    required this.appRedirectUri,
    required this.clientChallenge,
  }) : super._();
  @override
  IdentityLinkBegin rebuild(void Function(IdentityLinkBeginBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  IdentityLinkBeginBuilder toBuilder() =>
      IdentityLinkBeginBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IdentityLinkBegin &&
        providerId == other.providerId &&
        appRedirectUri == other.appRedirectUri &&
        clientChallenge == other.clientChallenge;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, providerId.hashCode);
    _$hash = $jc(_$hash, appRedirectUri.hashCode);
    _$hash = $jc(_$hash, clientChallenge.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'IdentityLinkBegin')
          ..add('providerId', providerId)
          ..add('appRedirectUri', appRedirectUri)
          ..add('clientChallenge', clientChallenge))
        .toString();
  }
}

class IdentityLinkBeginBuilder
    implements Builder<IdentityLinkBegin, IdentityLinkBeginBuilder> {
  _$IdentityLinkBegin? _$v;

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

  IdentityLinkBeginBuilder() {
    IdentityLinkBegin._defaults(this);
  }

  IdentityLinkBeginBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _providerId = $v.providerId;
      _appRedirectUri = $v.appRedirectUri;
      _clientChallenge = $v.clientChallenge;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IdentityLinkBegin other) {
    _$v = other as _$IdentityLinkBegin;
  }

  @override
  void update(void Function(IdentityLinkBeginBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IdentityLinkBegin build() => _build();

  _$IdentityLinkBegin _build() {
    final _$result =
        _$v ??
        _$IdentityLinkBegin._(
          providerId: BuiltValueNullFieldError.checkNotNull(
            providerId,
            r'IdentityLinkBegin',
            'providerId',
          ),
          appRedirectUri: BuiltValueNullFieldError.checkNotNull(
            appRedirectUri,
            r'IdentityLinkBegin',
            'appRedirectUri',
          ),
          clientChallenge: BuiltValueNullFieldError.checkNotNull(
            clientChallenge,
            r'IdentityLinkBegin',
            'clientChallenge',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'push_registration_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PushRegistrationCreate extends PushRegistrationCreate {
  @override
  final PushProvider provider;
  @override
  final String token;
  @override
  final PushEnvironment? environment;
  @override
  final String appId;

  factory _$PushRegistrationCreate([
    void Function(PushRegistrationCreateBuilder)? updates,
  ]) => (PushRegistrationCreateBuilder()..update(updates))._build();

  _$PushRegistrationCreate._({
    required this.provider,
    required this.token,
    this.environment,
    required this.appId,
  }) : super._();
  @override
  PushRegistrationCreate rebuild(
    void Function(PushRegistrationCreateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  PushRegistrationCreateBuilder toBuilder() =>
      PushRegistrationCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PushRegistrationCreate &&
        provider == other.provider &&
        token == other.token &&
        environment == other.environment &&
        appId == other.appId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, provider.hashCode);
    _$hash = $jc(_$hash, token.hashCode);
    _$hash = $jc(_$hash, environment.hashCode);
    _$hash = $jc(_$hash, appId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'PushRegistrationCreate')
          ..add('provider', provider)
          ..add('token', token)
          ..add('environment', environment)
          ..add('appId', appId))
        .toString();
  }
}

class PushRegistrationCreateBuilder
    implements Builder<PushRegistrationCreate, PushRegistrationCreateBuilder> {
  _$PushRegistrationCreate? _$v;

  PushProvider? _provider;
  PushProvider? get provider => _$this._provider;
  set provider(PushProvider? provider) => _$this._provider = provider;

  String? _token;
  String? get token => _$this._token;
  set token(String? token) => _$this._token = token;

  PushEnvironment? _environment;
  PushEnvironment? get environment => _$this._environment;
  set environment(PushEnvironment? environment) =>
      _$this._environment = environment;

  String? _appId;
  String? get appId => _$this._appId;
  set appId(String? appId) => _$this._appId = appId;

  PushRegistrationCreateBuilder() {
    PushRegistrationCreate._defaults(this);
  }

  PushRegistrationCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _provider = $v.provider;
      _token = $v.token;
      _environment = $v.environment;
      _appId = $v.appId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PushRegistrationCreate other) {
    _$v = other as _$PushRegistrationCreate;
  }

  @override
  void update(void Function(PushRegistrationCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PushRegistrationCreate build() => _build();

  _$PushRegistrationCreate _build() {
    final _$result =
        _$v ??
        _$PushRegistrationCreate._(
          provider: BuiltValueNullFieldError.checkNotNull(
            provider,
            r'PushRegistrationCreate',
            'provider',
          ),
          token: BuiltValueNullFieldError.checkNotNull(
            token,
            r'PushRegistrationCreate',
            'token',
          ),
          environment: environment,
          appId: BuiltValueNullFieldError.checkNotNull(
            appId,
            r'PushRegistrationCreate',
            'appId',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

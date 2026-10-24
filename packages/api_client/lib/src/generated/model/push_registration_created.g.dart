// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'push_registration_created.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PushRegistrationCreated extends PushRegistrationCreated {
  @override
  final String previewToken;
  @override
  final String id;
  @override
  final String deviceId;
  @override
  final PushProvider provider;
  @override
  final PushEnvironment environment;
  @override
  final String appId;
  @override
  final DateTime createdAt;
  @override
  final DateTime? disabledAt;
  @override
  final String? disabledReason;

  factory _$PushRegistrationCreated([
    void Function(PushRegistrationCreatedBuilder)? updates,
  ]) => (PushRegistrationCreatedBuilder()..update(updates))._build();

  _$PushRegistrationCreated._({
    required this.previewToken,
    required this.id,
    required this.deviceId,
    required this.provider,
    required this.environment,
    required this.appId,
    required this.createdAt,
    this.disabledAt,
    this.disabledReason,
  }) : super._();
  @override
  PushRegistrationCreated rebuild(
    void Function(PushRegistrationCreatedBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  PushRegistrationCreatedBuilder toBuilder() =>
      PushRegistrationCreatedBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PushRegistrationCreated &&
        previewToken == other.previewToken &&
        id == other.id &&
        deviceId == other.deviceId &&
        provider == other.provider &&
        environment == other.environment &&
        appId == other.appId &&
        createdAt == other.createdAt &&
        disabledAt == other.disabledAt &&
        disabledReason == other.disabledReason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, previewToken.hashCode);
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, deviceId.hashCode);
    _$hash = $jc(_$hash, provider.hashCode);
    _$hash = $jc(_$hash, environment.hashCode);
    _$hash = $jc(_$hash, appId.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, disabledAt.hashCode);
    _$hash = $jc(_$hash, disabledReason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'PushRegistrationCreated')
          ..add('previewToken', previewToken)
          ..add('id', id)
          ..add('deviceId', deviceId)
          ..add('provider', provider)
          ..add('environment', environment)
          ..add('appId', appId)
          ..add('createdAt', createdAt)
          ..add('disabledAt', disabledAt)
          ..add('disabledReason', disabledReason))
        .toString();
  }
}

class PushRegistrationCreatedBuilder
    implements
        Builder<PushRegistrationCreated, PushRegistrationCreatedBuilder>,
        PushRegistrationBuilder {
  _$PushRegistrationCreated? _$v;

  String? _previewToken;
  String? get previewToken => _$this._previewToken;
  set previewToken(covariant String? previewToken) =>
      _$this._previewToken = previewToken;

  String? _id;
  String? get id => _$this._id;
  set id(covariant String? id) => _$this._id = id;

  String? _deviceId;
  String? get deviceId => _$this._deviceId;
  set deviceId(covariant String? deviceId) => _$this._deviceId = deviceId;

  PushProvider? _provider;
  PushProvider? get provider => _$this._provider;
  set provider(covariant PushProvider? provider) => _$this._provider = provider;

  PushEnvironment? _environment;
  PushEnvironment? get environment => _$this._environment;
  set environment(covariant PushEnvironment? environment) =>
      _$this._environment = environment;

  String? _appId;
  String? get appId => _$this._appId;
  set appId(covariant String? appId) => _$this._appId = appId;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(covariant DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _disabledAt;
  DateTime? get disabledAt => _$this._disabledAt;
  set disabledAt(covariant DateTime? disabledAt) =>
      _$this._disabledAt = disabledAt;

  String? _disabledReason;
  String? get disabledReason => _$this._disabledReason;
  set disabledReason(covariant String? disabledReason) =>
      _$this._disabledReason = disabledReason;

  PushRegistrationCreatedBuilder() {
    PushRegistrationCreated._defaults(this);
  }

  PushRegistrationCreatedBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _previewToken = $v.previewToken;
      _id = $v.id;
      _deviceId = $v.deviceId;
      _provider = $v.provider;
      _environment = $v.environment;
      _appId = $v.appId;
      _createdAt = $v.createdAt;
      _disabledAt = $v.disabledAt;
      _disabledReason = $v.disabledReason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(covariant PushRegistrationCreated other) {
    _$v = other as _$PushRegistrationCreated;
  }

  @override
  void update(void Function(PushRegistrationCreatedBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PushRegistrationCreated build() => _build();

  _$PushRegistrationCreated _build() {
    final _$result =
        _$v ??
        _$PushRegistrationCreated._(
          previewToken: BuiltValueNullFieldError.checkNotNull(
            previewToken,
            r'PushRegistrationCreated',
            'previewToken',
          ),
          id: BuiltValueNullFieldError.checkNotNull(
            id,
            r'PushRegistrationCreated',
            'id',
          ),
          deviceId: BuiltValueNullFieldError.checkNotNull(
            deviceId,
            r'PushRegistrationCreated',
            'deviceId',
          ),
          provider: BuiltValueNullFieldError.checkNotNull(
            provider,
            r'PushRegistrationCreated',
            'provider',
          ),
          environment: BuiltValueNullFieldError.checkNotNull(
            environment,
            r'PushRegistrationCreated',
            'environment',
          ),
          appId: BuiltValueNullFieldError.checkNotNull(
            appId,
            r'PushRegistrationCreated',
            'appId',
          ),
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'PushRegistrationCreated',
            'createdAt',
          ),
          disabledAt: disabledAt,
          disabledReason: disabledReason,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

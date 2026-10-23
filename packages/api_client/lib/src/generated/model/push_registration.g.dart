// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'push_registration.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

abstract mixin class PushRegistrationBuilder {
  void replace(PushRegistration other);
  void update(void Function(PushRegistrationBuilder) updates);
  String? get id;
  set id(String? id);

  String? get deviceId;
  set deviceId(String? deviceId);

  PushProvider? get provider;
  set provider(PushProvider? provider);

  PushEnvironment? get environment;
  set environment(PushEnvironment? environment);

  String? get appId;
  set appId(String? appId);

  DateTime? get createdAt;
  set createdAt(DateTime? createdAt);

  DateTime? get disabledAt;
  set disabledAt(DateTime? disabledAt);

  String? get disabledReason;
  set disabledReason(String? disabledReason);
}

class _$$PushRegistration extends $PushRegistration {
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

  factory _$$PushRegistration([
    void Function($PushRegistrationBuilder)? updates,
  ]) => ($PushRegistrationBuilder()..update(updates))._build();

  _$$PushRegistration._({
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
  $PushRegistration rebuild(void Function($PushRegistrationBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  $PushRegistrationBuilder toBuilder() =>
      $PushRegistrationBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is $PushRegistration &&
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
    return (newBuiltValueToStringHelper(r'$PushRegistration')
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

class $PushRegistrationBuilder
    implements
        Builder<$PushRegistration, $PushRegistrationBuilder>,
        PushRegistrationBuilder {
  _$$PushRegistration? _$v;

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

  $PushRegistrationBuilder() {
    $PushRegistration._defaults(this);
  }

  $PushRegistrationBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
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
  void replace(covariant $PushRegistration other) {
    _$v = other as _$$PushRegistration;
  }

  @override
  void update(void Function($PushRegistrationBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  $PushRegistration build() => _build();

  _$$PushRegistration _build() {
    final _$result =
        _$v ??
        _$$PushRegistration._(
          id: BuiltValueNullFieldError.checkNotNull(
            id,
            r'$PushRegistration',
            'id',
          ),
          deviceId: BuiltValueNullFieldError.checkNotNull(
            deviceId,
            r'$PushRegistration',
            'deviceId',
          ),
          provider: BuiltValueNullFieldError.checkNotNull(
            provider,
            r'$PushRegistration',
            'provider',
          ),
          environment: BuiltValueNullFieldError.checkNotNull(
            environment,
            r'$PushRegistration',
            'environment',
          ),
          appId: BuiltValueNullFieldError.checkNotNull(
            appId,
            r'$PushRegistration',
            'appId',
          ),
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'$PushRegistration',
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

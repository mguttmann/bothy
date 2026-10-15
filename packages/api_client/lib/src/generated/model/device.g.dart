// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'device.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const DeviceStateEnum _$deviceStateEnum_pending = const DeviceStateEnum._(
  'pending',
);
const DeviceStateEnum _$deviceStateEnum_active = const DeviceStateEnum._(
  'active',
);
const DeviceStateEnum _$deviceStateEnum_revoked = const DeviceStateEnum._(
  'revoked',
);
const DeviceStateEnum _$deviceStateEnum_unknownDefaultOpenApi =
    const DeviceStateEnum._('unknownDefaultOpenApi');

DeviceStateEnum _$deviceStateEnumValueOf(String name) {
  switch (name) {
    case 'pending':
      return _$deviceStateEnum_pending;
    case 'active':
      return _$deviceStateEnum_active;
    case 'revoked':
      return _$deviceStateEnum_revoked;
    case 'unknownDefaultOpenApi':
      return _$deviceStateEnum_unknownDefaultOpenApi;
    default:
      return _$deviceStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<DeviceStateEnum> _$deviceStateEnumValues =
    BuiltSet<DeviceStateEnum>(const <DeviceStateEnum>[
      _$deviceStateEnum_pending,
      _$deviceStateEnum_active,
      _$deviceStateEnum_revoked,
      _$deviceStateEnum_unknownDefaultOpenApi,
    ]);

const DeviceRevokeReasonEnum _$deviceRevokeReasonEnum_revokedBySelf =
    const DeviceRevokeReasonEnum._('revokedBySelf');
const DeviceRevokeReasonEnum _$deviceRevokeReasonEnum_revokedByOwner =
    const DeviceRevokeReasonEnum._('revokedByOwner');
const DeviceRevokeReasonEnum _$deviceRevokeReasonEnum_userDisabled =
    const DeviceRevokeReasonEnum._('userDisabled');
const DeviceRevokeReasonEnum _$deviceRevokeReasonEnum_inactive =
    const DeviceRevokeReasonEnum._('inactive');
const DeviceRevokeReasonEnum _$deviceRevokeReasonEnum_verificationFailed =
    const DeviceRevokeReasonEnum._('verificationFailed');
const DeviceRevokeReasonEnum _$deviceRevokeReasonEnum_pendingExpired =
    const DeviceRevokeReasonEnum._('pendingExpired');
const DeviceRevokeReasonEnum _$deviceRevokeReasonEnum_pairingExpired =
    const DeviceRevokeReasonEnum._('pairingExpired');
const DeviceRevokeReasonEnum _$deviceRevokeReasonEnum_unknownDefaultOpenApi =
    const DeviceRevokeReasonEnum._('unknownDefaultOpenApi');

DeviceRevokeReasonEnum _$deviceRevokeReasonEnumValueOf(String name) {
  switch (name) {
    case 'revokedBySelf':
      return _$deviceRevokeReasonEnum_revokedBySelf;
    case 'revokedByOwner':
      return _$deviceRevokeReasonEnum_revokedByOwner;
    case 'userDisabled':
      return _$deviceRevokeReasonEnum_userDisabled;
    case 'inactive':
      return _$deviceRevokeReasonEnum_inactive;
    case 'verificationFailed':
      return _$deviceRevokeReasonEnum_verificationFailed;
    case 'pendingExpired':
      return _$deviceRevokeReasonEnum_pendingExpired;
    case 'pairingExpired':
      return _$deviceRevokeReasonEnum_pairingExpired;
    case 'unknownDefaultOpenApi':
      return _$deviceRevokeReasonEnum_unknownDefaultOpenApi;
    default:
      return _$deviceRevokeReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<DeviceRevokeReasonEnum> _$deviceRevokeReasonEnumValues =
    BuiltSet<DeviceRevokeReasonEnum>(const <DeviceRevokeReasonEnum>[
      _$deviceRevokeReasonEnum_revokedBySelf,
      _$deviceRevokeReasonEnum_revokedByOwner,
      _$deviceRevokeReasonEnum_userDisabled,
      _$deviceRevokeReasonEnum_inactive,
      _$deviceRevokeReasonEnum_verificationFailed,
      _$deviceRevokeReasonEnum_pendingExpired,
      _$deviceRevokeReasonEnum_pairingExpired,
      _$deviceRevokeReasonEnum_unknownDefaultOpenApi,
    ]);

Serializer<DeviceStateEnum> _$deviceStateEnumSerializer =
    _$DeviceStateEnumSerializer();
Serializer<DeviceRevokeReasonEnum> _$deviceRevokeReasonEnumSerializer =
    _$DeviceRevokeReasonEnumSerializer();

class _$DeviceStateEnumSerializer
    implements PrimitiveSerializer<DeviceStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'pending': 'pending',
    'active': 'active',
    'revoked': 'revoked',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'pending': 'pending',
    'active': 'active',
    'revoked': 'revoked',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[DeviceStateEnum];
  @override
  final String wireName = 'DeviceStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    DeviceStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  DeviceStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => DeviceStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$DeviceRevokeReasonEnumSerializer
    implements PrimitiveSerializer<DeviceRevokeReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'revokedBySelf': 'revoked_by_self',
    'revokedByOwner': 'revoked_by_owner',
    'userDisabled': 'user_disabled',
    'inactive': 'inactive',
    'verificationFailed': 'verification_failed',
    'pendingExpired': 'pending_expired',
    'pairingExpired': 'pairing_expired',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'revoked_by_self': 'revokedBySelf',
    'revoked_by_owner': 'revokedByOwner',
    'user_disabled': 'userDisabled',
    'inactive': 'inactive',
    'verification_failed': 'verificationFailed',
    'pending_expired': 'pendingExpired',
    'pairing_expired': 'pairingExpired',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[DeviceRevokeReasonEnum];
  @override
  final String wireName = 'DeviceRevokeReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    DeviceRevokeReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  DeviceRevokeReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => DeviceRevokeReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$Device extends Device {
  @override
  final String id;
  @override
  final String userId;
  @override
  final String name;
  @override
  final DevicePlatform platform;
  @override
  final DeviceStateEnum state;
  @override
  final String? appVersion;
  @override
  final String? keyThumbprint;
  @override
  final String? stepUpKeyThumbprint;
  @override
  final String? verificationCode;
  @override
  final DeviceRevokeReasonEnum? revokeReason;
  @override
  final DateTime? approvedAt;
  @override
  final DateTime? revokedAt;
  @override
  final DateTime createdAt;
  @override
  final DateTime? lastSeenAt;
  @override
  final BuiltList<PushRegistration>? pushRegistrations;

  factory _$Device([void Function(DeviceBuilder)? updates]) =>
      (DeviceBuilder()..update(updates))._build();

  _$Device._({
    required this.id,
    required this.userId,
    required this.name,
    required this.platform,
    required this.state,
    this.appVersion,
    this.keyThumbprint,
    this.stepUpKeyThumbprint,
    this.verificationCode,
    this.revokeReason,
    this.approvedAt,
    this.revokedAt,
    required this.createdAt,
    this.lastSeenAt,
    this.pushRegistrations,
  }) : super._();
  @override
  Device rebuild(void Function(DeviceBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DeviceBuilder toBuilder() => DeviceBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Device &&
        id == other.id &&
        userId == other.userId &&
        name == other.name &&
        platform == other.platform &&
        state == other.state &&
        appVersion == other.appVersion &&
        keyThumbprint == other.keyThumbprint &&
        stepUpKeyThumbprint == other.stepUpKeyThumbprint &&
        verificationCode == other.verificationCode &&
        revokeReason == other.revokeReason &&
        approvedAt == other.approvedAt &&
        revokedAt == other.revokedAt &&
        createdAt == other.createdAt &&
        lastSeenAt == other.lastSeenAt &&
        pushRegistrations == other.pushRegistrations;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, platform.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, appVersion.hashCode);
    _$hash = $jc(_$hash, keyThumbprint.hashCode);
    _$hash = $jc(_$hash, stepUpKeyThumbprint.hashCode);
    _$hash = $jc(_$hash, verificationCode.hashCode);
    _$hash = $jc(_$hash, revokeReason.hashCode);
    _$hash = $jc(_$hash, approvedAt.hashCode);
    _$hash = $jc(_$hash, revokedAt.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, lastSeenAt.hashCode);
    _$hash = $jc(_$hash, pushRegistrations.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Device')
          ..add('id', id)
          ..add('userId', userId)
          ..add('name', name)
          ..add('platform', platform)
          ..add('state', state)
          ..add('appVersion', appVersion)
          ..add('keyThumbprint', keyThumbprint)
          ..add('stepUpKeyThumbprint', stepUpKeyThumbprint)
          ..add('verificationCode', verificationCode)
          ..add('revokeReason', revokeReason)
          ..add('approvedAt', approvedAt)
          ..add('revokedAt', revokedAt)
          ..add('createdAt', createdAt)
          ..add('lastSeenAt', lastSeenAt)
          ..add('pushRegistrations', pushRegistrations))
        .toString();
  }
}

class DeviceBuilder implements Builder<Device, DeviceBuilder> {
  _$Device? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  DevicePlatform? _platform;
  DevicePlatform? get platform => _$this._platform;
  set platform(DevicePlatform? platform) => _$this._platform = platform;

  DeviceStateEnum? _state;
  DeviceStateEnum? get state => _$this._state;
  set state(DeviceStateEnum? state) => _$this._state = state;

  String? _appVersion;
  String? get appVersion => _$this._appVersion;
  set appVersion(String? appVersion) => _$this._appVersion = appVersion;

  String? _keyThumbprint;
  String? get keyThumbprint => _$this._keyThumbprint;
  set keyThumbprint(String? keyThumbprint) =>
      _$this._keyThumbprint = keyThumbprint;

  String? _stepUpKeyThumbprint;
  String? get stepUpKeyThumbprint => _$this._stepUpKeyThumbprint;
  set stepUpKeyThumbprint(String? stepUpKeyThumbprint) =>
      _$this._stepUpKeyThumbprint = stepUpKeyThumbprint;

  String? _verificationCode;
  String? get verificationCode => _$this._verificationCode;
  set verificationCode(String? verificationCode) =>
      _$this._verificationCode = verificationCode;

  DeviceRevokeReasonEnum? _revokeReason;
  DeviceRevokeReasonEnum? get revokeReason => _$this._revokeReason;
  set revokeReason(DeviceRevokeReasonEnum? revokeReason) =>
      _$this._revokeReason = revokeReason;

  DateTime? _approvedAt;
  DateTime? get approvedAt => _$this._approvedAt;
  set approvedAt(DateTime? approvedAt) => _$this._approvedAt = approvedAt;

  DateTime? _revokedAt;
  DateTime? get revokedAt => _$this._revokedAt;
  set revokedAt(DateTime? revokedAt) => _$this._revokedAt = revokedAt;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _lastSeenAt;
  DateTime? get lastSeenAt => _$this._lastSeenAt;
  set lastSeenAt(DateTime? lastSeenAt) => _$this._lastSeenAt = lastSeenAt;

  ListBuilder<PushRegistration>? _pushRegistrations;
  ListBuilder<PushRegistration> get pushRegistrations =>
      _$this._pushRegistrations ??= ListBuilder<PushRegistration>();
  set pushRegistrations(ListBuilder<PushRegistration>? pushRegistrations) =>
      _$this._pushRegistrations = pushRegistrations;

  DeviceBuilder() {
    Device._defaults(this);
  }

  DeviceBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _userId = $v.userId;
      _name = $v.name;
      _platform = $v.platform;
      _state = $v.state;
      _appVersion = $v.appVersion;
      _keyThumbprint = $v.keyThumbprint;
      _stepUpKeyThumbprint = $v.stepUpKeyThumbprint;
      _verificationCode = $v.verificationCode;
      _revokeReason = $v.revokeReason;
      _approvedAt = $v.approvedAt;
      _revokedAt = $v.revokedAt;
      _createdAt = $v.createdAt;
      _lastSeenAt = $v.lastSeenAt;
      _pushRegistrations = $v.pushRegistrations?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Device other) {
    _$v = other as _$Device;
  }

  @override
  void update(void Function(DeviceBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Device build() => _build();

  _$Device _build() {
    _$Device _$result;
    try {
      _$result =
          _$v ??
          _$Device._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'Device', 'id'),
            userId: BuiltValueNullFieldError.checkNotNull(
              userId,
              r'Device',
              'userId',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'Device',
              'name',
            ),
            platform: BuiltValueNullFieldError.checkNotNull(
              platform,
              r'Device',
              'platform',
            ),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'Device',
              'state',
            ),
            appVersion: appVersion,
            keyThumbprint: keyThumbprint,
            stepUpKeyThumbprint: stepUpKeyThumbprint,
            verificationCode: verificationCode,
            revokeReason: revokeReason,
            approvedAt: approvedAt,
            revokedAt: revokedAt,
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'Device',
              'createdAt',
            ),
            lastSeenAt: lastSeenAt,
            pushRegistrations: _pushRegistrations?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'pushRegistrations';
        _pushRegistrations?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Device',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

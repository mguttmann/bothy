// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'session.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const SessionAuthMethodEnum _$sessionAuthMethodEnum_oidc =
    const SessionAuthMethodEnum._('oidc');
const SessionAuthMethodEnum _$sessionAuthMethodEnum_trustedHeader =
    const SessionAuthMethodEnum._('trustedHeader');
const SessionAuthMethodEnum _$sessionAuthMethodEnum_passkey =
    const SessionAuthMethodEnum._('passkey');
const SessionAuthMethodEnum _$sessionAuthMethodEnum_passwordTotp =
    const SessionAuthMethodEnum._('passwordTotp');
const SessionAuthMethodEnum _$sessionAuthMethodEnum_enrollmentCode =
    const SessionAuthMethodEnum._('enrollmentCode');
const SessionAuthMethodEnum _$sessionAuthMethodEnum_devicePairing =
    const SessionAuthMethodEnum._('devicePairing');
const SessionAuthMethodEnum _$sessionAuthMethodEnum_unknownDefaultOpenApi =
    const SessionAuthMethodEnum._('unknownDefaultOpenApi');

SessionAuthMethodEnum _$sessionAuthMethodEnumValueOf(String name) {
  switch (name) {
    case 'oidc':
      return _$sessionAuthMethodEnum_oidc;
    case 'trustedHeader':
      return _$sessionAuthMethodEnum_trustedHeader;
    case 'passkey':
      return _$sessionAuthMethodEnum_passkey;
    case 'passwordTotp':
      return _$sessionAuthMethodEnum_passwordTotp;
    case 'enrollmentCode':
      return _$sessionAuthMethodEnum_enrollmentCode;
    case 'devicePairing':
      return _$sessionAuthMethodEnum_devicePairing;
    case 'unknownDefaultOpenApi':
      return _$sessionAuthMethodEnum_unknownDefaultOpenApi;
    default:
      return _$sessionAuthMethodEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<SessionAuthMethodEnum> _$sessionAuthMethodEnumValues =
    BuiltSet<SessionAuthMethodEnum>(const <SessionAuthMethodEnum>[
      _$sessionAuthMethodEnum_oidc,
      _$sessionAuthMethodEnum_trustedHeader,
      _$sessionAuthMethodEnum_passkey,
      _$sessionAuthMethodEnum_passwordTotp,
      _$sessionAuthMethodEnum_enrollmentCode,
      _$sessionAuthMethodEnum_devicePairing,
      _$sessionAuthMethodEnum_unknownDefaultOpenApi,
    ]);

const SessionClientComponentEnum _$sessionClientComponentEnum_app =
    const SessionClientComponentEnum._('app');
const SessionClientComponentEnum _$sessionClientComponentEnum_shareExt =
    const SessionClientComponentEnum._('shareExt');
const SessionClientComponentEnum _$sessionClientComponentEnum_console =
    const SessionClientComponentEnum._('console');
const SessionClientComponentEnum
_$sessionClientComponentEnum_unknownDefaultOpenApi =
    const SessionClientComponentEnum._('unknownDefaultOpenApi');

SessionClientComponentEnum _$sessionClientComponentEnumValueOf(String name) {
  switch (name) {
    case 'app':
      return _$sessionClientComponentEnum_app;
    case 'shareExt':
      return _$sessionClientComponentEnum_shareExt;
    case 'console':
      return _$sessionClientComponentEnum_console;
    case 'unknownDefaultOpenApi':
      return _$sessionClientComponentEnum_unknownDefaultOpenApi;
    default:
      return _$sessionClientComponentEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<SessionClientComponentEnum> _$sessionClientComponentEnumValues =
    BuiltSet<SessionClientComponentEnum>(const <SessionClientComponentEnum>[
      _$sessionClientComponentEnum_app,
      _$sessionClientComponentEnum_shareExt,
      _$sessionClientComponentEnum_console,
      _$sessionClientComponentEnum_unknownDefaultOpenApi,
    ]);

const SessionStepUpMethodEnum _$sessionStepUpMethodEnum_passkey =
    const SessionStepUpMethodEnum._('passkey');
const SessionStepUpMethodEnum _$sessionStepUpMethodEnum_deviceKey =
    const SessionStepUpMethodEnum._('deviceKey');
const SessionStepUpMethodEnum _$sessionStepUpMethodEnum_passwordTotp =
    const SessionStepUpMethodEnum._('passwordTotp');
const SessionStepUpMethodEnum _$sessionStepUpMethodEnum_oidcReauth =
    const SessionStepUpMethodEnum._('oidcReauth');
const SessionStepUpMethodEnum _$sessionStepUpMethodEnum_unknownDefaultOpenApi =
    const SessionStepUpMethodEnum._('unknownDefaultOpenApi');

SessionStepUpMethodEnum _$sessionStepUpMethodEnumValueOf(String name) {
  switch (name) {
    case 'passkey':
      return _$sessionStepUpMethodEnum_passkey;
    case 'deviceKey':
      return _$sessionStepUpMethodEnum_deviceKey;
    case 'passwordTotp':
      return _$sessionStepUpMethodEnum_passwordTotp;
    case 'oidcReauth':
      return _$sessionStepUpMethodEnum_oidcReauth;
    case 'unknownDefaultOpenApi':
      return _$sessionStepUpMethodEnum_unknownDefaultOpenApi;
    default:
      return _$sessionStepUpMethodEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<SessionStepUpMethodEnum> _$sessionStepUpMethodEnumValues =
    BuiltSet<SessionStepUpMethodEnum>(const <SessionStepUpMethodEnum>[
      _$sessionStepUpMethodEnum_passkey,
      _$sessionStepUpMethodEnum_deviceKey,
      _$sessionStepUpMethodEnum_passwordTotp,
      _$sessionStepUpMethodEnum_oidcReauth,
      _$sessionStepUpMethodEnum_unknownDefaultOpenApi,
    ]);

Serializer<SessionAuthMethodEnum> _$sessionAuthMethodEnumSerializer =
    _$SessionAuthMethodEnumSerializer();
Serializer<SessionClientComponentEnum> _$sessionClientComponentEnumSerializer =
    _$SessionClientComponentEnumSerializer();
Serializer<SessionStepUpMethodEnum> _$sessionStepUpMethodEnumSerializer =
    _$SessionStepUpMethodEnumSerializer();

class _$SessionAuthMethodEnumSerializer
    implements PrimitiveSerializer<SessionAuthMethodEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'oidc': 'oidc',
    'trustedHeader': 'trusted_header',
    'passkey': 'passkey',
    'passwordTotp': 'password_totp',
    'enrollmentCode': 'enrollment_code',
    'devicePairing': 'device_pairing',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'oidc': 'oidc',
    'trusted_header': 'trustedHeader',
    'passkey': 'passkey',
    'password_totp': 'passwordTotp',
    'enrollment_code': 'enrollmentCode',
    'device_pairing': 'devicePairing',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[SessionAuthMethodEnum];
  @override
  final String wireName = 'SessionAuthMethodEnum';

  @override
  Object serialize(
    Serializers serializers,
    SessionAuthMethodEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  SessionAuthMethodEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => SessionAuthMethodEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$SessionClientComponentEnumSerializer
    implements PrimitiveSerializer<SessionClientComponentEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'app': 'app',
    'shareExt': 'share_ext',
    'console': 'console',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'app': 'app',
    'share_ext': 'shareExt',
    'console': 'console',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[SessionClientComponentEnum];
  @override
  final String wireName = 'SessionClientComponentEnum';

  @override
  Object serialize(
    Serializers serializers,
    SessionClientComponentEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  SessionClientComponentEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => SessionClientComponentEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$SessionStepUpMethodEnumSerializer
    implements PrimitiveSerializer<SessionStepUpMethodEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'passkey': 'passkey',
    'deviceKey': 'device_key',
    'passwordTotp': 'password_totp',
    'oidcReauth': 'oidc_reauth',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'passkey': 'passkey',
    'device_key': 'deviceKey',
    'password_totp': 'passwordTotp',
    'oidc_reauth': 'oidcReauth',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[SessionStepUpMethodEnum];
  @override
  final String wireName = 'SessionStepUpMethodEnum';

  @override
  Object serialize(
    Serializers serializers,
    SessionStepUpMethodEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  SessionStepUpMethodEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => SessionStepUpMethodEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$Session extends Session {
  @override
  final String id;
  @override
  final String userId;
  @override
  final String deviceId;
  @override
  final SessionAuthMethodEnum authMethod;
  @override
  final SessionClientComponentEnum? clientComponent;
  @override
  final DateTime createdAt;
  @override
  final DateTime expiresAt;
  @override
  final DateTime? stepUpAt;
  @override
  final SessionStepUpMethodEnum? stepUpMethod;
  @override
  final DateTime? lastSeenAt;
  @override
  final bool? current;

  factory _$Session([void Function(SessionBuilder)? updates]) =>
      (SessionBuilder()..update(updates))._build();

  _$Session._({
    required this.id,
    required this.userId,
    required this.deviceId,
    required this.authMethod,
    this.clientComponent,
    required this.createdAt,
    required this.expiresAt,
    this.stepUpAt,
    this.stepUpMethod,
    this.lastSeenAt,
    this.current,
  }) : super._();
  @override
  Session rebuild(void Function(SessionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SessionBuilder toBuilder() => SessionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Session &&
        id == other.id &&
        userId == other.userId &&
        deviceId == other.deviceId &&
        authMethod == other.authMethod &&
        clientComponent == other.clientComponent &&
        createdAt == other.createdAt &&
        expiresAt == other.expiresAt &&
        stepUpAt == other.stepUpAt &&
        stepUpMethod == other.stepUpMethod &&
        lastSeenAt == other.lastSeenAt &&
        current == other.current;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, deviceId.hashCode);
    _$hash = $jc(_$hash, authMethod.hashCode);
    _$hash = $jc(_$hash, clientComponent.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jc(_$hash, stepUpAt.hashCode);
    _$hash = $jc(_$hash, stepUpMethod.hashCode);
    _$hash = $jc(_$hash, lastSeenAt.hashCode);
    _$hash = $jc(_$hash, current.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Session')
          ..add('id', id)
          ..add('userId', userId)
          ..add('deviceId', deviceId)
          ..add('authMethod', authMethod)
          ..add('clientComponent', clientComponent)
          ..add('createdAt', createdAt)
          ..add('expiresAt', expiresAt)
          ..add('stepUpAt', stepUpAt)
          ..add('stepUpMethod', stepUpMethod)
          ..add('lastSeenAt', lastSeenAt)
          ..add('current', current))
        .toString();
  }
}

class SessionBuilder implements Builder<Session, SessionBuilder> {
  _$Session? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  String? _deviceId;
  String? get deviceId => _$this._deviceId;
  set deviceId(String? deviceId) => _$this._deviceId = deviceId;

  SessionAuthMethodEnum? _authMethod;
  SessionAuthMethodEnum? get authMethod => _$this._authMethod;
  set authMethod(SessionAuthMethodEnum? authMethod) =>
      _$this._authMethod = authMethod;

  SessionClientComponentEnum? _clientComponent;
  SessionClientComponentEnum? get clientComponent => _$this._clientComponent;
  set clientComponent(SessionClientComponentEnum? clientComponent) =>
      _$this._clientComponent = clientComponent;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  DateTime? _stepUpAt;
  DateTime? get stepUpAt => _$this._stepUpAt;
  set stepUpAt(DateTime? stepUpAt) => _$this._stepUpAt = stepUpAt;

  SessionStepUpMethodEnum? _stepUpMethod;
  SessionStepUpMethodEnum? get stepUpMethod => _$this._stepUpMethod;
  set stepUpMethod(SessionStepUpMethodEnum? stepUpMethod) =>
      _$this._stepUpMethod = stepUpMethod;

  DateTime? _lastSeenAt;
  DateTime? get lastSeenAt => _$this._lastSeenAt;
  set lastSeenAt(DateTime? lastSeenAt) => _$this._lastSeenAt = lastSeenAt;

  bool? _current;
  bool? get current => _$this._current;
  set current(bool? current) => _$this._current = current;

  SessionBuilder() {
    Session._defaults(this);
  }

  SessionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _userId = $v.userId;
      _deviceId = $v.deviceId;
      _authMethod = $v.authMethod;
      _clientComponent = $v.clientComponent;
      _createdAt = $v.createdAt;
      _expiresAt = $v.expiresAt;
      _stepUpAt = $v.stepUpAt;
      _stepUpMethod = $v.stepUpMethod;
      _lastSeenAt = $v.lastSeenAt;
      _current = $v.current;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Session other) {
    _$v = other as _$Session;
  }

  @override
  void update(void Function(SessionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Session build() => _build();

  _$Session _build() {
    final _$result =
        _$v ??
        _$Session._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'Session', 'id'),
          userId: BuiltValueNullFieldError.checkNotNull(
            userId,
            r'Session',
            'userId',
          ),
          deviceId: BuiltValueNullFieldError.checkNotNull(
            deviceId,
            r'Session',
            'deviceId',
          ),
          authMethod: BuiltValueNullFieldError.checkNotNull(
            authMethod,
            r'Session',
            'authMethod',
          ),
          clientComponent: clientComponent,
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'Session',
            'createdAt',
          ),
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'Session',
            'expiresAt',
          ),
          stepUpAt: stepUpAt,
          stepUpMethod: stepUpMethod,
          lastSeenAt: lastSeenAt,
          current: current,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

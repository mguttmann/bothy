// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'console_session.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ConsoleSessionStepUpMethodEnum _$consoleSessionStepUpMethodEnum_passkey =
    const ConsoleSessionStepUpMethodEnum._('passkey');
const ConsoleSessionStepUpMethodEnum
_$consoleSessionStepUpMethodEnum_passwordTotp =
    const ConsoleSessionStepUpMethodEnum._('passwordTotp');
const ConsoleSessionStepUpMethodEnum
_$consoleSessionStepUpMethodEnum_oidcReauth =
    const ConsoleSessionStepUpMethodEnum._('oidcReauth');
const ConsoleSessionStepUpMethodEnum
_$consoleSessionStepUpMethodEnum_unknownDefaultOpenApi =
    const ConsoleSessionStepUpMethodEnum._('unknownDefaultOpenApi');

ConsoleSessionStepUpMethodEnum _$consoleSessionStepUpMethodEnumValueOf(
  String name,
) {
  switch (name) {
    case 'passkey':
      return _$consoleSessionStepUpMethodEnum_passkey;
    case 'passwordTotp':
      return _$consoleSessionStepUpMethodEnum_passwordTotp;
    case 'oidcReauth':
      return _$consoleSessionStepUpMethodEnum_oidcReauth;
    case 'unknownDefaultOpenApi':
      return _$consoleSessionStepUpMethodEnum_unknownDefaultOpenApi;
    default:
      return _$consoleSessionStepUpMethodEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ConsoleSessionStepUpMethodEnum>
_$consoleSessionStepUpMethodEnumValues =
    BuiltSet<ConsoleSessionStepUpMethodEnum>(
      const <ConsoleSessionStepUpMethodEnum>[
        _$consoleSessionStepUpMethodEnum_passkey,
        _$consoleSessionStepUpMethodEnum_passwordTotp,
        _$consoleSessionStepUpMethodEnum_oidcReauth,
        _$consoleSessionStepUpMethodEnum_unknownDefaultOpenApi,
      ],
    );

const ConsoleSessionStepUpMethodsAvailableEnum
_$consoleSessionStepUpMethodsAvailableEnum_passkey =
    const ConsoleSessionStepUpMethodsAvailableEnum._('passkey');
const ConsoleSessionStepUpMethodsAvailableEnum
_$consoleSessionStepUpMethodsAvailableEnum_oidcReauth =
    const ConsoleSessionStepUpMethodsAvailableEnum._('oidcReauth');
const ConsoleSessionStepUpMethodsAvailableEnum
_$consoleSessionStepUpMethodsAvailableEnum_passwordTotp =
    const ConsoleSessionStepUpMethodsAvailableEnum._('passwordTotp');
const ConsoleSessionStepUpMethodsAvailableEnum
_$consoleSessionStepUpMethodsAvailableEnum_unknownDefaultOpenApi =
    const ConsoleSessionStepUpMethodsAvailableEnum._('unknownDefaultOpenApi');

ConsoleSessionStepUpMethodsAvailableEnum
_$consoleSessionStepUpMethodsAvailableEnumValueOf(String name) {
  switch (name) {
    case 'passkey':
      return _$consoleSessionStepUpMethodsAvailableEnum_passkey;
    case 'oidcReauth':
      return _$consoleSessionStepUpMethodsAvailableEnum_oidcReauth;
    case 'passwordTotp':
      return _$consoleSessionStepUpMethodsAvailableEnum_passwordTotp;
    case 'unknownDefaultOpenApi':
      return _$consoleSessionStepUpMethodsAvailableEnum_unknownDefaultOpenApi;
    default:
      return _$consoleSessionStepUpMethodsAvailableEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ConsoleSessionStepUpMethodsAvailableEnum>
_$consoleSessionStepUpMethodsAvailableEnumValues =
    BuiltSet<ConsoleSessionStepUpMethodsAvailableEnum>(
      const <ConsoleSessionStepUpMethodsAvailableEnum>[
        _$consoleSessionStepUpMethodsAvailableEnum_passkey,
        _$consoleSessionStepUpMethodsAvailableEnum_oidcReauth,
        _$consoleSessionStepUpMethodsAvailableEnum_passwordTotp,
        _$consoleSessionStepUpMethodsAvailableEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<ConsoleSessionStepUpMethodEnum>
_$consoleSessionStepUpMethodEnumSerializer =
    _$ConsoleSessionStepUpMethodEnumSerializer();
Serializer<ConsoleSessionStepUpMethodsAvailableEnum>
_$consoleSessionStepUpMethodsAvailableEnumSerializer =
    _$ConsoleSessionStepUpMethodsAvailableEnumSerializer();

class _$ConsoleSessionStepUpMethodEnumSerializer
    implements PrimitiveSerializer<ConsoleSessionStepUpMethodEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'passkey': 'passkey',
    'passwordTotp': 'password_totp',
    'oidcReauth': 'oidc_reauth',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'passkey': 'passkey',
    'password_totp': 'passwordTotp',
    'oidc_reauth': 'oidcReauth',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ConsoleSessionStepUpMethodEnum];
  @override
  final String wireName = 'ConsoleSessionStepUpMethodEnum';

  @override
  Object serialize(
    Serializers serializers,
    ConsoleSessionStepUpMethodEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ConsoleSessionStepUpMethodEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ConsoleSessionStepUpMethodEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ConsoleSessionStepUpMethodsAvailableEnumSerializer
    implements PrimitiveSerializer<ConsoleSessionStepUpMethodsAvailableEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'passkey': 'passkey',
    'oidcReauth': 'oidc_reauth',
    'passwordTotp': 'password_totp',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'passkey': 'passkey',
    'oidc_reauth': 'oidcReauth',
    'password_totp': 'passwordTotp',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    ConsoleSessionStepUpMethodsAvailableEnum,
  ];
  @override
  final String wireName = 'ConsoleSessionStepUpMethodsAvailableEnum';

  @override
  Object serialize(
    Serializers serializers,
    ConsoleSessionStepUpMethodsAvailableEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ConsoleSessionStepUpMethodsAvailableEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ConsoleSessionStepUpMethodsAvailableEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ConsoleSession extends ConsoleSession {
  @override
  final String userId;
  @override
  final String displayName;
  @override
  final DateTime expiresAt;
  @override
  final int idleTimeoutSeconds;
  @override
  final DateTime? stepUpAt;
  @override
  final ConsoleSessionStepUpMethodEnum? stepUpMethod;
  @override
  final DateTime? stepUpValidUntil;
  @override
  final BuiltList<ConsoleSessionStepUpMethodsAvailableEnum>
  stepUpMethodsAvailable;
  @override
  final ConsoleSessionViewAs? viewAs;

  factory _$ConsoleSession([void Function(ConsoleSessionBuilder)? updates]) =>
      (ConsoleSessionBuilder()..update(updates))._build();

  _$ConsoleSession._({
    required this.userId,
    required this.displayName,
    required this.expiresAt,
    required this.idleTimeoutSeconds,
    this.stepUpAt,
    this.stepUpMethod,
    this.stepUpValidUntil,
    required this.stepUpMethodsAvailable,
    this.viewAs,
  }) : super._();
  @override
  ConsoleSession rebuild(void Function(ConsoleSessionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ConsoleSessionBuilder toBuilder() => ConsoleSessionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ConsoleSession &&
        userId == other.userId &&
        displayName == other.displayName &&
        expiresAt == other.expiresAt &&
        idleTimeoutSeconds == other.idleTimeoutSeconds &&
        stepUpAt == other.stepUpAt &&
        stepUpMethod == other.stepUpMethod &&
        stepUpValidUntil == other.stepUpValidUntil &&
        stepUpMethodsAvailable == other.stepUpMethodsAvailable &&
        viewAs == other.viewAs;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, displayName.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jc(_$hash, idleTimeoutSeconds.hashCode);
    _$hash = $jc(_$hash, stepUpAt.hashCode);
    _$hash = $jc(_$hash, stepUpMethod.hashCode);
    _$hash = $jc(_$hash, stepUpValidUntil.hashCode);
    _$hash = $jc(_$hash, stepUpMethodsAvailable.hashCode);
    _$hash = $jc(_$hash, viewAs.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ConsoleSession')
          ..add('userId', userId)
          ..add('displayName', displayName)
          ..add('expiresAt', expiresAt)
          ..add('idleTimeoutSeconds', idleTimeoutSeconds)
          ..add('stepUpAt', stepUpAt)
          ..add('stepUpMethod', stepUpMethod)
          ..add('stepUpValidUntil', stepUpValidUntil)
          ..add('stepUpMethodsAvailable', stepUpMethodsAvailable)
          ..add('viewAs', viewAs))
        .toString();
  }
}

class ConsoleSessionBuilder
    implements Builder<ConsoleSession, ConsoleSessionBuilder> {
  _$ConsoleSession? _$v;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  String? _displayName;
  String? get displayName => _$this._displayName;
  set displayName(String? displayName) => _$this._displayName = displayName;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  int? _idleTimeoutSeconds;
  int? get idleTimeoutSeconds => _$this._idleTimeoutSeconds;
  set idleTimeoutSeconds(int? idleTimeoutSeconds) =>
      _$this._idleTimeoutSeconds = idleTimeoutSeconds;

  DateTime? _stepUpAt;
  DateTime? get stepUpAt => _$this._stepUpAt;
  set stepUpAt(DateTime? stepUpAt) => _$this._stepUpAt = stepUpAt;

  ConsoleSessionStepUpMethodEnum? _stepUpMethod;
  ConsoleSessionStepUpMethodEnum? get stepUpMethod => _$this._stepUpMethod;
  set stepUpMethod(ConsoleSessionStepUpMethodEnum? stepUpMethod) =>
      _$this._stepUpMethod = stepUpMethod;

  DateTime? _stepUpValidUntil;
  DateTime? get stepUpValidUntil => _$this._stepUpValidUntil;
  set stepUpValidUntil(DateTime? stepUpValidUntil) =>
      _$this._stepUpValidUntil = stepUpValidUntil;

  ListBuilder<ConsoleSessionStepUpMethodsAvailableEnum>?
  _stepUpMethodsAvailable;
  ListBuilder<ConsoleSessionStepUpMethodsAvailableEnum>
  get stepUpMethodsAvailable => _$this._stepUpMethodsAvailable ??=
      ListBuilder<ConsoleSessionStepUpMethodsAvailableEnum>();
  set stepUpMethodsAvailable(
    ListBuilder<ConsoleSessionStepUpMethodsAvailableEnum>?
    stepUpMethodsAvailable,
  ) => _$this._stepUpMethodsAvailable = stepUpMethodsAvailable;

  ConsoleSessionViewAsBuilder? _viewAs;
  ConsoleSessionViewAsBuilder get viewAs =>
      _$this._viewAs ??= ConsoleSessionViewAsBuilder();
  set viewAs(ConsoleSessionViewAsBuilder? viewAs) => _$this._viewAs = viewAs;

  ConsoleSessionBuilder() {
    ConsoleSession._defaults(this);
  }

  ConsoleSessionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _userId = $v.userId;
      _displayName = $v.displayName;
      _expiresAt = $v.expiresAt;
      _idleTimeoutSeconds = $v.idleTimeoutSeconds;
      _stepUpAt = $v.stepUpAt;
      _stepUpMethod = $v.stepUpMethod;
      _stepUpValidUntil = $v.stepUpValidUntil;
      _stepUpMethodsAvailable = $v.stepUpMethodsAvailable.toBuilder();
      _viewAs = $v.viewAs?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ConsoleSession other) {
    _$v = other as _$ConsoleSession;
  }

  @override
  void update(void Function(ConsoleSessionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ConsoleSession build() => _build();

  _$ConsoleSession _build() {
    _$ConsoleSession _$result;
    try {
      _$result =
          _$v ??
          _$ConsoleSession._(
            userId: BuiltValueNullFieldError.checkNotNull(
              userId,
              r'ConsoleSession',
              'userId',
            ),
            displayName: BuiltValueNullFieldError.checkNotNull(
              displayName,
              r'ConsoleSession',
              'displayName',
            ),
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'ConsoleSession',
              'expiresAt',
            ),
            idleTimeoutSeconds: BuiltValueNullFieldError.checkNotNull(
              idleTimeoutSeconds,
              r'ConsoleSession',
              'idleTimeoutSeconds',
            ),
            stepUpAt: stepUpAt,
            stepUpMethod: stepUpMethod,
            stepUpValidUntil: stepUpValidUntil,
            stepUpMethodsAvailable: stepUpMethodsAvailable.build(),
            viewAs: _viewAs?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'stepUpMethodsAvailable';
        stepUpMethodsAvailable.build();
        _$failedField = 'viewAs';
        _viewAs?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ConsoleSession',
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

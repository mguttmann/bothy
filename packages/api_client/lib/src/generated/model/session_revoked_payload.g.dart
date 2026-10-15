// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'session_revoked_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const SessionRevokedPayloadReasonEnum _$sessionRevokedPayloadReasonEnum_logout =
    const SessionRevokedPayloadReasonEnum._('logout');
const SessionRevokedPayloadReasonEnum _$sessionRevokedPayloadReasonEnum_admin =
    const SessionRevokedPayloadReasonEnum._('admin');
const SessionRevokedPayloadReasonEnum
_$sessionRevokedPayloadReasonEnum_deviceRevoked =
    const SessionRevokedPayloadReasonEnum._('deviceRevoked');
const SessionRevokedPayloadReasonEnum
_$sessionRevokedPayloadReasonEnum_refreshReuse =
    const SessionRevokedPayloadReasonEnum._('refreshReuse');
const SessionRevokedPayloadReasonEnum
_$sessionRevokedPayloadReasonEnum_userDisabled =
    const SessionRevokedPayloadReasonEnum._('userDisabled');
const SessionRevokedPayloadReasonEnum
_$sessionRevokedPayloadReasonEnum_expired =
    const SessionRevokedPayloadReasonEnum._('expired');
const SessionRevokedPayloadReasonEnum
_$sessionRevokedPayloadReasonEnum_inactive =
    const SessionRevokedPayloadReasonEnum._('inactive');
const SessionRevokedPayloadReasonEnum
_$sessionRevokedPayloadReasonEnum_stepUpFailed =
    const SessionRevokedPayloadReasonEnum._('stepUpFailed');
const SessionRevokedPayloadReasonEnum
_$sessionRevokedPayloadReasonEnum_trustedHeaderMismatch =
    const SessionRevokedPayloadReasonEnum._('trustedHeaderMismatch');
const SessionRevokedPayloadReasonEnum
_$sessionRevokedPayloadReasonEnum_unknownDefaultOpenApi =
    const SessionRevokedPayloadReasonEnum._('unknownDefaultOpenApi');

SessionRevokedPayloadReasonEnum _$sessionRevokedPayloadReasonEnumValueOf(
  String name,
) {
  switch (name) {
    case 'logout':
      return _$sessionRevokedPayloadReasonEnum_logout;
    case 'admin':
      return _$sessionRevokedPayloadReasonEnum_admin;
    case 'deviceRevoked':
      return _$sessionRevokedPayloadReasonEnum_deviceRevoked;
    case 'refreshReuse':
      return _$sessionRevokedPayloadReasonEnum_refreshReuse;
    case 'userDisabled':
      return _$sessionRevokedPayloadReasonEnum_userDisabled;
    case 'expired':
      return _$sessionRevokedPayloadReasonEnum_expired;
    case 'inactive':
      return _$sessionRevokedPayloadReasonEnum_inactive;
    case 'stepUpFailed':
      return _$sessionRevokedPayloadReasonEnum_stepUpFailed;
    case 'trustedHeaderMismatch':
      return _$sessionRevokedPayloadReasonEnum_trustedHeaderMismatch;
    case 'unknownDefaultOpenApi':
      return _$sessionRevokedPayloadReasonEnum_unknownDefaultOpenApi;
    default:
      return _$sessionRevokedPayloadReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<SessionRevokedPayloadReasonEnum>
_$sessionRevokedPayloadReasonEnumValues =
    BuiltSet<SessionRevokedPayloadReasonEnum>(
      const <SessionRevokedPayloadReasonEnum>[
        _$sessionRevokedPayloadReasonEnum_logout,
        _$sessionRevokedPayloadReasonEnum_admin,
        _$sessionRevokedPayloadReasonEnum_deviceRevoked,
        _$sessionRevokedPayloadReasonEnum_refreshReuse,
        _$sessionRevokedPayloadReasonEnum_userDisabled,
        _$sessionRevokedPayloadReasonEnum_expired,
        _$sessionRevokedPayloadReasonEnum_inactive,
        _$sessionRevokedPayloadReasonEnum_stepUpFailed,
        _$sessionRevokedPayloadReasonEnum_trustedHeaderMismatch,
        _$sessionRevokedPayloadReasonEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<SessionRevokedPayloadReasonEnum>
_$sessionRevokedPayloadReasonEnumSerializer =
    _$SessionRevokedPayloadReasonEnumSerializer();

class _$SessionRevokedPayloadReasonEnumSerializer
    implements PrimitiveSerializer<SessionRevokedPayloadReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'logout': 'logout',
    'admin': 'admin',
    'deviceRevoked': 'device_revoked',
    'refreshReuse': 'refresh_reuse',
    'userDisabled': 'user_disabled',
    'expired': 'expired',
    'inactive': 'inactive',
    'stepUpFailed': 'step_up_failed',
    'trustedHeaderMismatch': 'trusted_header_mismatch',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'logout': 'logout',
    'admin': 'admin',
    'device_revoked': 'deviceRevoked',
    'refresh_reuse': 'refreshReuse',
    'user_disabled': 'userDisabled',
    'expired': 'expired',
    'inactive': 'inactive',
    'step_up_failed': 'stepUpFailed',
    'trusted_header_mismatch': 'trustedHeaderMismatch',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[SessionRevokedPayloadReasonEnum];
  @override
  final String wireName = 'SessionRevokedPayloadReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    SessionRevokedPayloadReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  SessionRevokedPayloadReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => SessionRevokedPayloadReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$SessionRevokedPayload extends SessionRevokedPayload {
  @override
  final String sessionId;
  @override
  final SessionRevokedPayloadReasonEnum reason;

  factory _$SessionRevokedPayload([
    void Function(SessionRevokedPayloadBuilder)? updates,
  ]) => (SessionRevokedPayloadBuilder()..update(updates))._build();

  _$SessionRevokedPayload._({required this.sessionId, required this.reason})
    : super._();
  @override
  SessionRevokedPayload rebuild(
    void Function(SessionRevokedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  SessionRevokedPayloadBuilder toBuilder() =>
      SessionRevokedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SessionRevokedPayload &&
        sessionId == other.sessionId &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SessionRevokedPayload')
          ..add('sessionId', sessionId)
          ..add('reason', reason))
        .toString();
  }
}

class SessionRevokedPayloadBuilder
    implements Builder<SessionRevokedPayload, SessionRevokedPayloadBuilder> {
  _$SessionRevokedPayload? _$v;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  SessionRevokedPayloadReasonEnum? _reason;
  SessionRevokedPayloadReasonEnum? get reason => _$this._reason;
  set reason(SessionRevokedPayloadReasonEnum? reason) =>
      _$this._reason = reason;

  SessionRevokedPayloadBuilder() {
    SessionRevokedPayload._defaults(this);
  }

  SessionRevokedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _sessionId = $v.sessionId;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SessionRevokedPayload other) {
    _$v = other as _$SessionRevokedPayload;
  }

  @override
  void update(void Function(SessionRevokedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SessionRevokedPayload build() => _build();

  _$SessionRevokedPayload _build() {
    final _$result =
        _$v ??
        _$SessionRevokedPayload._(
          sessionId: BuiltValueNullFieldError.checkNotNull(
            sessionId,
            r'SessionRevokedPayload',
            'sessionId',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'SessionRevokedPayload',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

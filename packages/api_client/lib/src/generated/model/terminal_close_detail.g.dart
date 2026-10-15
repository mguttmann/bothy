// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'terminal_close_detail.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const TerminalCloseDetail _$tokenInvalid = const TerminalCloseDetail._(
  'tokenInvalid',
);
const TerminalCloseDetail _$tokenExpired = const TerminalCloseDetail._(
  'tokenExpired',
);
const TerminalCloseDetail _$tokenReplayed = const TerminalCloseDetail._(
  'tokenReplayed',
);
const TerminalCloseDetail _$handshakeFailed = const TerminalCloseDetail._(
  'handshakeFailed',
);
const TerminalCloseDetail _$profileUnknown = const TerminalCloseDetail._(
  'profileUnknown',
);
const TerminalCloseDetail _$sessionUserUnknown = const TerminalCloseDetail._(
  'sessionUserUnknown',
);
const TerminalCloseDetail _$sessionUserMismatch = const TerminalCloseDetail._(
  'sessionUserMismatch',
);
const TerminalCloseDetail _$serverHostOwnerOnly = const TerminalCloseDetail._(
  'serverHostOwnerOnly',
);
const TerminalCloseDetail _$localConfirmDenied = const TerminalCloseDetail._(
  'localConfirmDenied',
);
const TerminalCloseDetail _$subscriptionLoginDetected =
    const TerminalCloseDetail._('subscriptionLoginDetected');
const TerminalCloseDetail _$helperUntrusted = const TerminalCloseDetail._(
  'helperUntrusted',
);
const TerminalCloseDetail _$sharedLocation = const TerminalCloseDetail._(
  'sharedLocation',
);
const TerminalCloseDetail _$ptyFailed = const TerminalCloseDetail._(
  'ptyFailed',
);
const TerminalCloseDetail _$openTimeout = const TerminalCloseDetail._(
  'openTimeout',
);
const TerminalCloseDetail _$unknownDefaultOpenApi = const TerminalCloseDetail._(
  'unknownDefaultOpenApi',
);

TerminalCloseDetail _$valueOf(String name) {
  switch (name) {
    case 'tokenInvalid':
      return _$tokenInvalid;
    case 'tokenExpired':
      return _$tokenExpired;
    case 'tokenReplayed':
      return _$tokenReplayed;
    case 'handshakeFailed':
      return _$handshakeFailed;
    case 'profileUnknown':
      return _$profileUnknown;
    case 'sessionUserUnknown':
      return _$sessionUserUnknown;
    case 'sessionUserMismatch':
      return _$sessionUserMismatch;
    case 'serverHostOwnerOnly':
      return _$serverHostOwnerOnly;
    case 'localConfirmDenied':
      return _$localConfirmDenied;
    case 'subscriptionLoginDetected':
      return _$subscriptionLoginDetected;
    case 'helperUntrusted':
      return _$helperUntrusted;
    case 'sharedLocation':
      return _$sharedLocation;
    case 'ptyFailed':
      return _$ptyFailed;
    case 'openTimeout':
      return _$openTimeout;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<TerminalCloseDetail> _$values = BuiltSet<TerminalCloseDetail>(
  const <TerminalCloseDetail>[
    _$tokenInvalid,
    _$tokenExpired,
    _$tokenReplayed,
    _$handshakeFailed,
    _$profileUnknown,
    _$sessionUserUnknown,
    _$sessionUserMismatch,
    _$serverHostOwnerOnly,
    _$localConfirmDenied,
    _$subscriptionLoginDetected,
    _$helperUntrusted,
    _$sharedLocation,
    _$ptyFailed,
    _$openTimeout,
    _$unknownDefaultOpenApi,
  ],
);

class _$TerminalCloseDetailMeta {
  const _$TerminalCloseDetailMeta();
  TerminalCloseDetail get tokenInvalid => _$tokenInvalid;
  TerminalCloseDetail get tokenExpired => _$tokenExpired;
  TerminalCloseDetail get tokenReplayed => _$tokenReplayed;
  TerminalCloseDetail get handshakeFailed => _$handshakeFailed;
  TerminalCloseDetail get profileUnknown => _$profileUnknown;
  TerminalCloseDetail get sessionUserUnknown => _$sessionUserUnknown;
  TerminalCloseDetail get sessionUserMismatch => _$sessionUserMismatch;
  TerminalCloseDetail get serverHostOwnerOnly => _$serverHostOwnerOnly;
  TerminalCloseDetail get localConfirmDenied => _$localConfirmDenied;
  TerminalCloseDetail get subscriptionLoginDetected =>
      _$subscriptionLoginDetected;
  TerminalCloseDetail get helperUntrusted => _$helperUntrusted;
  TerminalCloseDetail get sharedLocation => _$sharedLocation;
  TerminalCloseDetail get ptyFailed => _$ptyFailed;
  TerminalCloseDetail get openTimeout => _$openTimeout;
  TerminalCloseDetail get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  TerminalCloseDetail valueOf(String name) => _$valueOf(name);
  BuiltSet<TerminalCloseDetail> get values => _$values;
}

mixin _$TerminalCloseDetailMixin {
  // ignore: non_constant_identifier_names
  _$TerminalCloseDetailMeta get TerminalCloseDetail =>
      const _$TerminalCloseDetailMeta();
}

Serializer<TerminalCloseDetail> _$terminalCloseDetailSerializer =
    _$TerminalCloseDetailSerializer();

class _$TerminalCloseDetailSerializer
    implements PrimitiveSerializer<TerminalCloseDetail> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'tokenInvalid': 'token_invalid',
    'tokenExpired': 'token_expired',
    'tokenReplayed': 'token_replayed',
    'handshakeFailed': 'handshake_failed',
    'profileUnknown': 'profile_unknown',
    'sessionUserUnknown': 'session_user_unknown',
    'sessionUserMismatch': 'session_user_mismatch',
    'serverHostOwnerOnly': 'server_host_owner_only',
    'localConfirmDenied': 'local_confirm_denied',
    'subscriptionLoginDetected': 'subscription_login_detected',
    'helperUntrusted': 'helper_untrusted',
    'sharedLocation': 'shared_location',
    'ptyFailed': 'pty_failed',
    'openTimeout': 'open_timeout',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'token_invalid': 'tokenInvalid',
    'token_expired': 'tokenExpired',
    'token_replayed': 'tokenReplayed',
    'handshake_failed': 'handshakeFailed',
    'profile_unknown': 'profileUnknown',
    'session_user_unknown': 'sessionUserUnknown',
    'session_user_mismatch': 'sessionUserMismatch',
    'server_host_owner_only': 'serverHostOwnerOnly',
    'local_confirm_denied': 'localConfirmDenied',
    'subscription_login_detected': 'subscriptionLoginDetected',
    'helper_untrusted': 'helperUntrusted',
    'shared_location': 'sharedLocation',
    'pty_failed': 'ptyFailed',
    'open_timeout': 'openTimeout',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[TerminalCloseDetail];
  @override
  final String wireName = 'TerminalCloseDetail';

  @override
  Object serialize(
    Serializers serializers,
    TerminalCloseDetail object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  TerminalCloseDetail deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => TerminalCloseDetail.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

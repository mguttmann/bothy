// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'terminal_session_grant.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const TerminalSessionGrantNoiseProtocolEnum
_$terminalSessionGrantNoiseProtocolEnum_noiseNK25519ChaChaPolySHA256 =
    const TerminalSessionGrantNoiseProtocolEnum._(
      'noiseNK25519ChaChaPolySHA256',
    );
const TerminalSessionGrantNoiseProtocolEnum
_$terminalSessionGrantNoiseProtocolEnum_unknownDefaultOpenApi =
    const TerminalSessionGrantNoiseProtocolEnum._('unknownDefaultOpenApi');

TerminalSessionGrantNoiseProtocolEnum
_$terminalSessionGrantNoiseProtocolEnumValueOf(String name) {
  switch (name) {
    case 'noiseNK25519ChaChaPolySHA256':
      return _$terminalSessionGrantNoiseProtocolEnum_noiseNK25519ChaChaPolySHA256;
    case 'unknownDefaultOpenApi':
      return _$terminalSessionGrantNoiseProtocolEnum_unknownDefaultOpenApi;
    default:
      return _$terminalSessionGrantNoiseProtocolEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<TerminalSessionGrantNoiseProtocolEnum>
_$terminalSessionGrantNoiseProtocolEnumValues =
    BuiltSet<TerminalSessionGrantNoiseProtocolEnum>(
      const <TerminalSessionGrantNoiseProtocolEnum>[
        _$terminalSessionGrantNoiseProtocolEnum_noiseNK25519ChaChaPolySHA256,
        _$terminalSessionGrantNoiseProtocolEnum_unknownDefaultOpenApi,
      ],
    );

const TerminalSessionGrantPrologueVersionEnum
_$terminalSessionGrantPrologueVersionEnum_bothyTerminalSlash1 =
    const TerminalSessionGrantPrologueVersionEnum._('bothyTerminalSlash1');
const TerminalSessionGrantPrologueVersionEnum
_$terminalSessionGrantPrologueVersionEnum_unknownDefaultOpenApi =
    const TerminalSessionGrantPrologueVersionEnum._('unknownDefaultOpenApi');

TerminalSessionGrantPrologueVersionEnum
_$terminalSessionGrantPrologueVersionEnumValueOf(String name) {
  switch (name) {
    case 'bothyTerminalSlash1':
      return _$terminalSessionGrantPrologueVersionEnum_bothyTerminalSlash1;
    case 'unknownDefaultOpenApi':
      return _$terminalSessionGrantPrologueVersionEnum_unknownDefaultOpenApi;
    default:
      return _$terminalSessionGrantPrologueVersionEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<TerminalSessionGrantPrologueVersionEnum>
_$terminalSessionGrantPrologueVersionEnumValues =
    BuiltSet<TerminalSessionGrantPrologueVersionEnum>(
      const <TerminalSessionGrantPrologueVersionEnum>[
        _$terminalSessionGrantPrologueVersionEnum_bothyTerminalSlash1,
        _$terminalSessionGrantPrologueVersionEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<TerminalSessionGrantNoiseProtocolEnum>
_$terminalSessionGrantNoiseProtocolEnumSerializer =
    _$TerminalSessionGrantNoiseProtocolEnumSerializer();
Serializer<TerminalSessionGrantPrologueVersionEnum>
_$terminalSessionGrantPrologueVersionEnumSerializer =
    _$TerminalSessionGrantPrologueVersionEnumSerializer();

class _$TerminalSessionGrantNoiseProtocolEnumSerializer
    implements PrimitiveSerializer<TerminalSessionGrantNoiseProtocolEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'noiseNK25519ChaChaPolySHA256': 'Noise_NK_25519_ChaChaPoly_SHA256',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'Noise_NK_25519_ChaChaPoly_SHA256': 'noiseNK25519ChaChaPolySHA256',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    TerminalSessionGrantNoiseProtocolEnum,
  ];
  @override
  final String wireName = 'TerminalSessionGrantNoiseProtocolEnum';

  @override
  Object serialize(
    Serializers serializers,
    TerminalSessionGrantNoiseProtocolEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  TerminalSessionGrantNoiseProtocolEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => TerminalSessionGrantNoiseProtocolEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$TerminalSessionGrantPrologueVersionEnumSerializer
    implements PrimitiveSerializer<TerminalSessionGrantPrologueVersionEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'bothyTerminalSlash1': 'bothy-terminal/1',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'bothy-terminal/1': 'bothyTerminalSlash1',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    TerminalSessionGrantPrologueVersionEnum,
  ];
  @override
  final String wireName = 'TerminalSessionGrantPrologueVersionEnum';

  @override
  Object serialize(
    Serializers serializers,
    TerminalSessionGrantPrologueVersionEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  TerminalSessionGrantPrologueVersionEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => TerminalSessionGrantPrologueVersionEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$TerminalSessionGrant extends TerminalSessionGrant {
  @override
  final TerminalSession session;
  @override
  final String sessionToken;
  @override
  final DateTime sessionTokenExpiresAt;
  @override
  final String streamPath;
  @override
  final String noisePublicKey;
  @override
  final String keyFingerprint;
  @override
  final int keyGeneration;
  @override
  final TerminalSessionGrantNoiseProtocolEnum noiseProtocol;
  @override
  final TerminalSessionGrantPrologueVersionEnum prologueVersion;

  factory _$TerminalSessionGrant([
    void Function(TerminalSessionGrantBuilder)? updates,
  ]) => (TerminalSessionGrantBuilder()..update(updates))._build();

  _$TerminalSessionGrant._({
    required this.session,
    required this.sessionToken,
    required this.sessionTokenExpiresAt,
    required this.streamPath,
    required this.noisePublicKey,
    required this.keyFingerprint,
    required this.keyGeneration,
    required this.noiseProtocol,
    required this.prologueVersion,
  }) : super._();
  @override
  TerminalSessionGrant rebuild(
    void Function(TerminalSessionGrantBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  TerminalSessionGrantBuilder toBuilder() =>
      TerminalSessionGrantBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TerminalSessionGrant &&
        session == other.session &&
        sessionToken == other.sessionToken &&
        sessionTokenExpiresAt == other.sessionTokenExpiresAt &&
        streamPath == other.streamPath &&
        noisePublicKey == other.noisePublicKey &&
        keyFingerprint == other.keyFingerprint &&
        keyGeneration == other.keyGeneration &&
        noiseProtocol == other.noiseProtocol &&
        prologueVersion == other.prologueVersion;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, session.hashCode);
    _$hash = $jc(_$hash, sessionToken.hashCode);
    _$hash = $jc(_$hash, sessionTokenExpiresAt.hashCode);
    _$hash = $jc(_$hash, streamPath.hashCode);
    _$hash = $jc(_$hash, noisePublicKey.hashCode);
    _$hash = $jc(_$hash, keyFingerprint.hashCode);
    _$hash = $jc(_$hash, keyGeneration.hashCode);
    _$hash = $jc(_$hash, noiseProtocol.hashCode);
    _$hash = $jc(_$hash, prologueVersion.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TerminalSessionGrant')
          ..add('session', session)
          ..add('sessionToken', sessionToken)
          ..add('sessionTokenExpiresAt', sessionTokenExpiresAt)
          ..add('streamPath', streamPath)
          ..add('noisePublicKey', noisePublicKey)
          ..add('keyFingerprint', keyFingerprint)
          ..add('keyGeneration', keyGeneration)
          ..add('noiseProtocol', noiseProtocol)
          ..add('prologueVersion', prologueVersion))
        .toString();
  }
}

class TerminalSessionGrantBuilder
    implements Builder<TerminalSessionGrant, TerminalSessionGrantBuilder> {
  _$TerminalSessionGrant? _$v;

  TerminalSessionBuilder? _session;
  TerminalSessionBuilder get session =>
      _$this._session ??= TerminalSessionBuilder();
  set session(TerminalSessionBuilder? session) => _$this._session = session;

  String? _sessionToken;
  String? get sessionToken => _$this._sessionToken;
  set sessionToken(String? sessionToken) => _$this._sessionToken = sessionToken;

  DateTime? _sessionTokenExpiresAt;
  DateTime? get sessionTokenExpiresAt => _$this._sessionTokenExpiresAt;
  set sessionTokenExpiresAt(DateTime? sessionTokenExpiresAt) =>
      _$this._sessionTokenExpiresAt = sessionTokenExpiresAt;

  String? _streamPath;
  String? get streamPath => _$this._streamPath;
  set streamPath(String? streamPath) => _$this._streamPath = streamPath;

  String? _noisePublicKey;
  String? get noisePublicKey => _$this._noisePublicKey;
  set noisePublicKey(String? noisePublicKey) =>
      _$this._noisePublicKey = noisePublicKey;

  String? _keyFingerprint;
  String? get keyFingerprint => _$this._keyFingerprint;
  set keyFingerprint(String? keyFingerprint) =>
      _$this._keyFingerprint = keyFingerprint;

  int? _keyGeneration;
  int? get keyGeneration => _$this._keyGeneration;
  set keyGeneration(int? keyGeneration) =>
      _$this._keyGeneration = keyGeneration;

  TerminalSessionGrantNoiseProtocolEnum? _noiseProtocol;
  TerminalSessionGrantNoiseProtocolEnum? get noiseProtocol =>
      _$this._noiseProtocol;
  set noiseProtocol(TerminalSessionGrantNoiseProtocolEnum? noiseProtocol) =>
      _$this._noiseProtocol = noiseProtocol;

  TerminalSessionGrantPrologueVersionEnum? _prologueVersion;
  TerminalSessionGrantPrologueVersionEnum? get prologueVersion =>
      _$this._prologueVersion;
  set prologueVersion(
    TerminalSessionGrantPrologueVersionEnum? prologueVersion,
  ) => _$this._prologueVersion = prologueVersion;

  TerminalSessionGrantBuilder() {
    TerminalSessionGrant._defaults(this);
  }

  TerminalSessionGrantBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _session = $v.session.toBuilder();
      _sessionToken = $v.sessionToken;
      _sessionTokenExpiresAt = $v.sessionTokenExpiresAt;
      _streamPath = $v.streamPath;
      _noisePublicKey = $v.noisePublicKey;
      _keyFingerprint = $v.keyFingerprint;
      _keyGeneration = $v.keyGeneration;
      _noiseProtocol = $v.noiseProtocol;
      _prologueVersion = $v.prologueVersion;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TerminalSessionGrant other) {
    _$v = other as _$TerminalSessionGrant;
  }

  @override
  void update(void Function(TerminalSessionGrantBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TerminalSessionGrant build() => _build();

  _$TerminalSessionGrant _build() {
    _$TerminalSessionGrant _$result;
    try {
      _$result =
          _$v ??
          _$TerminalSessionGrant._(
            session: session.build(),
            sessionToken: BuiltValueNullFieldError.checkNotNull(
              sessionToken,
              r'TerminalSessionGrant',
              'sessionToken',
            ),
            sessionTokenExpiresAt: BuiltValueNullFieldError.checkNotNull(
              sessionTokenExpiresAt,
              r'TerminalSessionGrant',
              'sessionTokenExpiresAt',
            ),
            streamPath: BuiltValueNullFieldError.checkNotNull(
              streamPath,
              r'TerminalSessionGrant',
              'streamPath',
            ),
            noisePublicKey: BuiltValueNullFieldError.checkNotNull(
              noisePublicKey,
              r'TerminalSessionGrant',
              'noisePublicKey',
            ),
            keyFingerprint: BuiltValueNullFieldError.checkNotNull(
              keyFingerprint,
              r'TerminalSessionGrant',
              'keyFingerprint',
            ),
            keyGeneration: BuiltValueNullFieldError.checkNotNull(
              keyGeneration,
              r'TerminalSessionGrant',
              'keyGeneration',
            ),
            noiseProtocol: BuiltValueNullFieldError.checkNotNull(
              noiseProtocol,
              r'TerminalSessionGrant',
              'noiseProtocol',
            ),
            prologueVersion: BuiltValueNullFieldError.checkNotNull(
              prologueVersion,
              r'TerminalSessionGrant',
              'prologueVersion',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'session';
        session.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'TerminalSessionGrant',
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

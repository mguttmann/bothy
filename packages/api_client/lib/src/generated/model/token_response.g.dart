// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'token_response.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const TokenResponseTokenTypeEnum _$tokenResponseTokenTypeEnum_dPoP =
    const TokenResponseTokenTypeEnum._('dPoP');
const TokenResponseTokenTypeEnum
_$tokenResponseTokenTypeEnum_unknownDefaultOpenApi =
    const TokenResponseTokenTypeEnum._('unknownDefaultOpenApi');

TokenResponseTokenTypeEnum _$tokenResponseTokenTypeEnumValueOf(String name) {
  switch (name) {
    case 'dPoP':
      return _$tokenResponseTokenTypeEnum_dPoP;
    case 'unknownDefaultOpenApi':
      return _$tokenResponseTokenTypeEnum_unknownDefaultOpenApi;
    default:
      return _$tokenResponseTokenTypeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<TokenResponseTokenTypeEnum> _$tokenResponseTokenTypeEnumValues =
    BuiltSet<TokenResponseTokenTypeEnum>(const <TokenResponseTokenTypeEnum>[
      _$tokenResponseTokenTypeEnum_dPoP,
      _$tokenResponseTokenTypeEnum_unknownDefaultOpenApi,
    ]);

Serializer<TokenResponseTokenTypeEnum> _$tokenResponseTokenTypeEnumSerializer =
    _$TokenResponseTokenTypeEnumSerializer();

class _$TokenResponseTokenTypeEnumSerializer
    implements PrimitiveSerializer<TokenResponseTokenTypeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'dPoP': 'DPoP',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'DPoP': 'dPoP',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[TokenResponseTokenTypeEnum];
  @override
  final String wireName = 'TokenResponseTokenTypeEnum';

  @override
  Object serialize(
    Serializers serializers,
    TokenResponseTokenTypeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  TokenResponseTokenTypeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => TokenResponseTokenTypeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$TokenResponse extends TokenResponse {
  @override
  final String accessToken;
  @override
  final TokenResponseTokenTypeEnum tokenType;
  @override
  final int expiresIn;
  @override
  final String refreshToken;
  @override
  final DateTime refreshExpiresAt;
  @override
  final Session session;
  @override
  final Device device;

  factory _$TokenResponse([void Function(TokenResponseBuilder)? updates]) =>
      (TokenResponseBuilder()..update(updates))._build();

  _$TokenResponse._({
    required this.accessToken,
    required this.tokenType,
    required this.expiresIn,
    required this.refreshToken,
    required this.refreshExpiresAt,
    required this.session,
    required this.device,
  }) : super._();
  @override
  TokenResponse rebuild(void Function(TokenResponseBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TokenResponseBuilder toBuilder() => TokenResponseBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TokenResponse &&
        accessToken == other.accessToken &&
        tokenType == other.tokenType &&
        expiresIn == other.expiresIn &&
        refreshToken == other.refreshToken &&
        refreshExpiresAt == other.refreshExpiresAt &&
        session == other.session &&
        device == other.device;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, accessToken.hashCode);
    _$hash = $jc(_$hash, tokenType.hashCode);
    _$hash = $jc(_$hash, expiresIn.hashCode);
    _$hash = $jc(_$hash, refreshToken.hashCode);
    _$hash = $jc(_$hash, refreshExpiresAt.hashCode);
    _$hash = $jc(_$hash, session.hashCode);
    _$hash = $jc(_$hash, device.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TokenResponse')
          ..add('accessToken', accessToken)
          ..add('tokenType', tokenType)
          ..add('expiresIn', expiresIn)
          ..add('refreshToken', refreshToken)
          ..add('refreshExpiresAt', refreshExpiresAt)
          ..add('session', session)
          ..add('device', device))
        .toString();
  }
}

class TokenResponseBuilder
    implements Builder<TokenResponse, TokenResponseBuilder> {
  _$TokenResponse? _$v;

  String? _accessToken;
  String? get accessToken => _$this._accessToken;
  set accessToken(String? accessToken) => _$this._accessToken = accessToken;

  TokenResponseTokenTypeEnum? _tokenType;
  TokenResponseTokenTypeEnum? get tokenType => _$this._tokenType;
  set tokenType(TokenResponseTokenTypeEnum? tokenType) =>
      _$this._tokenType = tokenType;

  int? _expiresIn;
  int? get expiresIn => _$this._expiresIn;
  set expiresIn(int? expiresIn) => _$this._expiresIn = expiresIn;

  String? _refreshToken;
  String? get refreshToken => _$this._refreshToken;
  set refreshToken(String? refreshToken) => _$this._refreshToken = refreshToken;

  DateTime? _refreshExpiresAt;
  DateTime? get refreshExpiresAt => _$this._refreshExpiresAt;
  set refreshExpiresAt(DateTime? refreshExpiresAt) =>
      _$this._refreshExpiresAt = refreshExpiresAt;

  SessionBuilder? _session;
  SessionBuilder get session => _$this._session ??= SessionBuilder();
  set session(SessionBuilder? session) => _$this._session = session;

  DeviceBuilder? _device;
  DeviceBuilder get device => _$this._device ??= DeviceBuilder();
  set device(DeviceBuilder? device) => _$this._device = device;

  TokenResponseBuilder() {
    TokenResponse._defaults(this);
  }

  TokenResponseBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _accessToken = $v.accessToken;
      _tokenType = $v.tokenType;
      _expiresIn = $v.expiresIn;
      _refreshToken = $v.refreshToken;
      _refreshExpiresAt = $v.refreshExpiresAt;
      _session = $v.session.toBuilder();
      _device = $v.device.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TokenResponse other) {
    _$v = other as _$TokenResponse;
  }

  @override
  void update(void Function(TokenResponseBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TokenResponse build() => _build();

  _$TokenResponse _build() {
    _$TokenResponse _$result;
    try {
      _$result =
          _$v ??
          _$TokenResponse._(
            accessToken: BuiltValueNullFieldError.checkNotNull(
              accessToken,
              r'TokenResponse',
              'accessToken',
            ),
            tokenType: BuiltValueNullFieldError.checkNotNull(
              tokenType,
              r'TokenResponse',
              'tokenType',
            ),
            expiresIn: BuiltValueNullFieldError.checkNotNull(
              expiresIn,
              r'TokenResponse',
              'expiresIn',
            ),
            refreshToken: BuiltValueNullFieldError.checkNotNull(
              refreshToken,
              r'TokenResponse',
              'refreshToken',
            ),
            refreshExpiresAt: BuiltValueNullFieldError.checkNotNull(
              refreshExpiresAt,
              r'TokenResponse',
              'refreshExpiresAt',
            ),
            session: session.build(),
            device: device.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'session';
        session.build();
        _$failedField = 'device';
        device.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'TokenResponse',
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

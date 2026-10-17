// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'api_token.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ApiToken extends ApiToken {
  @override
  final String id;
  @override
  final String principalId;
  @override
  final String name;
  @override
  final String tokenPrefix;
  @override
  final BuiltList<AppScope> scopes;
  @override
  final String? dpopJkt;
  @override
  final DateTime? expiresAt;
  @override
  final DateTime createdAt;
  @override
  final DateTime? lastUsedAt;
  @override
  final DateTime? revokedAt;

  factory _$ApiToken([void Function(ApiTokenBuilder)? updates]) =>
      (ApiTokenBuilder()..update(updates))._build();

  _$ApiToken._({
    required this.id,
    required this.principalId,
    required this.name,
    required this.tokenPrefix,
    required this.scopes,
    this.dpopJkt,
    this.expiresAt,
    required this.createdAt,
    this.lastUsedAt,
    this.revokedAt,
  }) : super._();
  @override
  ApiToken rebuild(void Function(ApiTokenBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ApiTokenBuilder toBuilder() => ApiTokenBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ApiToken &&
        id == other.id &&
        principalId == other.principalId &&
        name == other.name &&
        tokenPrefix == other.tokenPrefix &&
        scopes == other.scopes &&
        dpopJkt == other.dpopJkt &&
        expiresAt == other.expiresAt &&
        createdAt == other.createdAt &&
        lastUsedAt == other.lastUsedAt &&
        revokedAt == other.revokedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, principalId.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, tokenPrefix.hashCode);
    _$hash = $jc(_$hash, scopes.hashCode);
    _$hash = $jc(_$hash, dpopJkt.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, lastUsedAt.hashCode);
    _$hash = $jc(_$hash, revokedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ApiToken')
          ..add('id', id)
          ..add('principalId', principalId)
          ..add('name', name)
          ..add('tokenPrefix', tokenPrefix)
          ..add('scopes', scopes)
          ..add('dpopJkt', dpopJkt)
          ..add('expiresAt', expiresAt)
          ..add('createdAt', createdAt)
          ..add('lastUsedAt', lastUsedAt)
          ..add('revokedAt', revokedAt))
        .toString();
  }
}

class ApiTokenBuilder implements Builder<ApiToken, ApiTokenBuilder> {
  _$ApiToken? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _principalId;
  String? get principalId => _$this._principalId;
  set principalId(String? principalId) => _$this._principalId = principalId;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _tokenPrefix;
  String? get tokenPrefix => _$this._tokenPrefix;
  set tokenPrefix(String? tokenPrefix) => _$this._tokenPrefix = tokenPrefix;

  ListBuilder<AppScope>? _scopes;
  ListBuilder<AppScope> get scopes =>
      _$this._scopes ??= ListBuilder<AppScope>();
  set scopes(ListBuilder<AppScope>? scopes) => _$this._scopes = scopes;

  String? _dpopJkt;
  String? get dpopJkt => _$this._dpopJkt;
  set dpopJkt(String? dpopJkt) => _$this._dpopJkt = dpopJkt;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _lastUsedAt;
  DateTime? get lastUsedAt => _$this._lastUsedAt;
  set lastUsedAt(DateTime? lastUsedAt) => _$this._lastUsedAt = lastUsedAt;

  DateTime? _revokedAt;
  DateTime? get revokedAt => _$this._revokedAt;
  set revokedAt(DateTime? revokedAt) => _$this._revokedAt = revokedAt;

  ApiTokenBuilder() {
    ApiToken._defaults(this);
  }

  ApiTokenBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _principalId = $v.principalId;
      _name = $v.name;
      _tokenPrefix = $v.tokenPrefix;
      _scopes = $v.scopes.toBuilder();
      _dpopJkt = $v.dpopJkt;
      _expiresAt = $v.expiresAt;
      _createdAt = $v.createdAt;
      _lastUsedAt = $v.lastUsedAt;
      _revokedAt = $v.revokedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ApiToken other) {
    _$v = other as _$ApiToken;
  }

  @override
  void update(void Function(ApiTokenBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ApiToken build() => _build();

  _$ApiToken _build() {
    _$ApiToken _$result;
    try {
      _$result =
          _$v ??
          _$ApiToken._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'ApiToken', 'id'),
            principalId: BuiltValueNullFieldError.checkNotNull(
              principalId,
              r'ApiToken',
              'principalId',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'ApiToken',
              'name',
            ),
            tokenPrefix: BuiltValueNullFieldError.checkNotNull(
              tokenPrefix,
              r'ApiToken',
              'tokenPrefix',
            ),
            scopes: scopes.build(),
            dpopJkt: dpopJkt,
            expiresAt: expiresAt,
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'ApiToken',
              'createdAt',
            ),
            lastUsedAt: lastUsedAt,
            revokedAt: revokedAt,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'scopes';
        scopes.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ApiToken',
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

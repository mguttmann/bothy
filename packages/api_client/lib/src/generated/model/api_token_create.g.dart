// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'api_token_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ApiTokenCreate extends ApiTokenCreate {
  @override
  final String name;
  @override
  final BuiltList<AppScope>? scopes;
  @override
  final String? dpopJkt;
  @override
  final DateTime expiresAt;

  factory _$ApiTokenCreate([void Function(ApiTokenCreateBuilder)? updates]) =>
      (ApiTokenCreateBuilder()..update(updates))._build();

  _$ApiTokenCreate._({
    required this.name,
    this.scopes,
    this.dpopJkt,
    required this.expiresAt,
  }) : super._();
  @override
  ApiTokenCreate rebuild(void Function(ApiTokenCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ApiTokenCreateBuilder toBuilder() => ApiTokenCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ApiTokenCreate &&
        name == other.name &&
        scopes == other.scopes &&
        dpopJkt == other.dpopJkt &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, scopes.hashCode);
    _$hash = $jc(_$hash, dpopJkt.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ApiTokenCreate')
          ..add('name', name)
          ..add('scopes', scopes)
          ..add('dpopJkt', dpopJkt)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class ApiTokenCreateBuilder
    implements Builder<ApiTokenCreate, ApiTokenCreateBuilder> {
  _$ApiTokenCreate? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

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

  ApiTokenCreateBuilder() {
    ApiTokenCreate._defaults(this);
  }

  ApiTokenCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _scopes = $v.scopes?.toBuilder();
      _dpopJkt = $v.dpopJkt;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ApiTokenCreate other) {
    _$v = other as _$ApiTokenCreate;
  }

  @override
  void update(void Function(ApiTokenCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ApiTokenCreate build() => _build();

  _$ApiTokenCreate _build() {
    _$ApiTokenCreate _$result;
    try {
      _$result =
          _$v ??
          _$ApiTokenCreate._(
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'ApiTokenCreate',
              'name',
            ),
            scopes: _scopes?.build(),
            dpopJkt: dpopJkt,
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'ApiTokenCreate',
              'expiresAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'scopes';
        _scopes?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ApiTokenCreate',
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

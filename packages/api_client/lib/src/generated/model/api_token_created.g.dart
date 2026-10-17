// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'api_token_created.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ApiTokenCreated extends ApiTokenCreated {
  @override
  final ApiToken token;
  @override
  final String secret;

  factory _$ApiTokenCreated([void Function(ApiTokenCreatedBuilder)? updates]) =>
      (ApiTokenCreatedBuilder()..update(updates))._build();

  _$ApiTokenCreated._({required this.token, required this.secret}) : super._();
  @override
  ApiTokenCreated rebuild(void Function(ApiTokenCreatedBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ApiTokenCreatedBuilder toBuilder() => ApiTokenCreatedBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ApiTokenCreated &&
        token == other.token &&
        secret == other.secret;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, token.hashCode);
    _$hash = $jc(_$hash, secret.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ApiTokenCreated')
          ..add('token', token)
          ..add('secret', secret))
        .toString();
  }
}

class ApiTokenCreatedBuilder
    implements Builder<ApiTokenCreated, ApiTokenCreatedBuilder> {
  _$ApiTokenCreated? _$v;

  ApiTokenBuilder? _token;
  ApiTokenBuilder get token => _$this._token ??= ApiTokenBuilder();
  set token(ApiTokenBuilder? token) => _$this._token = token;

  String? _secret;
  String? get secret => _$this._secret;
  set secret(String? secret) => _$this._secret = secret;

  ApiTokenCreatedBuilder() {
    ApiTokenCreated._defaults(this);
  }

  ApiTokenCreatedBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _token = $v.token.toBuilder();
      _secret = $v.secret;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ApiTokenCreated other) {
    _$v = other as _$ApiTokenCreated;
  }

  @override
  void update(void Function(ApiTokenCreatedBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ApiTokenCreated build() => _build();

  _$ApiTokenCreated _build() {
    _$ApiTokenCreated _$result;
    try {
      _$result =
          _$v ??
          _$ApiTokenCreated._(
            token: token.build(),
            secret: BuiltValueNullFieldError.checkNotNull(
              secret,
              r'ApiTokenCreated',
              'secret',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'token';
        token.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ApiTokenCreated',
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

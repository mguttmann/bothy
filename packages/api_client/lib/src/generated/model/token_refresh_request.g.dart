// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'token_refresh_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TokenRefreshRequest extends TokenRefreshRequest {
  @override
  final String refreshToken;

  factory _$TokenRefreshRequest([
    void Function(TokenRefreshRequestBuilder)? updates,
  ]) => (TokenRefreshRequestBuilder()..update(updates))._build();

  _$TokenRefreshRequest._({required this.refreshToken}) : super._();
  @override
  TokenRefreshRequest rebuild(
    void Function(TokenRefreshRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  TokenRefreshRequestBuilder toBuilder() =>
      TokenRefreshRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TokenRefreshRequest && refreshToken == other.refreshToken;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, refreshToken.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'TokenRefreshRequest',
    )..add('refreshToken', refreshToken)).toString();
  }
}

class TokenRefreshRequestBuilder
    implements Builder<TokenRefreshRequest, TokenRefreshRequestBuilder> {
  _$TokenRefreshRequest? _$v;

  String? _refreshToken;
  String? get refreshToken => _$this._refreshToken;
  set refreshToken(String? refreshToken) => _$this._refreshToken = refreshToken;

  TokenRefreshRequestBuilder() {
    TokenRefreshRequest._defaults(this);
  }

  TokenRefreshRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _refreshToken = $v.refreshToken;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TokenRefreshRequest other) {
    _$v = other as _$TokenRefreshRequest;
  }

  @override
  void update(void Function(TokenRefreshRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TokenRefreshRequest build() => _build();

  _$TokenRefreshRequest _build() {
    final _$result =
        _$v ??
        _$TokenRefreshRequest._(
          refreshToken: BuiltValueNullFieldError.checkNotNull(
            refreshToken,
            r'TokenRefreshRequest',
            'refreshToken',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

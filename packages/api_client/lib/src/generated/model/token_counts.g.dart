// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'token_counts.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TokenCounts extends TokenCounts {
  @override
  final int tokensIn;
  @override
  final int tokensOut;
  @override
  final int tokensCacheRead;
  @override
  final int tokensCacheWrite;
  @override
  final int tokensCacheWrite1h;

  factory _$TokenCounts([void Function(TokenCountsBuilder)? updates]) =>
      (TokenCountsBuilder()..update(updates))._build();

  _$TokenCounts._({
    required this.tokensIn,
    required this.tokensOut,
    required this.tokensCacheRead,
    required this.tokensCacheWrite,
    required this.tokensCacheWrite1h,
  }) : super._();
  @override
  TokenCounts rebuild(void Function(TokenCountsBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TokenCountsBuilder toBuilder() => TokenCountsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TokenCounts &&
        tokensIn == other.tokensIn &&
        tokensOut == other.tokensOut &&
        tokensCacheRead == other.tokensCacheRead &&
        tokensCacheWrite == other.tokensCacheWrite &&
        tokensCacheWrite1h == other.tokensCacheWrite1h;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, tokensIn.hashCode);
    _$hash = $jc(_$hash, tokensOut.hashCode);
    _$hash = $jc(_$hash, tokensCacheRead.hashCode);
    _$hash = $jc(_$hash, tokensCacheWrite.hashCode);
    _$hash = $jc(_$hash, tokensCacheWrite1h.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TokenCounts')
          ..add('tokensIn', tokensIn)
          ..add('tokensOut', tokensOut)
          ..add('tokensCacheRead', tokensCacheRead)
          ..add('tokensCacheWrite', tokensCacheWrite)
          ..add('tokensCacheWrite1h', tokensCacheWrite1h))
        .toString();
  }
}

class TokenCountsBuilder implements Builder<TokenCounts, TokenCountsBuilder> {
  _$TokenCounts? _$v;

  int? _tokensIn;
  int? get tokensIn => _$this._tokensIn;
  set tokensIn(int? tokensIn) => _$this._tokensIn = tokensIn;

  int? _tokensOut;
  int? get tokensOut => _$this._tokensOut;
  set tokensOut(int? tokensOut) => _$this._tokensOut = tokensOut;

  int? _tokensCacheRead;
  int? get tokensCacheRead => _$this._tokensCacheRead;
  set tokensCacheRead(int? tokensCacheRead) =>
      _$this._tokensCacheRead = tokensCacheRead;

  int? _tokensCacheWrite;
  int? get tokensCacheWrite => _$this._tokensCacheWrite;
  set tokensCacheWrite(int? tokensCacheWrite) =>
      _$this._tokensCacheWrite = tokensCacheWrite;

  int? _tokensCacheWrite1h;
  int? get tokensCacheWrite1h => _$this._tokensCacheWrite1h;
  set tokensCacheWrite1h(int? tokensCacheWrite1h) =>
      _$this._tokensCacheWrite1h = tokensCacheWrite1h;

  TokenCountsBuilder() {
    TokenCounts._defaults(this);
  }

  TokenCountsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _tokensIn = $v.tokensIn;
      _tokensOut = $v.tokensOut;
      _tokensCacheRead = $v.tokensCacheRead;
      _tokensCacheWrite = $v.tokensCacheWrite;
      _tokensCacheWrite1h = $v.tokensCacheWrite1h;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TokenCounts other) {
    _$v = other as _$TokenCounts;
  }

  @override
  void update(void Function(TokenCountsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TokenCounts build() => _build();

  _$TokenCounts _build() {
    final _$result =
        _$v ??
        _$TokenCounts._(
          tokensIn: BuiltValueNullFieldError.checkNotNull(
            tokensIn,
            r'TokenCounts',
            'tokensIn',
          ),
          tokensOut: BuiltValueNullFieldError.checkNotNull(
            tokensOut,
            r'TokenCounts',
            'tokensOut',
          ),
          tokensCacheRead: BuiltValueNullFieldError.checkNotNull(
            tokensCacheRead,
            r'TokenCounts',
            'tokensCacheRead',
          ),
          tokensCacheWrite: BuiltValueNullFieldError.checkNotNull(
            tokensCacheWrite,
            r'TokenCounts',
            'tokensCacheWrite',
          ),
          tokensCacheWrite1h: BuiltValueNullFieldError.checkNotNull(
            tokensCacheWrite1h,
            r'TokenCounts',
            'tokensCacheWrite1h',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'invocation_token_renew.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$InvocationTokenRenew extends InvocationTokenRenew {
  @override
  final String invocationToken;

  factory _$InvocationTokenRenew([
    void Function(InvocationTokenRenewBuilder)? updates,
  ]) => (InvocationTokenRenewBuilder()..update(updates))._build();

  _$InvocationTokenRenew._({required this.invocationToken}) : super._();
  @override
  InvocationTokenRenew rebuild(
    void Function(InvocationTokenRenewBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  InvocationTokenRenewBuilder toBuilder() =>
      InvocationTokenRenewBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InvocationTokenRenew &&
        invocationToken == other.invocationToken;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, invocationToken.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'InvocationTokenRenew',
    )..add('invocationToken', invocationToken)).toString();
  }
}

class InvocationTokenRenewBuilder
    implements Builder<InvocationTokenRenew, InvocationTokenRenewBuilder> {
  _$InvocationTokenRenew? _$v;

  String? _invocationToken;
  String? get invocationToken => _$this._invocationToken;
  set invocationToken(String? invocationToken) =>
      _$this._invocationToken = invocationToken;

  InvocationTokenRenewBuilder() {
    InvocationTokenRenew._defaults(this);
  }

  InvocationTokenRenewBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _invocationToken = $v.invocationToken;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InvocationTokenRenew other) {
    _$v = other as _$InvocationTokenRenew;
  }

  @override
  void update(void Function(InvocationTokenRenewBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InvocationTokenRenew build() => _build();

  _$InvocationTokenRenew _build() {
    final _$result =
        _$v ??
        _$InvocationTokenRenew._(
          invocationToken: BuiltValueNullFieldError.checkNotNull(
            invocationToken,
            r'InvocationTokenRenew',
            'invocationToken',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

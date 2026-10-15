// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'invocation_token_grant.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$InvocationTokenGrant extends InvocationTokenGrant {
  @override
  final String invocationToken;
  @override
  final DateTime expiresAt;

  factory _$InvocationTokenGrant([
    void Function(InvocationTokenGrantBuilder)? updates,
  ]) => (InvocationTokenGrantBuilder()..update(updates))._build();

  _$InvocationTokenGrant._({
    required this.invocationToken,
    required this.expiresAt,
  }) : super._();
  @override
  InvocationTokenGrant rebuild(
    void Function(InvocationTokenGrantBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  InvocationTokenGrantBuilder toBuilder() =>
      InvocationTokenGrantBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InvocationTokenGrant &&
        invocationToken == other.invocationToken &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, invocationToken.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'InvocationTokenGrant')
          ..add('invocationToken', invocationToken)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class InvocationTokenGrantBuilder
    implements Builder<InvocationTokenGrant, InvocationTokenGrantBuilder> {
  _$InvocationTokenGrant? _$v;

  String? _invocationToken;
  String? get invocationToken => _$this._invocationToken;
  set invocationToken(String? invocationToken) =>
      _$this._invocationToken = invocationToken;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  InvocationTokenGrantBuilder() {
    InvocationTokenGrant._defaults(this);
  }

  InvocationTokenGrantBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _invocationToken = $v.invocationToken;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InvocationTokenGrant other) {
    _$v = other as _$InvocationTokenGrant;
  }

  @override
  void update(void Function(InvocationTokenGrantBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InvocationTokenGrant build() => _build();

  _$InvocationTokenGrant _build() {
    final _$result =
        _$v ??
        _$InvocationTokenGrant._(
          invocationToken: BuiltValueNullFieldError.checkNotNull(
            invocationToken,
            r'InvocationTokenGrant',
            'invocationToken',
          ),
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'InvocationTokenGrant',
            'expiresAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

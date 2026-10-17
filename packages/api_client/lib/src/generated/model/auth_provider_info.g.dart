// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'auth_provider_info.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AuthProviderInfo extends AuthProviderInfo {
  @override
  final String id;
  @override
  final IdentityProviderKind kind;
  @override
  final String name;

  factory _$AuthProviderInfo([
    void Function(AuthProviderInfoBuilder)? updates,
  ]) => (AuthProviderInfoBuilder()..update(updates))._build();

  _$AuthProviderInfo._({
    required this.id,
    required this.kind,
    required this.name,
  }) : super._();
  @override
  AuthProviderInfo rebuild(void Function(AuthProviderInfoBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AuthProviderInfoBuilder toBuilder() =>
      AuthProviderInfoBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AuthProviderInfo &&
        id == other.id &&
        kind == other.kind &&
        name == other.name;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AuthProviderInfo')
          ..add('id', id)
          ..add('kind', kind)
          ..add('name', name))
        .toString();
  }
}

class AuthProviderInfoBuilder
    implements Builder<AuthProviderInfo, AuthProviderInfoBuilder> {
  _$AuthProviderInfo? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  IdentityProviderKind? _kind;
  IdentityProviderKind? get kind => _$this._kind;
  set kind(IdentityProviderKind? kind) => _$this._kind = kind;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  AuthProviderInfoBuilder() {
    AuthProviderInfo._defaults(this);
  }

  AuthProviderInfoBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _kind = $v.kind;
      _name = $v.name;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AuthProviderInfo other) {
    _$v = other as _$AuthProviderInfo;
  }

  @override
  void update(void Function(AuthProviderInfoBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AuthProviderInfo build() => _build();

  _$AuthProviderInfo _build() {
    final _$result =
        _$v ??
        _$AuthProviderInfo._(
          id: BuiltValueNullFieldError.checkNotNull(
            id,
            r'AuthProviderInfo',
            'id',
          ),
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'AuthProviderInfo',
            'kind',
          ),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'AuthProviderInfo',
            'name',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'owner.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Owner extends Owner {
  @override
  final PrincipalRef user;
  @override
  final String? grantedBy;
  @override
  final DateTime grantedAt;
  @override
  final String reason;

  factory _$Owner([void Function(OwnerBuilder)? updates]) =>
      (OwnerBuilder()..update(updates))._build();

  _$Owner._({
    required this.user,
    this.grantedBy,
    required this.grantedAt,
    required this.reason,
  }) : super._();
  @override
  Owner rebuild(void Function(OwnerBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  OwnerBuilder toBuilder() => OwnerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Owner &&
        user == other.user &&
        grantedBy == other.grantedBy &&
        grantedAt == other.grantedAt &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, user.hashCode);
    _$hash = $jc(_$hash, grantedBy.hashCode);
    _$hash = $jc(_$hash, grantedAt.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Owner')
          ..add('user', user)
          ..add('grantedBy', grantedBy)
          ..add('grantedAt', grantedAt)
          ..add('reason', reason))
        .toString();
  }
}

class OwnerBuilder implements Builder<Owner, OwnerBuilder> {
  _$Owner? _$v;

  PrincipalRefBuilder? _user;
  PrincipalRefBuilder get user => _$this._user ??= PrincipalRefBuilder();
  set user(PrincipalRefBuilder? user) => _$this._user = user;

  String? _grantedBy;
  String? get grantedBy => _$this._grantedBy;
  set grantedBy(String? grantedBy) => _$this._grantedBy = grantedBy;

  DateTime? _grantedAt;
  DateTime? get grantedAt => _$this._grantedAt;
  set grantedAt(DateTime? grantedAt) => _$this._grantedAt = grantedAt;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  OwnerBuilder() {
    Owner._defaults(this);
  }

  OwnerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _user = $v.user.toBuilder();
      _grantedBy = $v.grantedBy;
      _grantedAt = $v.grantedAt;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Owner other) {
    _$v = other as _$Owner;
  }

  @override
  void update(void Function(OwnerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Owner build() => _build();

  _$Owner _build() {
    _$Owner _$result;
    try {
      _$result =
          _$v ??
          _$Owner._(
            user: user.build(),
            grantedBy: grantedBy,
            grantedAt: BuiltValueNullFieldError.checkNotNull(
              grantedAt,
              r'Owner',
              'grantedAt',
            ),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'Owner',
              'reason',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'user';
        user.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(r'Owner', _$failedField, e.toString());
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

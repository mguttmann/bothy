// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'owner_add.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$OwnerAdd extends OwnerAdd {
  @override
  final String userId;
  @override
  final String reason;
  @override
  final ResolveI1? resolveI1;

  factory _$OwnerAdd([void Function(OwnerAddBuilder)? updates]) =>
      (OwnerAddBuilder()..update(updates))._build();

  _$OwnerAdd._({required this.userId, required this.reason, this.resolveI1})
    : super._();
  @override
  OwnerAdd rebuild(void Function(OwnerAddBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  OwnerAddBuilder toBuilder() => OwnerAddBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is OwnerAdd &&
        userId == other.userId &&
        reason == other.reason &&
        resolveI1 == other.resolveI1;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, resolveI1.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'OwnerAdd')
          ..add('userId', userId)
          ..add('reason', reason)
          ..add('resolveI1', resolveI1))
        .toString();
  }
}

class OwnerAddBuilder implements Builder<OwnerAdd, OwnerAddBuilder> {
  _$OwnerAdd? _$v;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  ResolveI1Builder? _resolveI1;
  ResolveI1Builder get resolveI1 => _$this._resolveI1 ??= ResolveI1Builder();
  set resolveI1(ResolveI1Builder? resolveI1) => _$this._resolveI1 = resolveI1;

  OwnerAddBuilder() {
    OwnerAdd._defaults(this);
  }

  OwnerAddBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _userId = $v.userId;
      _reason = $v.reason;
      _resolveI1 = $v.resolveI1?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(OwnerAdd other) {
    _$v = other as _$OwnerAdd;
  }

  @override
  void update(void Function(OwnerAddBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  OwnerAdd build() => _build();

  _$OwnerAdd _build() {
    _$OwnerAdd _$result;
    try {
      _$result =
          _$v ??
          _$OwnerAdd._(
            userId: BuiltValueNullFieldError.checkNotNull(
              userId,
              r'OwnerAdd',
              'userId',
            ),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'OwnerAdd',
              'reason',
            ),
            resolveI1: _resolveI1?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'resolveI1';
        _resolveI1?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'OwnerAdd',
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

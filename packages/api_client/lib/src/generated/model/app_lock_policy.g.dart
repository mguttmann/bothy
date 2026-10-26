// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_lock_policy.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AppLockPolicy extends AppLockPolicy {
  @override
  final bool required_;
  @override
  final int afterMs;

  factory _$AppLockPolicy([void Function(AppLockPolicyBuilder)? updates]) =>
      (AppLockPolicyBuilder()..update(updates))._build();

  _$AppLockPolicy._({required this.required_, required this.afterMs})
    : super._();
  @override
  AppLockPolicy rebuild(void Function(AppLockPolicyBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AppLockPolicyBuilder toBuilder() => AppLockPolicyBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AppLockPolicy &&
        required_ == other.required_ &&
        afterMs == other.afterMs;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, required_.hashCode);
    _$hash = $jc(_$hash, afterMs.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AppLockPolicy')
          ..add('required_', required_)
          ..add('afterMs', afterMs))
        .toString();
  }
}

class AppLockPolicyBuilder
    implements Builder<AppLockPolicy, AppLockPolicyBuilder> {
  _$AppLockPolicy? _$v;

  bool? _required_;
  bool? get required_ => _$this._required_;
  set required_(bool? required_) => _$this._required_ = required_;

  int? _afterMs;
  int? get afterMs => _$this._afterMs;
  set afterMs(int? afterMs) => _$this._afterMs = afterMs;

  AppLockPolicyBuilder() {
    AppLockPolicy._defaults(this);
  }

  AppLockPolicyBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _required_ = $v.required_;
      _afterMs = $v.afterMs;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AppLockPolicy other) {
    _$v = other as _$AppLockPolicy;
  }

  @override
  void update(void Function(AppLockPolicyBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AppLockPolicy build() => _build();

  _$AppLockPolicy _build() {
    final _$result =
        _$v ??
        _$AppLockPolicy._(
          required_: BuiltValueNullFieldError.checkNotNull(
            required_,
            r'AppLockPolicy',
            'required_',
          ),
          afterMs: BuiltValueNullFieldError.checkNotNull(
            afterMs,
            r'AppLockPolicy',
            'afterMs',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

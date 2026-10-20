// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_manifest_scopes.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AppManifestScopes extends AppManifestScopes {
  @override
  final BuiltList<AppScope> required_;
  @override
  final BuiltList<AppScope> optional;

  factory _$AppManifestScopes([
    void Function(AppManifestScopesBuilder)? updates,
  ]) => (AppManifestScopesBuilder()..update(updates))._build();

  _$AppManifestScopes._({required this.required_, required this.optional})
    : super._();
  @override
  AppManifestScopes rebuild(void Function(AppManifestScopesBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AppManifestScopesBuilder toBuilder() =>
      AppManifestScopesBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AppManifestScopes &&
        required_ == other.required_ &&
        optional == other.optional;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, required_.hashCode);
    _$hash = $jc(_$hash, optional.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AppManifestScopes')
          ..add('required_', required_)
          ..add('optional', optional))
        .toString();
  }
}

class AppManifestScopesBuilder
    implements Builder<AppManifestScopes, AppManifestScopesBuilder> {
  _$AppManifestScopes? _$v;

  ListBuilder<AppScope>? _required_;
  ListBuilder<AppScope> get required_ =>
      _$this._required_ ??= ListBuilder<AppScope>();
  set required_(ListBuilder<AppScope>? required_) =>
      _$this._required_ = required_;

  ListBuilder<AppScope>? _optional;
  ListBuilder<AppScope> get optional =>
      _$this._optional ??= ListBuilder<AppScope>();
  set optional(ListBuilder<AppScope>? optional) => _$this._optional = optional;

  AppManifestScopesBuilder() {
    AppManifestScopes._defaults(this);
  }

  AppManifestScopesBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _required_ = $v.required_.toBuilder();
      _optional = $v.optional.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AppManifestScopes other) {
    _$v = other as _$AppManifestScopes;
  }

  @override
  void update(void Function(AppManifestScopesBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AppManifestScopes build() => _build();

  _$AppManifestScopes _build() {
    _$AppManifestScopes _$result;
    try {
      _$result =
          _$v ??
          _$AppManifestScopes._(
            required_: required_.build(),
            optional: optional.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'required_';
        required_.build();
        _$failedField = 'optional';
        optional.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AppManifestScopes',
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

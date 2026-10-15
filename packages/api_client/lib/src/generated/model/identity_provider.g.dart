// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'identity_provider.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$IdentityProvider extends IdentityProvider {
  @override
  final String id;
  @override
  final IdentityProviderKind kind;
  @override
  final String name;
  @override
  final BuiltMap<String, JsonObject?> config;
  @override
  final bool enabled;
  @override
  final bool allowProvisioning;
  @override
  final DateTime? updatedAt;

  factory _$IdentityProvider([
    void Function(IdentityProviderBuilder)? updates,
  ]) => (IdentityProviderBuilder()..update(updates))._build();

  _$IdentityProvider._({
    required this.id,
    required this.kind,
    required this.name,
    required this.config,
    required this.enabled,
    required this.allowProvisioning,
    this.updatedAt,
  }) : super._();
  @override
  IdentityProvider rebuild(void Function(IdentityProviderBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  IdentityProviderBuilder toBuilder() =>
      IdentityProviderBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IdentityProvider &&
        id == other.id &&
        kind == other.kind &&
        name == other.name &&
        config == other.config &&
        enabled == other.enabled &&
        allowProvisioning == other.allowProvisioning &&
        updatedAt == other.updatedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, config.hashCode);
    _$hash = $jc(_$hash, enabled.hashCode);
    _$hash = $jc(_$hash, allowProvisioning.hashCode);
    _$hash = $jc(_$hash, updatedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'IdentityProvider')
          ..add('id', id)
          ..add('kind', kind)
          ..add('name', name)
          ..add('config', config)
          ..add('enabled', enabled)
          ..add('allowProvisioning', allowProvisioning)
          ..add('updatedAt', updatedAt))
        .toString();
  }
}

class IdentityProviderBuilder
    implements Builder<IdentityProvider, IdentityProviderBuilder> {
  _$IdentityProvider? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  IdentityProviderKind? _kind;
  IdentityProviderKind? get kind => _$this._kind;
  set kind(IdentityProviderKind? kind) => _$this._kind = kind;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  MapBuilder<String, JsonObject?>? _config;
  MapBuilder<String, JsonObject?> get config =>
      _$this._config ??= MapBuilder<String, JsonObject?>();
  set config(MapBuilder<String, JsonObject?>? config) =>
      _$this._config = config;

  bool? _enabled;
  bool? get enabled => _$this._enabled;
  set enabled(bool? enabled) => _$this._enabled = enabled;

  bool? _allowProvisioning;
  bool? get allowProvisioning => _$this._allowProvisioning;
  set allowProvisioning(bool? allowProvisioning) =>
      _$this._allowProvisioning = allowProvisioning;

  DateTime? _updatedAt;
  DateTime? get updatedAt => _$this._updatedAt;
  set updatedAt(DateTime? updatedAt) => _$this._updatedAt = updatedAt;

  IdentityProviderBuilder() {
    IdentityProvider._defaults(this);
  }

  IdentityProviderBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _kind = $v.kind;
      _name = $v.name;
      _config = $v.config.toBuilder();
      _enabled = $v.enabled;
      _allowProvisioning = $v.allowProvisioning;
      _updatedAt = $v.updatedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IdentityProvider other) {
    _$v = other as _$IdentityProvider;
  }

  @override
  void update(void Function(IdentityProviderBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IdentityProvider build() => _build();

  _$IdentityProvider _build() {
    _$IdentityProvider _$result;
    try {
      _$result =
          _$v ??
          _$IdentityProvider._(
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'IdentityProvider',
              'id',
            ),
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'IdentityProvider',
              'kind',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'IdentityProvider',
              'name',
            ),
            config: config.build(),
            enabled: BuiltValueNullFieldError.checkNotNull(
              enabled,
              r'IdentityProvider',
              'enabled',
            ),
            allowProvisioning: BuiltValueNullFieldError.checkNotNull(
              allowProvisioning,
              r'IdentityProvider',
              'allowProvisioning',
            ),
            updatedAt: updatedAt,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'config';
        config.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'IdentityProvider',
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

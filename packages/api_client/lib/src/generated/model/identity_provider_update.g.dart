// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'identity_provider_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$IdentityProviderUpdate extends IdentityProviderUpdate {
  @override
  final String? name;
  @override
  final BuiltMap<String, JsonObject?>? config;
  @override
  final bool? enabled;
  @override
  final bool? allowProvisioning;

  factory _$IdentityProviderUpdate([
    void Function(IdentityProviderUpdateBuilder)? updates,
  ]) => (IdentityProviderUpdateBuilder()..update(updates))._build();

  _$IdentityProviderUpdate._({
    this.name,
    this.config,
    this.enabled,
    this.allowProvisioning,
  }) : super._();
  @override
  IdentityProviderUpdate rebuild(
    void Function(IdentityProviderUpdateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  IdentityProviderUpdateBuilder toBuilder() =>
      IdentityProviderUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IdentityProviderUpdate &&
        name == other.name &&
        config == other.config &&
        enabled == other.enabled &&
        allowProvisioning == other.allowProvisioning;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, config.hashCode);
    _$hash = $jc(_$hash, enabled.hashCode);
    _$hash = $jc(_$hash, allowProvisioning.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'IdentityProviderUpdate')
          ..add('name', name)
          ..add('config', config)
          ..add('enabled', enabled)
          ..add('allowProvisioning', allowProvisioning))
        .toString();
  }
}

class IdentityProviderUpdateBuilder
    implements Builder<IdentityProviderUpdate, IdentityProviderUpdateBuilder> {
  _$IdentityProviderUpdate? _$v;

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

  IdentityProviderUpdateBuilder() {
    IdentityProviderUpdate._defaults(this);
  }

  IdentityProviderUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _config = $v.config?.toBuilder();
      _enabled = $v.enabled;
      _allowProvisioning = $v.allowProvisioning;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IdentityProviderUpdate other) {
    _$v = other as _$IdentityProviderUpdate;
  }

  @override
  void update(void Function(IdentityProviderUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IdentityProviderUpdate build() => _build();

  _$IdentityProviderUpdate _build() {
    _$IdentityProviderUpdate _$result;
    try {
      _$result =
          _$v ??
          _$IdentityProviderUpdate._(
            name: name,
            config: _config?.build(),
            enabled: enabled,
            allowProvisioning: allowProvisioning,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'config';
        _config?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'IdentityProviderUpdate',
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

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'identity_provider_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$IdentityProviderCreate extends IdentityProviderCreate {
  @override
  final IdentityProviderKind kind;
  @override
  final String name;
  @override
  final BuiltMap<String, JsonObject?> config;
  @override
  final bool? enabled;
  @override
  final bool? allowProvisioning;

  factory _$IdentityProviderCreate([
    void Function(IdentityProviderCreateBuilder)? updates,
  ]) => (IdentityProviderCreateBuilder()..update(updates))._build();

  _$IdentityProviderCreate._({
    required this.kind,
    required this.name,
    required this.config,
    this.enabled,
    this.allowProvisioning,
  }) : super._();
  @override
  IdentityProviderCreate rebuild(
    void Function(IdentityProviderCreateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  IdentityProviderCreateBuilder toBuilder() =>
      IdentityProviderCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IdentityProviderCreate &&
        kind == other.kind &&
        name == other.name &&
        config == other.config &&
        enabled == other.enabled &&
        allowProvisioning == other.allowProvisioning;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, config.hashCode);
    _$hash = $jc(_$hash, enabled.hashCode);
    _$hash = $jc(_$hash, allowProvisioning.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'IdentityProviderCreate')
          ..add('kind', kind)
          ..add('name', name)
          ..add('config', config)
          ..add('enabled', enabled)
          ..add('allowProvisioning', allowProvisioning))
        .toString();
  }
}

class IdentityProviderCreateBuilder
    implements Builder<IdentityProviderCreate, IdentityProviderCreateBuilder> {
  _$IdentityProviderCreate? _$v;

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

  IdentityProviderCreateBuilder() {
    IdentityProviderCreate._defaults(this);
  }

  IdentityProviderCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _name = $v.name;
      _config = $v.config.toBuilder();
      _enabled = $v.enabled;
      _allowProvisioning = $v.allowProvisioning;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IdentityProviderCreate other) {
    _$v = other as _$IdentityProviderCreate;
  }

  @override
  void update(void Function(IdentityProviderCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IdentityProviderCreate build() => _build();

  _$IdentityProviderCreate _build() {
    _$IdentityProviderCreate _$result;
    try {
      _$result =
          _$v ??
          _$IdentityProviderCreate._(
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'IdentityProviderCreate',
              'kind',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'IdentityProviderCreate',
              'name',
            ),
            config: config.build(),
            enabled: enabled,
            allowProvisioning: allowProvisioning,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'config';
        config.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'IdentityProviderCreate',
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

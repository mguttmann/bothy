// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentCreate extends AgentCreate {
  @override
  final String handle;
  @override
  final String displayName;
  @override
  final String adapterKind;
  @override
  final String? description;
  @override
  final BuiltMap<String, JsonObject?>? operatorConfig;

  factory _$AgentCreate([void Function(AgentCreateBuilder)? updates]) =>
      (AgentCreateBuilder()..update(updates))._build();

  _$AgentCreate._({
    required this.handle,
    required this.displayName,
    required this.adapterKind,
    this.description,
    this.operatorConfig,
  }) : super._();
  @override
  AgentCreate rebuild(void Function(AgentCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentCreateBuilder toBuilder() => AgentCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentCreate &&
        handle == other.handle &&
        displayName == other.displayName &&
        adapterKind == other.adapterKind &&
        description == other.description &&
        operatorConfig == other.operatorConfig;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, handle.hashCode);
    _$hash = $jc(_$hash, displayName.hashCode);
    _$hash = $jc(_$hash, adapterKind.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, operatorConfig.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentCreate')
          ..add('handle', handle)
          ..add('displayName', displayName)
          ..add('adapterKind', adapterKind)
          ..add('description', description)
          ..add('operatorConfig', operatorConfig))
        .toString();
  }
}

class AgentCreateBuilder implements Builder<AgentCreate, AgentCreateBuilder> {
  _$AgentCreate? _$v;

  String? _handle;
  String? get handle => _$this._handle;
  set handle(String? handle) => _$this._handle = handle;

  String? _displayName;
  String? get displayName => _$this._displayName;
  set displayName(String? displayName) => _$this._displayName = displayName;

  String? _adapterKind;
  String? get adapterKind => _$this._adapterKind;
  set adapterKind(String? adapterKind) => _$this._adapterKind = adapterKind;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  MapBuilder<String, JsonObject?>? _operatorConfig;
  MapBuilder<String, JsonObject?> get operatorConfig =>
      _$this._operatorConfig ??= MapBuilder<String, JsonObject?>();
  set operatorConfig(MapBuilder<String, JsonObject?>? operatorConfig) =>
      _$this._operatorConfig = operatorConfig;

  AgentCreateBuilder() {
    AgentCreate._defaults(this);
  }

  AgentCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _handle = $v.handle;
      _displayName = $v.displayName;
      _adapterKind = $v.adapterKind;
      _description = $v.description;
      _operatorConfig = $v.operatorConfig?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentCreate other) {
    _$v = other as _$AgentCreate;
  }

  @override
  void update(void Function(AgentCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentCreate build() => _build();

  _$AgentCreate _build() {
    _$AgentCreate _$result;
    try {
      _$result =
          _$v ??
          _$AgentCreate._(
            handle: BuiltValueNullFieldError.checkNotNull(
              handle,
              r'AgentCreate',
              'handle',
            ),
            displayName: BuiltValueNullFieldError.checkNotNull(
              displayName,
              r'AgentCreate',
              'displayName',
            ),
            adapterKind: BuiltValueNullFieldError.checkNotNull(
              adapterKind,
              r'AgentCreate',
              'adapterKind',
            ),
            description: description,
            operatorConfig: _operatorConfig?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'operatorConfig';
        _operatorConfig?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentCreate',
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

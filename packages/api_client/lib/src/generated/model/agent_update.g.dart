// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentUpdate extends AgentUpdate {
  @override
  final String? displayName;
  @override
  final String? description;
  @override
  final BuiltMap<String, JsonObject?>? operatorConfig;

  factory _$AgentUpdate([void Function(AgentUpdateBuilder)? updates]) =>
      (AgentUpdateBuilder()..update(updates))._build();

  _$AgentUpdate._({this.displayName, this.description, this.operatorConfig})
    : super._();
  @override
  AgentUpdate rebuild(void Function(AgentUpdateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentUpdateBuilder toBuilder() => AgentUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentUpdate &&
        displayName == other.displayName &&
        description == other.description &&
        operatorConfig == other.operatorConfig;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, displayName.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, operatorConfig.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentUpdate')
          ..add('displayName', displayName)
          ..add('description', description)
          ..add('operatorConfig', operatorConfig))
        .toString();
  }
}

class AgentUpdateBuilder implements Builder<AgentUpdate, AgentUpdateBuilder> {
  _$AgentUpdate? _$v;

  String? _displayName;
  String? get displayName => _$this._displayName;
  set displayName(String? displayName) => _$this._displayName = displayName;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  MapBuilder<String, JsonObject?>? _operatorConfig;
  MapBuilder<String, JsonObject?> get operatorConfig =>
      _$this._operatorConfig ??= MapBuilder<String, JsonObject?>();
  set operatorConfig(MapBuilder<String, JsonObject?>? operatorConfig) =>
      _$this._operatorConfig = operatorConfig;

  AgentUpdateBuilder() {
    AgentUpdate._defaults(this);
  }

  AgentUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _displayName = $v.displayName;
      _description = $v.description;
      _operatorConfig = $v.operatorConfig?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentUpdate other) {
    _$v = other as _$AgentUpdate;
  }

  @override
  void update(void Function(AgentUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentUpdate build() => _build();

  _$AgentUpdate _build() {
    _$AgentUpdate _$result;
    try {
      _$result =
          _$v ??
          _$AgentUpdate._(
            displayName: displayName,
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
          r'AgentUpdate',
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

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_definition.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentDefinition extends AgentDefinition {
  @override
  final String instructions;
  @override
  final BuiltList<AgentDefinitionSkill> skills;
  @override
  final BuiltList<AgentDefinitionMcpServer> mcpServers;
  @override
  final AgentDefinitionSettings settings;
  @override
  final AgentDefinitionOrigin origin;

  factory _$AgentDefinition([void Function(AgentDefinitionBuilder)? updates]) =>
      (AgentDefinitionBuilder()..update(updates))._build();

  _$AgentDefinition._({
    required this.instructions,
    required this.skills,
    required this.mcpServers,
    required this.settings,
    required this.origin,
  }) : super._();
  @override
  AgentDefinition rebuild(void Function(AgentDefinitionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentDefinitionBuilder toBuilder() => AgentDefinitionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentDefinition &&
        instructions == other.instructions &&
        skills == other.skills &&
        mcpServers == other.mcpServers &&
        settings == other.settings &&
        origin == other.origin;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, instructions.hashCode);
    _$hash = $jc(_$hash, skills.hashCode);
    _$hash = $jc(_$hash, mcpServers.hashCode);
    _$hash = $jc(_$hash, settings.hashCode);
    _$hash = $jc(_$hash, origin.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentDefinition')
          ..add('instructions', instructions)
          ..add('skills', skills)
          ..add('mcpServers', mcpServers)
          ..add('settings', settings)
          ..add('origin', origin))
        .toString();
  }
}

class AgentDefinitionBuilder
    implements Builder<AgentDefinition, AgentDefinitionBuilder> {
  _$AgentDefinition? _$v;

  String? _instructions;
  String? get instructions => _$this._instructions;
  set instructions(String? instructions) => _$this._instructions = instructions;

  ListBuilder<AgentDefinitionSkill>? _skills;
  ListBuilder<AgentDefinitionSkill> get skills =>
      _$this._skills ??= ListBuilder<AgentDefinitionSkill>();
  set skills(ListBuilder<AgentDefinitionSkill>? skills) =>
      _$this._skills = skills;

  ListBuilder<AgentDefinitionMcpServer>? _mcpServers;
  ListBuilder<AgentDefinitionMcpServer> get mcpServers =>
      _$this._mcpServers ??= ListBuilder<AgentDefinitionMcpServer>();
  set mcpServers(ListBuilder<AgentDefinitionMcpServer>? mcpServers) =>
      _$this._mcpServers = mcpServers;

  AgentDefinitionSettingsBuilder? _settings;
  AgentDefinitionSettingsBuilder get settings =>
      _$this._settings ??= AgentDefinitionSettingsBuilder();
  set settings(AgentDefinitionSettingsBuilder? settings) =>
      _$this._settings = settings;

  AgentDefinitionOriginBuilder? _origin;
  AgentDefinitionOriginBuilder get origin =>
      _$this._origin ??= AgentDefinitionOriginBuilder();
  set origin(AgentDefinitionOriginBuilder? origin) => _$this._origin = origin;

  AgentDefinitionBuilder() {
    AgentDefinition._defaults(this);
  }

  AgentDefinitionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _instructions = $v.instructions;
      _skills = $v.skills.toBuilder();
      _mcpServers = $v.mcpServers.toBuilder();
      _settings = $v.settings.toBuilder();
      _origin = $v.origin.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentDefinition other) {
    _$v = other as _$AgentDefinition;
  }

  @override
  void update(void Function(AgentDefinitionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentDefinition build() => _build();

  _$AgentDefinition _build() {
    _$AgentDefinition _$result;
    try {
      _$result =
          _$v ??
          _$AgentDefinition._(
            instructions: BuiltValueNullFieldError.checkNotNull(
              instructions,
              r'AgentDefinition',
              'instructions',
            ),
            skills: skills.build(),
            mcpServers: mcpServers.build(),
            settings: settings.build(),
            origin: origin.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'skills';
        skills.build();
        _$failedField = 'mcpServers';
        mcpServers.build();
        _$failedField = 'settings';
        settings.build();
        _$failedField = 'origin';
        origin.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentDefinition',
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

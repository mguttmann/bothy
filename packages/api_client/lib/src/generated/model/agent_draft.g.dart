// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_draft.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentDraft extends AgentDraft {
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
  @override
  final String name;
  @override
  final String handle;
  @override
  final String description;
  @override
  final AgentDraftChannel? channelSuggestion;

  factory _$AgentDraft([void Function(AgentDraftBuilder)? updates]) =>
      (AgentDraftBuilder()..update(updates))._build();

  _$AgentDraft._({
    required this.instructions,
    required this.skills,
    required this.mcpServers,
    required this.settings,
    required this.origin,
    required this.name,
    required this.handle,
    required this.description,
    this.channelSuggestion,
  }) : super._();
  @override
  AgentDraft rebuild(void Function(AgentDraftBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentDraftBuilder toBuilder() => AgentDraftBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentDraft &&
        instructions == other.instructions &&
        skills == other.skills &&
        mcpServers == other.mcpServers &&
        settings == other.settings &&
        origin == other.origin &&
        name == other.name &&
        handle == other.handle &&
        description == other.description &&
        channelSuggestion == other.channelSuggestion;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, instructions.hashCode);
    _$hash = $jc(_$hash, skills.hashCode);
    _$hash = $jc(_$hash, mcpServers.hashCode);
    _$hash = $jc(_$hash, settings.hashCode);
    _$hash = $jc(_$hash, origin.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, handle.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, channelSuggestion.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentDraft')
          ..add('instructions', instructions)
          ..add('skills', skills)
          ..add('mcpServers', mcpServers)
          ..add('settings', settings)
          ..add('origin', origin)
          ..add('name', name)
          ..add('handle', handle)
          ..add('description', description)
          ..add('channelSuggestion', channelSuggestion))
        .toString();
  }
}

class AgentDraftBuilder implements Builder<AgentDraft, AgentDraftBuilder> {
  _$AgentDraft? _$v;

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

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _handle;
  String? get handle => _$this._handle;
  set handle(String? handle) => _$this._handle = handle;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  AgentDraftChannelBuilder? _channelSuggestion;
  AgentDraftChannelBuilder get channelSuggestion =>
      _$this._channelSuggestion ??= AgentDraftChannelBuilder();
  set channelSuggestion(AgentDraftChannelBuilder? channelSuggestion) =>
      _$this._channelSuggestion = channelSuggestion;

  AgentDraftBuilder() {
    AgentDraft._defaults(this);
  }

  AgentDraftBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _instructions = $v.instructions;
      _skills = $v.skills.toBuilder();
      _mcpServers = $v.mcpServers.toBuilder();
      _settings = $v.settings.toBuilder();
      _origin = $v.origin.toBuilder();
      _name = $v.name;
      _handle = $v.handle;
      _description = $v.description;
      _channelSuggestion = $v.channelSuggestion?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentDraft other) {
    _$v = other as _$AgentDraft;
  }

  @override
  void update(void Function(AgentDraftBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentDraft build() => _build();

  _$AgentDraft _build() {
    _$AgentDraft _$result;
    try {
      _$result =
          _$v ??
          _$AgentDraft._(
            instructions: BuiltValueNullFieldError.checkNotNull(
              instructions,
              r'AgentDraft',
              'instructions',
            ),
            skills: skills.build(),
            mcpServers: mcpServers.build(),
            settings: settings.build(),
            origin: origin.build(),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'AgentDraft',
              'name',
            ),
            handle: BuiltValueNullFieldError.checkNotNull(
              handle,
              r'AgentDraft',
              'handle',
            ),
            description: BuiltValueNullFieldError.checkNotNull(
              description,
              r'AgentDraft',
              'description',
            ),
            channelSuggestion: _channelSuggestion?.build(),
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

        _$failedField = 'channelSuggestion';
        _channelSuggestion?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentDraft',
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

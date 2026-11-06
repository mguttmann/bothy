// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_definition_skill.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentDefinitionSkill extends AgentDefinitionSkill {
  @override
  final String name;
  @override
  final String description;
  @override
  final String body;
  @override
  final BuiltList<AgentDefinitionSkillFile>? files;

  factory _$AgentDefinitionSkill([
    void Function(AgentDefinitionSkillBuilder)? updates,
  ]) => (AgentDefinitionSkillBuilder()..update(updates))._build();

  _$AgentDefinitionSkill._({
    required this.name,
    required this.description,
    required this.body,
    this.files,
  }) : super._();
  @override
  AgentDefinitionSkill rebuild(
    void Function(AgentDefinitionSkillBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentDefinitionSkillBuilder toBuilder() =>
      AgentDefinitionSkillBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentDefinitionSkill &&
        name == other.name &&
        description == other.description &&
        body == other.body &&
        files == other.files;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, body.hashCode);
    _$hash = $jc(_$hash, files.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentDefinitionSkill')
          ..add('name', name)
          ..add('description', description)
          ..add('body', body)
          ..add('files', files))
        .toString();
  }
}

class AgentDefinitionSkillBuilder
    implements Builder<AgentDefinitionSkill, AgentDefinitionSkillBuilder> {
  _$AgentDefinitionSkill? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  String? _body;
  String? get body => _$this._body;
  set body(String? body) => _$this._body = body;

  ListBuilder<AgentDefinitionSkillFile>? _files;
  ListBuilder<AgentDefinitionSkillFile> get files =>
      _$this._files ??= ListBuilder<AgentDefinitionSkillFile>();
  set files(ListBuilder<AgentDefinitionSkillFile>? files) =>
      _$this._files = files;

  AgentDefinitionSkillBuilder() {
    AgentDefinitionSkill._defaults(this);
  }

  AgentDefinitionSkillBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _description = $v.description;
      _body = $v.body;
      _files = $v.files?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentDefinitionSkill other) {
    _$v = other as _$AgentDefinitionSkill;
  }

  @override
  void update(void Function(AgentDefinitionSkillBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentDefinitionSkill build() => _build();

  _$AgentDefinitionSkill _build() {
    _$AgentDefinitionSkill _$result;
    try {
      _$result =
          _$v ??
          _$AgentDefinitionSkill._(
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'AgentDefinitionSkill',
              'name',
            ),
            description: BuiltValueNullFieldError.checkNotNull(
              description,
              r'AgentDefinitionSkill',
              'description',
            ),
            body: BuiltValueNullFieldError.checkNotNull(
              body,
              r'AgentDefinitionSkill',
              'body',
            ),
            files: _files?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'files';
        _files?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentDefinitionSkill',
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

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_definition_skill_file.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentDefinitionSkillFile extends AgentDefinitionSkillFile {
  @override
  final String name;
  @override
  final String content;

  factory _$AgentDefinitionSkillFile([
    void Function(AgentDefinitionSkillFileBuilder)? updates,
  ]) => (AgentDefinitionSkillFileBuilder()..update(updates))._build();

  _$AgentDefinitionSkillFile._({required this.name, required this.content})
    : super._();
  @override
  AgentDefinitionSkillFile rebuild(
    void Function(AgentDefinitionSkillFileBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentDefinitionSkillFileBuilder toBuilder() =>
      AgentDefinitionSkillFileBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentDefinitionSkillFile &&
        name == other.name &&
        content == other.content;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, content.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentDefinitionSkillFile')
          ..add('name', name)
          ..add('content', content))
        .toString();
  }
}

class AgentDefinitionSkillFileBuilder
    implements
        Builder<AgentDefinitionSkillFile, AgentDefinitionSkillFileBuilder> {
  _$AgentDefinitionSkillFile? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _content;
  String? get content => _$this._content;
  set content(String? content) => _$this._content = content;

  AgentDefinitionSkillFileBuilder() {
    AgentDefinitionSkillFile._defaults(this);
  }

  AgentDefinitionSkillFileBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _content = $v.content;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentDefinitionSkillFile other) {
    _$v = other as _$AgentDefinitionSkillFile;
  }

  @override
  void update(void Function(AgentDefinitionSkillFileBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentDefinitionSkillFile build() => _build();

  _$AgentDefinitionSkillFile _build() {
    final _$result =
        _$v ??
        _$AgentDefinitionSkillFile._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'AgentDefinitionSkillFile',
            'name',
          ),
          content: BuiltValueNullFieldError.checkNotNull(
            content,
            r'AgentDefinitionSkillFile',
            'content',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

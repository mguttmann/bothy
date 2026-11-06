// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_definition_entry.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentDefinitionEntry extends AgentDefinitionEntry {
  @override
  final String name;
  @override
  final String? value;
  @override
  final String? secretRef;

  factory _$AgentDefinitionEntry([
    void Function(AgentDefinitionEntryBuilder)? updates,
  ]) => (AgentDefinitionEntryBuilder()..update(updates))._build();

  _$AgentDefinitionEntry._({required this.name, this.value, this.secretRef})
    : super._();
  @override
  AgentDefinitionEntry rebuild(
    void Function(AgentDefinitionEntryBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentDefinitionEntryBuilder toBuilder() =>
      AgentDefinitionEntryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentDefinitionEntry &&
        name == other.name &&
        value == other.value &&
        secretRef == other.secretRef;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, value.hashCode);
    _$hash = $jc(_$hash, secretRef.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentDefinitionEntry')
          ..add('name', name)
          ..add('value', value)
          ..add('secretRef', secretRef))
        .toString();
  }
}

class AgentDefinitionEntryBuilder
    implements Builder<AgentDefinitionEntry, AgentDefinitionEntryBuilder> {
  _$AgentDefinitionEntry? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _value;
  String? get value => _$this._value;
  set value(String? value) => _$this._value = value;

  String? _secretRef;
  String? get secretRef => _$this._secretRef;
  set secretRef(String? secretRef) => _$this._secretRef = secretRef;

  AgentDefinitionEntryBuilder() {
    AgentDefinitionEntry._defaults(this);
  }

  AgentDefinitionEntryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _value = $v.value;
      _secretRef = $v.secretRef;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentDefinitionEntry other) {
    _$v = other as _$AgentDefinitionEntry;
  }

  @override
  void update(void Function(AgentDefinitionEntryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentDefinitionEntry build() => _build();

  _$AgentDefinitionEntry _build() {
    final _$result =
        _$v ??
        _$AgentDefinitionEntry._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'AgentDefinitionEntry',
            'name',
          ),
          value: value,
          secretRef: secretRef,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

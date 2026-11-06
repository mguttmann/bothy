// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_definition_changed_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentDefinitionChangedPayload extends AgentDefinitionChangedPayload {
  @override
  final int definitionVersion;

  factory _$AgentDefinitionChangedPayload([
    void Function(AgentDefinitionChangedPayloadBuilder)? updates,
  ]) => (AgentDefinitionChangedPayloadBuilder()..update(updates))._build();

  _$AgentDefinitionChangedPayload._({required this.definitionVersion})
    : super._();
  @override
  AgentDefinitionChangedPayload rebuild(
    void Function(AgentDefinitionChangedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentDefinitionChangedPayloadBuilder toBuilder() =>
      AgentDefinitionChangedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentDefinitionChangedPayload &&
        definitionVersion == other.definitionVersion;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, definitionVersion.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'AgentDefinitionChangedPayload',
    )..add('definitionVersion', definitionVersion)).toString();
  }
}

class AgentDefinitionChangedPayloadBuilder
    implements
        Builder<
          AgentDefinitionChangedPayload,
          AgentDefinitionChangedPayloadBuilder
        > {
  _$AgentDefinitionChangedPayload? _$v;

  int? _definitionVersion;
  int? get definitionVersion => _$this._definitionVersion;
  set definitionVersion(int? definitionVersion) =>
      _$this._definitionVersion = definitionVersion;

  AgentDefinitionChangedPayloadBuilder() {
    AgentDefinitionChangedPayload._defaults(this);
  }

  AgentDefinitionChangedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _definitionVersion = $v.definitionVersion;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentDefinitionChangedPayload other) {
    _$v = other as _$AgentDefinitionChangedPayload;
  }

  @override
  void update(void Function(AgentDefinitionChangedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentDefinitionChangedPayload build() => _build();

  _$AgentDefinitionChangedPayload _build() {
    final _$result =
        _$v ??
        _$AgentDefinitionChangedPayload._(
          definitionVersion: BuiltValueNullFieldError.checkNotNull(
            definitionVersion,
            r'AgentDefinitionChangedPayload',
            'definitionVersion',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

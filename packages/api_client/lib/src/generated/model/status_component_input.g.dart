// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_component_input.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatusComponentInput extends StatusComponentInput {
  @override
  final String name;
  @override
  final String? description;
  @override
  final StatusComponentState? state;
  @override
  final String? healthSource;
  @override
  final String? agentId;
  @override
  final String? afterComponentId;

  factory _$StatusComponentInput([
    void Function(StatusComponentInputBuilder)? updates,
  ]) => (StatusComponentInputBuilder()..update(updates))._build();

  _$StatusComponentInput._({
    required this.name,
    this.description,
    this.state,
    this.healthSource,
    this.agentId,
    this.afterComponentId,
  }) : super._();
  @override
  StatusComponentInput rebuild(
    void Function(StatusComponentInputBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StatusComponentInputBuilder toBuilder() =>
      StatusComponentInputBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatusComponentInput &&
        name == other.name &&
        description == other.description &&
        state == other.state &&
        healthSource == other.healthSource &&
        agentId == other.agentId &&
        afterComponentId == other.afterComponentId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, healthSource.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, afterComponentId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatusComponentInput')
          ..add('name', name)
          ..add('description', description)
          ..add('state', state)
          ..add('healthSource', healthSource)
          ..add('agentId', agentId)
          ..add('afterComponentId', afterComponentId))
        .toString();
  }
}

class StatusComponentInputBuilder
    implements Builder<StatusComponentInput, StatusComponentInputBuilder> {
  _$StatusComponentInput? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  StatusComponentState? _state;
  StatusComponentState? get state => _$this._state;
  set state(StatusComponentState? state) => _$this._state = state;

  String? _healthSource;
  String? get healthSource => _$this._healthSource;
  set healthSource(String? healthSource) => _$this._healthSource = healthSource;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _afterComponentId;
  String? get afterComponentId => _$this._afterComponentId;
  set afterComponentId(String? afterComponentId) =>
      _$this._afterComponentId = afterComponentId;

  StatusComponentInputBuilder() {
    StatusComponentInput._defaults(this);
  }

  StatusComponentInputBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _description = $v.description;
      _state = $v.state;
      _healthSource = $v.healthSource;
      _agentId = $v.agentId;
      _afterComponentId = $v.afterComponentId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatusComponentInput other) {
    _$v = other as _$StatusComponentInput;
  }

  @override
  void update(void Function(StatusComponentInputBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatusComponentInput build() => _build();

  _$StatusComponentInput _build() {
    final _$result =
        _$v ??
        _$StatusComponentInput._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'StatusComponentInput',
            'name',
          ),
          description: description,
          state: state,
          healthSource: healthSource,
          agentId: agentId,
          afterComponentId: afterComponentId,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

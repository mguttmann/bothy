// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_component.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatusComponent extends StatusComponent {
  @override
  final String id;
  @override
  final String name;
  @override
  final String description;
  @override
  final String sortKey;
  @override
  final StatusComponentState state;
  @override
  final String? healthSource;
  @override
  final String? agentId;
  @override
  final DateTime updatedAt;

  factory _$StatusComponent([void Function(StatusComponentBuilder)? updates]) =>
      (StatusComponentBuilder()..update(updates))._build();

  _$StatusComponent._({
    required this.id,
    required this.name,
    required this.description,
    required this.sortKey,
    required this.state,
    this.healthSource,
    this.agentId,
    required this.updatedAt,
  }) : super._();
  @override
  StatusComponent rebuild(void Function(StatusComponentBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatusComponentBuilder toBuilder() => StatusComponentBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatusComponent &&
        id == other.id &&
        name == other.name &&
        description == other.description &&
        sortKey == other.sortKey &&
        state == other.state &&
        healthSource == other.healthSource &&
        agentId == other.agentId &&
        updatedAt == other.updatedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, sortKey.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, healthSource.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, updatedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatusComponent')
          ..add('id', id)
          ..add('name', name)
          ..add('description', description)
          ..add('sortKey', sortKey)
          ..add('state', state)
          ..add('healthSource', healthSource)
          ..add('agentId', agentId)
          ..add('updatedAt', updatedAt))
        .toString();
  }
}

class StatusComponentBuilder
    implements Builder<StatusComponent, StatusComponentBuilder> {
  _$StatusComponent? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  String? _sortKey;
  String? get sortKey => _$this._sortKey;
  set sortKey(String? sortKey) => _$this._sortKey = sortKey;

  StatusComponentState? _state;
  StatusComponentState? get state => _$this._state;
  set state(StatusComponentState? state) => _$this._state = state;

  String? _healthSource;
  String? get healthSource => _$this._healthSource;
  set healthSource(String? healthSource) => _$this._healthSource = healthSource;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  DateTime? _updatedAt;
  DateTime? get updatedAt => _$this._updatedAt;
  set updatedAt(DateTime? updatedAt) => _$this._updatedAt = updatedAt;

  StatusComponentBuilder() {
    StatusComponent._defaults(this);
  }

  StatusComponentBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _name = $v.name;
      _description = $v.description;
      _sortKey = $v.sortKey;
      _state = $v.state;
      _healthSource = $v.healthSource;
      _agentId = $v.agentId;
      _updatedAt = $v.updatedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatusComponent other) {
    _$v = other as _$StatusComponent;
  }

  @override
  void update(void Function(StatusComponentBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatusComponent build() => _build();

  _$StatusComponent _build() {
    final _$result =
        _$v ??
        _$StatusComponent._(
          id: BuiltValueNullFieldError.checkNotNull(
            id,
            r'StatusComponent',
            'id',
          ),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'StatusComponent',
            'name',
          ),
          description: BuiltValueNullFieldError.checkNotNull(
            description,
            r'StatusComponent',
            'description',
          ),
          sortKey: BuiltValueNullFieldError.checkNotNull(
            sortKey,
            r'StatusComponent',
            'sortKey',
          ),
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'StatusComponent',
            'state',
          ),
          healthSource: healthSource,
          agentId: agentId,
          updatedAt: BuiltValueNullFieldError.checkNotNull(
            updatedAt,
            r'StatusComponent',
            'updatedAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

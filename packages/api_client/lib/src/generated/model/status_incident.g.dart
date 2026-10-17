// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_incident.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatusIncident extends StatusIncident {
  @override
  final String id;
  @override
  final String title;
  @override
  final StatusIncidentKind kind;
  @override
  final StatusIncidentImpact impact;
  @override
  final StatusIncidentState state;
  @override
  final DateTime startedAt;
  @override
  final DateTime? resolvedAt;
  @override
  final BuiltList<String> componentIds;
  @override
  final BuiltList<StatusIncidentUpdate> updates;

  factory _$StatusIncident([void Function(StatusIncidentBuilder)? updates]) =>
      (StatusIncidentBuilder()..update(updates))._build();

  _$StatusIncident._({
    required this.id,
    required this.title,
    required this.kind,
    required this.impact,
    required this.state,
    required this.startedAt,
    this.resolvedAt,
    required this.componentIds,
    required this.updates,
  }) : super._();
  @override
  StatusIncident rebuild(void Function(StatusIncidentBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatusIncidentBuilder toBuilder() => StatusIncidentBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatusIncident &&
        id == other.id &&
        title == other.title &&
        kind == other.kind &&
        impact == other.impact &&
        state == other.state &&
        startedAt == other.startedAt &&
        resolvedAt == other.resolvedAt &&
        componentIds == other.componentIds &&
        updates == other.updates;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, title.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, impact.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, resolvedAt.hashCode);
    _$hash = $jc(_$hash, componentIds.hashCode);
    _$hash = $jc(_$hash, updates.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatusIncident')
          ..add('id', id)
          ..add('title', title)
          ..add('kind', kind)
          ..add('impact', impact)
          ..add('state', state)
          ..add('startedAt', startedAt)
          ..add('resolvedAt', resolvedAt)
          ..add('componentIds', componentIds)
          ..add('updates', updates))
        .toString();
  }
}

class StatusIncidentBuilder
    implements Builder<StatusIncident, StatusIncidentBuilder> {
  _$StatusIncident? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _title;
  String? get title => _$this._title;
  set title(String? title) => _$this._title = title;

  StatusIncidentKind? _kind;
  StatusIncidentKind? get kind => _$this._kind;
  set kind(StatusIncidentKind? kind) => _$this._kind = kind;

  StatusIncidentImpact? _impact;
  StatusIncidentImpact? get impact => _$this._impact;
  set impact(StatusIncidentImpact? impact) => _$this._impact = impact;

  StatusIncidentState? _state;
  StatusIncidentState? get state => _$this._state;
  set state(StatusIncidentState? state) => _$this._state = state;

  DateTime? _startedAt;
  DateTime? get startedAt => _$this._startedAt;
  set startedAt(DateTime? startedAt) => _$this._startedAt = startedAt;

  DateTime? _resolvedAt;
  DateTime? get resolvedAt => _$this._resolvedAt;
  set resolvedAt(DateTime? resolvedAt) => _$this._resolvedAt = resolvedAt;

  ListBuilder<String>? _componentIds;
  ListBuilder<String> get componentIds =>
      _$this._componentIds ??= ListBuilder<String>();
  set componentIds(ListBuilder<String>? componentIds) =>
      _$this._componentIds = componentIds;

  ListBuilder<StatusIncidentUpdate>? _updates;
  ListBuilder<StatusIncidentUpdate> get updates =>
      _$this._updates ??= ListBuilder<StatusIncidentUpdate>();
  set updates(ListBuilder<StatusIncidentUpdate>? updates) =>
      _$this._updates = updates;

  StatusIncidentBuilder() {
    StatusIncident._defaults(this);
  }

  StatusIncidentBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _title = $v.title;
      _kind = $v.kind;
      _impact = $v.impact;
      _state = $v.state;
      _startedAt = $v.startedAt;
      _resolvedAt = $v.resolvedAt;
      _componentIds = $v.componentIds.toBuilder();
      _updates = $v.updates.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatusIncident other) {
    _$v = other as _$StatusIncident;
  }

  @override
  void update(void Function(StatusIncidentBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatusIncident build() => _build();

  _$StatusIncident _build() {
    _$StatusIncident _$result;
    try {
      _$result =
          _$v ??
          _$StatusIncident._(
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'StatusIncident',
              'id',
            ),
            title: BuiltValueNullFieldError.checkNotNull(
              title,
              r'StatusIncident',
              'title',
            ),
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'StatusIncident',
              'kind',
            ),
            impact: BuiltValueNullFieldError.checkNotNull(
              impact,
              r'StatusIncident',
              'impact',
            ),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'StatusIncident',
              'state',
            ),
            startedAt: BuiltValueNullFieldError.checkNotNull(
              startedAt,
              r'StatusIncident',
              'startedAt',
            ),
            resolvedAt: resolvedAt,
            componentIds: componentIds.build(),
            updates: updates.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'componentIds';
        componentIds.build();
        _$failedField = 'updates';
        updates.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatusIncident',
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

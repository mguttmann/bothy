// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_incident_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatusIncidentCreate extends StatusIncidentCreate {
  @override
  final String title;
  @override
  final StatusIncidentKind kind;
  @override
  final StatusIncidentImpact impact;
  @override
  final StatusIncidentState? state;
  @override
  final DateTime startedAt;
  @override
  final BuiltList<String> componentIds;
  @override
  final String bodyMd;

  factory _$StatusIncidentCreate([
    void Function(StatusIncidentCreateBuilder)? updates,
  ]) => (StatusIncidentCreateBuilder()..update(updates))._build();

  _$StatusIncidentCreate._({
    required this.title,
    required this.kind,
    required this.impact,
    this.state,
    required this.startedAt,
    required this.componentIds,
    required this.bodyMd,
  }) : super._();
  @override
  StatusIncidentCreate rebuild(
    void Function(StatusIncidentCreateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StatusIncidentCreateBuilder toBuilder() =>
      StatusIncidentCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatusIncidentCreate &&
        title == other.title &&
        kind == other.kind &&
        impact == other.impact &&
        state == other.state &&
        startedAt == other.startedAt &&
        componentIds == other.componentIds &&
        bodyMd == other.bodyMd;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, title.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, impact.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, componentIds.hashCode);
    _$hash = $jc(_$hash, bodyMd.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatusIncidentCreate')
          ..add('title', title)
          ..add('kind', kind)
          ..add('impact', impact)
          ..add('state', state)
          ..add('startedAt', startedAt)
          ..add('componentIds', componentIds)
          ..add('bodyMd', bodyMd))
        .toString();
  }
}

class StatusIncidentCreateBuilder
    implements Builder<StatusIncidentCreate, StatusIncidentCreateBuilder> {
  _$StatusIncidentCreate? _$v;

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

  ListBuilder<String>? _componentIds;
  ListBuilder<String> get componentIds =>
      _$this._componentIds ??= ListBuilder<String>();
  set componentIds(ListBuilder<String>? componentIds) =>
      _$this._componentIds = componentIds;

  String? _bodyMd;
  String? get bodyMd => _$this._bodyMd;
  set bodyMd(String? bodyMd) => _$this._bodyMd = bodyMd;

  StatusIncidentCreateBuilder() {
    StatusIncidentCreate._defaults(this);
  }

  StatusIncidentCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _title = $v.title;
      _kind = $v.kind;
      _impact = $v.impact;
      _state = $v.state;
      _startedAt = $v.startedAt;
      _componentIds = $v.componentIds.toBuilder();
      _bodyMd = $v.bodyMd;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatusIncidentCreate other) {
    _$v = other as _$StatusIncidentCreate;
  }

  @override
  void update(void Function(StatusIncidentCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatusIncidentCreate build() => _build();

  _$StatusIncidentCreate _build() {
    _$StatusIncidentCreate _$result;
    try {
      _$result =
          _$v ??
          _$StatusIncidentCreate._(
            title: BuiltValueNullFieldError.checkNotNull(
              title,
              r'StatusIncidentCreate',
              'title',
            ),
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'StatusIncidentCreate',
              'kind',
            ),
            impact: BuiltValueNullFieldError.checkNotNull(
              impact,
              r'StatusIncidentCreate',
              'impact',
            ),
            state: state,
            startedAt: BuiltValueNullFieldError.checkNotNull(
              startedAt,
              r'StatusIncidentCreate',
              'startedAt',
            ),
            componentIds: componentIds.build(),
            bodyMd: BuiltValueNullFieldError.checkNotNull(
              bodyMd,
              r'StatusIncidentCreate',
              'bodyMd',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'componentIds';
        componentIds.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatusIncidentCreate',
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

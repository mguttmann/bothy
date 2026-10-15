// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_incident_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatusIncidentUpdate extends StatusIncidentUpdate {
  @override
  final String id;
  @override
  final StatusIncidentState state;
  @override
  final String bodyMd;
  @override
  final DateTime createdAt;

  factory _$StatusIncidentUpdate([
    void Function(StatusIncidentUpdateBuilder)? updates,
  ]) => (StatusIncidentUpdateBuilder()..update(updates))._build();

  _$StatusIncidentUpdate._({
    required this.id,
    required this.state,
    required this.bodyMd,
    required this.createdAt,
  }) : super._();
  @override
  StatusIncidentUpdate rebuild(
    void Function(StatusIncidentUpdateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StatusIncidentUpdateBuilder toBuilder() =>
      StatusIncidentUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatusIncidentUpdate &&
        id == other.id &&
        state == other.state &&
        bodyMd == other.bodyMd &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, bodyMd.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatusIncidentUpdate')
          ..add('id', id)
          ..add('state', state)
          ..add('bodyMd', bodyMd)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class StatusIncidentUpdateBuilder
    implements Builder<StatusIncidentUpdate, StatusIncidentUpdateBuilder> {
  _$StatusIncidentUpdate? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  StatusIncidentState? _state;
  StatusIncidentState? get state => _$this._state;
  set state(StatusIncidentState? state) => _$this._state = state;

  String? _bodyMd;
  String? get bodyMd => _$this._bodyMd;
  set bodyMd(String? bodyMd) => _$this._bodyMd = bodyMd;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  StatusIncidentUpdateBuilder() {
    StatusIncidentUpdate._defaults(this);
  }

  StatusIncidentUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _state = $v.state;
      _bodyMd = $v.bodyMd;
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatusIncidentUpdate other) {
    _$v = other as _$StatusIncidentUpdate;
  }

  @override
  void update(void Function(StatusIncidentUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatusIncidentUpdate build() => _build();

  _$StatusIncidentUpdate _build() {
    final _$result =
        _$v ??
        _$StatusIncidentUpdate._(
          id: BuiltValueNullFieldError.checkNotNull(
            id,
            r'StatusIncidentUpdate',
            'id',
          ),
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'StatusIncidentUpdate',
            'state',
          ),
          bodyMd: BuiltValueNullFieldError.checkNotNull(
            bodyMd,
            r'StatusIncidentUpdate',
            'bodyMd',
          ),
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'StatusIncidentUpdate',
            'createdAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

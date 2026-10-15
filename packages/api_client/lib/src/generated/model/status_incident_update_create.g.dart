// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_incident_update_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatusIncidentUpdateCreate extends StatusIncidentUpdateCreate {
  @override
  final StatusIncidentState state;
  @override
  final String bodyMd;

  factory _$StatusIncidentUpdateCreate([
    void Function(StatusIncidentUpdateCreateBuilder)? updates,
  ]) => (StatusIncidentUpdateCreateBuilder()..update(updates))._build();

  _$StatusIncidentUpdateCreate._({required this.state, required this.bodyMd})
    : super._();
  @override
  StatusIncidentUpdateCreate rebuild(
    void Function(StatusIncidentUpdateCreateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StatusIncidentUpdateCreateBuilder toBuilder() =>
      StatusIncidentUpdateCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatusIncidentUpdateCreate &&
        state == other.state &&
        bodyMd == other.bodyMd;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, bodyMd.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatusIncidentUpdateCreate')
          ..add('state', state)
          ..add('bodyMd', bodyMd))
        .toString();
  }
}

class StatusIncidentUpdateCreateBuilder
    implements
        Builder<StatusIncidentUpdateCreate, StatusIncidentUpdateCreateBuilder> {
  _$StatusIncidentUpdateCreate? _$v;

  StatusIncidentState? _state;
  StatusIncidentState? get state => _$this._state;
  set state(StatusIncidentState? state) => _$this._state = state;

  String? _bodyMd;
  String? get bodyMd => _$this._bodyMd;
  set bodyMd(String? bodyMd) => _$this._bodyMd = bodyMd;

  StatusIncidentUpdateCreateBuilder() {
    StatusIncidentUpdateCreate._defaults(this);
  }

  StatusIncidentUpdateCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _state = $v.state;
      _bodyMd = $v.bodyMd;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatusIncidentUpdateCreate other) {
    _$v = other as _$StatusIncidentUpdateCreate;
  }

  @override
  void update(void Function(StatusIncidentUpdateCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatusIncidentUpdateCreate build() => _build();

  _$StatusIncidentUpdateCreate _build() {
    final _$result =
        _$v ??
        _$StatusIncidentUpdateCreate._(
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'StatusIncidentUpdateCreate',
            'state',
          ),
          bodyMd: BuiltValueNullFieldError.checkNotNull(
            bodyMd,
            r'StatusIncidentUpdateCreate',
            'bodyMd',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

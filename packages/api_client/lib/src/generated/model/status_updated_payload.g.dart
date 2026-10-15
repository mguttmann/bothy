// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_updated_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatusUpdatedPayload extends StatusUpdatedPayload {
  @override
  final StatusComponentState overallState;
  @override
  final BuiltList<String>? changedComponentIds;
  @override
  final BuiltList<String>? changedIncidentIds;

  factory _$StatusUpdatedPayload([
    void Function(StatusUpdatedPayloadBuilder)? updates,
  ]) => (StatusUpdatedPayloadBuilder()..update(updates))._build();

  _$StatusUpdatedPayload._({
    required this.overallState,
    this.changedComponentIds,
    this.changedIncidentIds,
  }) : super._();
  @override
  StatusUpdatedPayload rebuild(
    void Function(StatusUpdatedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StatusUpdatedPayloadBuilder toBuilder() =>
      StatusUpdatedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatusUpdatedPayload &&
        overallState == other.overallState &&
        changedComponentIds == other.changedComponentIds &&
        changedIncidentIds == other.changedIncidentIds;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, overallState.hashCode);
    _$hash = $jc(_$hash, changedComponentIds.hashCode);
    _$hash = $jc(_$hash, changedIncidentIds.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatusUpdatedPayload')
          ..add('overallState', overallState)
          ..add('changedComponentIds', changedComponentIds)
          ..add('changedIncidentIds', changedIncidentIds))
        .toString();
  }
}

class StatusUpdatedPayloadBuilder
    implements Builder<StatusUpdatedPayload, StatusUpdatedPayloadBuilder> {
  _$StatusUpdatedPayload? _$v;

  StatusComponentState? _overallState;
  StatusComponentState? get overallState => _$this._overallState;
  set overallState(StatusComponentState? overallState) =>
      _$this._overallState = overallState;

  ListBuilder<String>? _changedComponentIds;
  ListBuilder<String> get changedComponentIds =>
      _$this._changedComponentIds ??= ListBuilder<String>();
  set changedComponentIds(ListBuilder<String>? changedComponentIds) =>
      _$this._changedComponentIds = changedComponentIds;

  ListBuilder<String>? _changedIncidentIds;
  ListBuilder<String> get changedIncidentIds =>
      _$this._changedIncidentIds ??= ListBuilder<String>();
  set changedIncidentIds(ListBuilder<String>? changedIncidentIds) =>
      _$this._changedIncidentIds = changedIncidentIds;

  StatusUpdatedPayloadBuilder() {
    StatusUpdatedPayload._defaults(this);
  }

  StatusUpdatedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _overallState = $v.overallState;
      _changedComponentIds = $v.changedComponentIds?.toBuilder();
      _changedIncidentIds = $v.changedIncidentIds?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatusUpdatedPayload other) {
    _$v = other as _$StatusUpdatedPayload;
  }

  @override
  void update(void Function(StatusUpdatedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatusUpdatedPayload build() => _build();

  _$StatusUpdatedPayload _build() {
    _$StatusUpdatedPayload _$result;
    try {
      _$result =
          _$v ??
          _$StatusUpdatedPayload._(
            overallState: BuiltValueNullFieldError.checkNotNull(
              overallState,
              r'StatusUpdatedPayload',
              'overallState',
            ),
            changedComponentIds: _changedComponentIds?.build(),
            changedIncidentIds: _changedIncidentIds?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'changedComponentIds';
        _changedComponentIds?.build();
        _$failedField = 'changedIncidentIds';
        _changedIncidentIds?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatusUpdatedPayload',
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

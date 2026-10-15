// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gdpr_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GdprRequest extends GdprRequest {
  @override
  final String id;
  @override
  final String subjectUserId;
  @override
  final GdprRequestKind kind;
  @override
  final GdprRequestState state;
  @override
  final String requestedBy;
  @override
  final String? approvedBy;
  @override
  final String reason;
  @override
  final DateTime dueAt;
  @override
  final DateTime? resultExpiresAt;
  @override
  final BuiltMap<String, JsonObject?>? progress;
  @override
  final String? error;
  @override
  final DateTime createdAt;
  @override
  final DateTime? completedAt;

  factory _$GdprRequest([void Function(GdprRequestBuilder)? updates]) =>
      (GdprRequestBuilder()..update(updates))._build();

  _$GdprRequest._({
    required this.id,
    required this.subjectUserId,
    required this.kind,
    required this.state,
    required this.requestedBy,
    this.approvedBy,
    required this.reason,
    required this.dueAt,
    this.resultExpiresAt,
    this.progress,
    this.error,
    required this.createdAt,
    this.completedAt,
  }) : super._();
  @override
  GdprRequest rebuild(void Function(GdprRequestBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GdprRequestBuilder toBuilder() => GdprRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GdprRequest &&
        id == other.id &&
        subjectUserId == other.subjectUserId &&
        kind == other.kind &&
        state == other.state &&
        requestedBy == other.requestedBy &&
        approvedBy == other.approvedBy &&
        reason == other.reason &&
        dueAt == other.dueAt &&
        resultExpiresAt == other.resultExpiresAt &&
        progress == other.progress &&
        error == other.error &&
        createdAt == other.createdAt &&
        completedAt == other.completedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, subjectUserId.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, requestedBy.hashCode);
    _$hash = $jc(_$hash, approvedBy.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, dueAt.hashCode);
    _$hash = $jc(_$hash, resultExpiresAt.hashCode);
    _$hash = $jc(_$hash, progress.hashCode);
    _$hash = $jc(_$hash, error.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, completedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GdprRequest')
          ..add('id', id)
          ..add('subjectUserId', subjectUserId)
          ..add('kind', kind)
          ..add('state', state)
          ..add('requestedBy', requestedBy)
          ..add('approvedBy', approvedBy)
          ..add('reason', reason)
          ..add('dueAt', dueAt)
          ..add('resultExpiresAt', resultExpiresAt)
          ..add('progress', progress)
          ..add('error', error)
          ..add('createdAt', createdAt)
          ..add('completedAt', completedAt))
        .toString();
  }
}

class GdprRequestBuilder implements Builder<GdprRequest, GdprRequestBuilder> {
  _$GdprRequest? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _subjectUserId;
  String? get subjectUserId => _$this._subjectUserId;
  set subjectUserId(String? subjectUserId) =>
      _$this._subjectUserId = subjectUserId;

  GdprRequestKind? _kind;
  GdprRequestKind? get kind => _$this._kind;
  set kind(GdprRequestKind? kind) => _$this._kind = kind;

  GdprRequestState? _state;
  GdprRequestState? get state => _$this._state;
  set state(GdprRequestState? state) => _$this._state = state;

  String? _requestedBy;
  String? get requestedBy => _$this._requestedBy;
  set requestedBy(String? requestedBy) => _$this._requestedBy = requestedBy;

  String? _approvedBy;
  String? get approvedBy => _$this._approvedBy;
  set approvedBy(String? approvedBy) => _$this._approvedBy = approvedBy;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  DateTime? _dueAt;
  DateTime? get dueAt => _$this._dueAt;
  set dueAt(DateTime? dueAt) => _$this._dueAt = dueAt;

  DateTime? _resultExpiresAt;
  DateTime? get resultExpiresAt => _$this._resultExpiresAt;
  set resultExpiresAt(DateTime? resultExpiresAt) =>
      _$this._resultExpiresAt = resultExpiresAt;

  MapBuilder<String, JsonObject?>? _progress;
  MapBuilder<String, JsonObject?> get progress =>
      _$this._progress ??= MapBuilder<String, JsonObject?>();
  set progress(MapBuilder<String, JsonObject?>? progress) =>
      _$this._progress = progress;

  String? _error;
  String? get error => _$this._error;
  set error(String? error) => _$this._error = error;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _completedAt;
  DateTime? get completedAt => _$this._completedAt;
  set completedAt(DateTime? completedAt) => _$this._completedAt = completedAt;

  GdprRequestBuilder() {
    GdprRequest._defaults(this);
  }

  GdprRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _subjectUserId = $v.subjectUserId;
      _kind = $v.kind;
      _state = $v.state;
      _requestedBy = $v.requestedBy;
      _approvedBy = $v.approvedBy;
      _reason = $v.reason;
      _dueAt = $v.dueAt;
      _resultExpiresAt = $v.resultExpiresAt;
      _progress = $v.progress?.toBuilder();
      _error = $v.error;
      _createdAt = $v.createdAt;
      _completedAt = $v.completedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GdprRequest other) {
    _$v = other as _$GdprRequest;
  }

  @override
  void update(void Function(GdprRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GdprRequest build() => _build();

  _$GdprRequest _build() {
    _$GdprRequest _$result;
    try {
      _$result =
          _$v ??
          _$GdprRequest._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'GdprRequest', 'id'),
            subjectUserId: BuiltValueNullFieldError.checkNotNull(
              subjectUserId,
              r'GdprRequest',
              'subjectUserId',
            ),
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'GdprRequest',
              'kind',
            ),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'GdprRequest',
              'state',
            ),
            requestedBy: BuiltValueNullFieldError.checkNotNull(
              requestedBy,
              r'GdprRequest',
              'requestedBy',
            ),
            approvedBy: approvedBy,
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'GdprRequest',
              'reason',
            ),
            dueAt: BuiltValueNullFieldError.checkNotNull(
              dueAt,
              r'GdprRequest',
              'dueAt',
            ),
            resultExpiresAt: resultExpiresAt,
            progress: _progress?.build(),
            error: error,
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'GdprRequest',
              'createdAt',
            ),
            completedAt: completedAt,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'progress';
        _progress?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GdprRequest',
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

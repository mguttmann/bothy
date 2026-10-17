// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gdpr_request_due_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GdprRequestDuePayload extends GdprRequestDuePayload {
  @override
  final String requestId;
  @override
  final GdprRequestKind kind;
  @override
  final DateTime dueAt;

  factory _$GdprRequestDuePayload([
    void Function(GdprRequestDuePayloadBuilder)? updates,
  ]) => (GdprRequestDuePayloadBuilder()..update(updates))._build();

  _$GdprRequestDuePayload._({
    required this.requestId,
    required this.kind,
    required this.dueAt,
  }) : super._();
  @override
  GdprRequestDuePayload rebuild(
    void Function(GdprRequestDuePayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  GdprRequestDuePayloadBuilder toBuilder() =>
      GdprRequestDuePayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GdprRequestDuePayload &&
        requestId == other.requestId &&
        kind == other.kind &&
        dueAt == other.dueAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, requestId.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, dueAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GdprRequestDuePayload')
          ..add('requestId', requestId)
          ..add('kind', kind)
          ..add('dueAt', dueAt))
        .toString();
  }
}

class GdprRequestDuePayloadBuilder
    implements Builder<GdprRequestDuePayload, GdprRequestDuePayloadBuilder> {
  _$GdprRequestDuePayload? _$v;

  String? _requestId;
  String? get requestId => _$this._requestId;
  set requestId(String? requestId) => _$this._requestId = requestId;

  GdprRequestKind? _kind;
  GdprRequestKind? get kind => _$this._kind;
  set kind(GdprRequestKind? kind) => _$this._kind = kind;

  DateTime? _dueAt;
  DateTime? get dueAt => _$this._dueAt;
  set dueAt(DateTime? dueAt) => _$this._dueAt = dueAt;

  GdprRequestDuePayloadBuilder() {
    GdprRequestDuePayload._defaults(this);
  }

  GdprRequestDuePayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _requestId = $v.requestId;
      _kind = $v.kind;
      _dueAt = $v.dueAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GdprRequestDuePayload other) {
    _$v = other as _$GdprRequestDuePayload;
  }

  @override
  void update(void Function(GdprRequestDuePayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GdprRequestDuePayload build() => _build();

  _$GdprRequestDuePayload _build() {
    final _$result =
        _$v ??
        _$GdprRequestDuePayload._(
          requestId: BuiltValueNullFieldError.checkNotNull(
            requestId,
            r'GdprRequestDuePayload',
            'requestId',
          ),
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'GdprRequestDuePayload',
            'kind',
          ),
          dueAt: BuiltValueNullFieldError.checkNotNull(
            dueAt,
            r'GdprRequestDuePayload',
            'dueAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gdpr_request_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GdprRequestCreate extends GdprRequestCreate {
  @override
  final String? subjectUserId;
  @override
  final GdprRequestKind kind;
  @override
  final String reason;

  factory _$GdprRequestCreate([
    void Function(GdprRequestCreateBuilder)? updates,
  ]) => (GdprRequestCreateBuilder()..update(updates))._build();

  _$GdprRequestCreate._({
    this.subjectUserId,
    required this.kind,
    required this.reason,
  }) : super._();
  @override
  GdprRequestCreate rebuild(void Function(GdprRequestCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GdprRequestCreateBuilder toBuilder() =>
      GdprRequestCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GdprRequestCreate &&
        subjectUserId == other.subjectUserId &&
        kind == other.kind &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, subjectUserId.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GdprRequestCreate')
          ..add('subjectUserId', subjectUserId)
          ..add('kind', kind)
          ..add('reason', reason))
        .toString();
  }
}

class GdprRequestCreateBuilder
    implements Builder<GdprRequestCreate, GdprRequestCreateBuilder> {
  _$GdprRequestCreate? _$v;

  String? _subjectUserId;
  String? get subjectUserId => _$this._subjectUserId;
  set subjectUserId(String? subjectUserId) =>
      _$this._subjectUserId = subjectUserId;

  GdprRequestKind? _kind;
  GdprRequestKind? get kind => _$this._kind;
  set kind(GdprRequestKind? kind) => _$this._kind = kind;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  GdprRequestCreateBuilder() {
    GdprRequestCreate._defaults(this);
  }

  GdprRequestCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _subjectUserId = $v.subjectUserId;
      _kind = $v.kind;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GdprRequestCreate other) {
    _$v = other as _$GdprRequestCreate;
  }

  @override
  void update(void Function(GdprRequestCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GdprRequestCreate build() => _build();

  _$GdprRequestCreate _build() {
    final _$result =
        _$v ??
        _$GdprRequestCreate._(
          subjectUserId: subjectUserId,
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'GdprRequestCreate',
            'kind',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'GdprRequestCreate',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

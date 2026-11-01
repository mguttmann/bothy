// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_trash_check.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ProjectTrashCheck extends ProjectTrashCheck {
  @override
  final bool ok;
  @override
  final String? reason;
  @override
  final DateTime? checkedAt;

  factory _$ProjectTrashCheck([
    void Function(ProjectTrashCheckBuilder)? updates,
  ]) => (ProjectTrashCheckBuilder()..update(updates))._build();

  _$ProjectTrashCheck._({required this.ok, this.reason, this.checkedAt})
    : super._();
  @override
  ProjectTrashCheck rebuild(void Function(ProjectTrashCheckBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ProjectTrashCheckBuilder toBuilder() =>
      ProjectTrashCheckBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProjectTrashCheck &&
        ok == other.ok &&
        reason == other.reason &&
        checkedAt == other.checkedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, ok.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, checkedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProjectTrashCheck')
          ..add('ok', ok)
          ..add('reason', reason)
          ..add('checkedAt', checkedAt))
        .toString();
  }
}

class ProjectTrashCheckBuilder
    implements Builder<ProjectTrashCheck, ProjectTrashCheckBuilder> {
  _$ProjectTrashCheck? _$v;

  bool? _ok;
  bool? get ok => _$this._ok;
  set ok(bool? ok) => _$this._ok = ok;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  DateTime? _checkedAt;
  DateTime? get checkedAt => _$this._checkedAt;
  set checkedAt(DateTime? checkedAt) => _$this._checkedAt = checkedAt;

  ProjectTrashCheckBuilder() {
    ProjectTrashCheck._defaults(this);
  }

  ProjectTrashCheckBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _ok = $v.ok;
      _reason = $v.reason;
      _checkedAt = $v.checkedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProjectTrashCheck other) {
    _$v = other as _$ProjectTrashCheck;
  }

  @override
  void update(void Function(ProjectTrashCheckBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProjectTrashCheck build() => _build();

  _$ProjectTrashCheck _build() {
    final _$result =
        _$v ??
        _$ProjectTrashCheck._(
          ok: BuiltValueNullFieldError.checkNotNull(
            ok,
            r'ProjectTrashCheck',
            'ok',
          ),
          reason: reason,
          checkedAt: checkedAt,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

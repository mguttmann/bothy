// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_trash_check_report.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ProjectTrashCheckReport extends ProjectTrashCheckReport {
  @override
  final bool ok;
  @override
  final String? reason;

  factory _$ProjectTrashCheckReport([
    void Function(ProjectTrashCheckReportBuilder)? updates,
  ]) => (ProjectTrashCheckReportBuilder()..update(updates))._build();

  _$ProjectTrashCheckReport._({required this.ok, this.reason}) : super._();
  @override
  ProjectTrashCheckReport rebuild(
    void Function(ProjectTrashCheckReportBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ProjectTrashCheckReportBuilder toBuilder() =>
      ProjectTrashCheckReportBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProjectTrashCheckReport &&
        ok == other.ok &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, ok.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProjectTrashCheckReport')
          ..add('ok', ok)
          ..add('reason', reason))
        .toString();
  }
}

class ProjectTrashCheckReportBuilder
    implements
        Builder<ProjectTrashCheckReport, ProjectTrashCheckReportBuilder> {
  _$ProjectTrashCheckReport? _$v;

  bool? _ok;
  bool? get ok => _$this._ok;
  set ok(bool? ok) => _$this._ok = ok;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  ProjectTrashCheckReportBuilder() {
    ProjectTrashCheckReport._defaults(this);
  }

  ProjectTrashCheckReportBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _ok = $v.ok;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProjectTrashCheckReport other) {
    _$v = other as _$ProjectTrashCheckReport;
  }

  @override
  void update(void Function(ProjectTrashCheckReportBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProjectTrashCheckReport build() => _build();

  _$ProjectTrashCheckReport _build() {
    final _$result =
        _$v ??
        _$ProjectTrashCheckReport._(
          ok: BuiltValueNullFieldError.checkNotNull(
            ok,
            r'ProjectTrashCheckReport',
            'ok',
          ),
          reason: reason,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

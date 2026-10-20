// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_binding_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ProjectBindingPut extends ProjectBindingPut {
  @override
  final String workspaceRef;
  @override
  final String? repositoryUrl;
  @override
  final String? defaultBranch;
  @override
  final ProjectRunnerConfig? runnerConfig;
  @override
  final String reason;

  factory _$ProjectBindingPut([
    void Function(ProjectBindingPutBuilder)? updates,
  ]) => (ProjectBindingPutBuilder()..update(updates))._build();

  _$ProjectBindingPut._({
    required this.workspaceRef,
    this.repositoryUrl,
    this.defaultBranch,
    this.runnerConfig,
    required this.reason,
  }) : super._();
  @override
  ProjectBindingPut rebuild(void Function(ProjectBindingPutBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ProjectBindingPutBuilder toBuilder() =>
      ProjectBindingPutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProjectBindingPut &&
        workspaceRef == other.workspaceRef &&
        repositoryUrl == other.repositoryUrl &&
        defaultBranch == other.defaultBranch &&
        runnerConfig == other.runnerConfig &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, workspaceRef.hashCode);
    _$hash = $jc(_$hash, repositoryUrl.hashCode);
    _$hash = $jc(_$hash, defaultBranch.hashCode);
    _$hash = $jc(_$hash, runnerConfig.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProjectBindingPut')
          ..add('workspaceRef', workspaceRef)
          ..add('repositoryUrl', repositoryUrl)
          ..add('defaultBranch', defaultBranch)
          ..add('runnerConfig', runnerConfig)
          ..add('reason', reason))
        .toString();
  }
}

class ProjectBindingPutBuilder
    implements Builder<ProjectBindingPut, ProjectBindingPutBuilder> {
  _$ProjectBindingPut? _$v;

  String? _workspaceRef;
  String? get workspaceRef => _$this._workspaceRef;
  set workspaceRef(String? workspaceRef) => _$this._workspaceRef = workspaceRef;

  String? _repositoryUrl;
  String? get repositoryUrl => _$this._repositoryUrl;
  set repositoryUrl(String? repositoryUrl) =>
      _$this._repositoryUrl = repositoryUrl;

  String? _defaultBranch;
  String? get defaultBranch => _$this._defaultBranch;
  set defaultBranch(String? defaultBranch) =>
      _$this._defaultBranch = defaultBranch;

  ProjectRunnerConfigBuilder? _runnerConfig;
  ProjectRunnerConfigBuilder get runnerConfig =>
      _$this._runnerConfig ??= ProjectRunnerConfigBuilder();
  set runnerConfig(ProjectRunnerConfigBuilder? runnerConfig) =>
      _$this._runnerConfig = runnerConfig;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  ProjectBindingPutBuilder() {
    ProjectBindingPut._defaults(this);
  }

  ProjectBindingPutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _workspaceRef = $v.workspaceRef;
      _repositoryUrl = $v.repositoryUrl;
      _defaultBranch = $v.defaultBranch;
      _runnerConfig = $v.runnerConfig?.toBuilder();
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProjectBindingPut other) {
    _$v = other as _$ProjectBindingPut;
  }

  @override
  void update(void Function(ProjectBindingPutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProjectBindingPut build() => _build();

  _$ProjectBindingPut _build() {
    _$ProjectBindingPut _$result;
    try {
      _$result =
          _$v ??
          _$ProjectBindingPut._(
            workspaceRef: BuiltValueNullFieldError.checkNotNull(
              workspaceRef,
              r'ProjectBindingPut',
              'workspaceRef',
            ),
            repositoryUrl: repositoryUrl,
            defaultBranch: defaultBranch,
            runnerConfig: _runnerConfig?.build(),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'ProjectBindingPut',
              'reason',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'runnerConfig';
        _runnerConfig?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ProjectBindingPut',
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

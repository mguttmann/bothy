// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_binding.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ProjectBinding extends ProjectBinding {
  @override
  final String workspaceRef;
  @override
  final String? repositoryUrl;
  @override
  final String? defaultBranch;
  @override
  final ProjectRunnerConfig runnerConfig;
  @override
  final bool ready;
  @override
  final int version;

  factory _$ProjectBinding([void Function(ProjectBindingBuilder)? updates]) =>
      (ProjectBindingBuilder()..update(updates))._build();

  _$ProjectBinding._({
    required this.workspaceRef,
    this.repositoryUrl,
    this.defaultBranch,
    required this.runnerConfig,
    required this.ready,
    required this.version,
  }) : super._();
  @override
  ProjectBinding rebuild(void Function(ProjectBindingBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ProjectBindingBuilder toBuilder() => ProjectBindingBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProjectBinding &&
        workspaceRef == other.workspaceRef &&
        repositoryUrl == other.repositoryUrl &&
        defaultBranch == other.defaultBranch &&
        runnerConfig == other.runnerConfig &&
        ready == other.ready &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, workspaceRef.hashCode);
    _$hash = $jc(_$hash, repositoryUrl.hashCode);
    _$hash = $jc(_$hash, defaultBranch.hashCode);
    _$hash = $jc(_$hash, runnerConfig.hashCode);
    _$hash = $jc(_$hash, ready.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProjectBinding')
          ..add('workspaceRef', workspaceRef)
          ..add('repositoryUrl', repositoryUrl)
          ..add('defaultBranch', defaultBranch)
          ..add('runnerConfig', runnerConfig)
          ..add('ready', ready)
          ..add('version', version))
        .toString();
  }
}

class ProjectBindingBuilder
    implements Builder<ProjectBinding, ProjectBindingBuilder> {
  _$ProjectBinding? _$v;

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

  bool? _ready;
  bool? get ready => _$this._ready;
  set ready(bool? ready) => _$this._ready = ready;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  ProjectBindingBuilder() {
    ProjectBinding._defaults(this);
  }

  ProjectBindingBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _workspaceRef = $v.workspaceRef;
      _repositoryUrl = $v.repositoryUrl;
      _defaultBranch = $v.defaultBranch;
      _runnerConfig = $v.runnerConfig.toBuilder();
      _ready = $v.ready;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProjectBinding other) {
    _$v = other as _$ProjectBinding;
  }

  @override
  void update(void Function(ProjectBindingBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProjectBinding build() => _build();

  _$ProjectBinding _build() {
    _$ProjectBinding _$result;
    try {
      _$result =
          _$v ??
          _$ProjectBinding._(
            workspaceRef: BuiltValueNullFieldError.checkNotNull(
              workspaceRef,
              r'ProjectBinding',
              'workspaceRef',
            ),
            repositoryUrl: repositoryUrl,
            defaultBranch: defaultBranch,
            runnerConfig: runnerConfig.build(),
            ready: BuiltValueNullFieldError.checkNotNull(
              ready,
              r'ProjectBinding',
              'ready',
            ),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'ProjectBinding',
              'version',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'runnerConfig';
        runnerConfig.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ProjectBinding',
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

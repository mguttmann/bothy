// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_runner_config.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ProjectRunnerConfig extends ProjectRunnerConfig {
  @override
  final String? model;
  @override
  final BuiltSet<String>? allowedTools;

  factory _$ProjectRunnerConfig([
    void Function(ProjectRunnerConfigBuilder)? updates,
  ]) => (ProjectRunnerConfigBuilder()..update(updates))._build();

  _$ProjectRunnerConfig._({this.model, this.allowedTools}) : super._();
  @override
  ProjectRunnerConfig rebuild(
    void Function(ProjectRunnerConfigBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ProjectRunnerConfigBuilder toBuilder() =>
      ProjectRunnerConfigBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProjectRunnerConfig &&
        model == other.model &&
        allowedTools == other.allowedTools;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, model.hashCode);
    _$hash = $jc(_$hash, allowedTools.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProjectRunnerConfig')
          ..add('model', model)
          ..add('allowedTools', allowedTools))
        .toString();
  }
}

class ProjectRunnerConfigBuilder
    implements Builder<ProjectRunnerConfig, ProjectRunnerConfigBuilder> {
  _$ProjectRunnerConfig? _$v;

  String? _model;
  String? get model => _$this._model;
  set model(String? model) => _$this._model = model;

  SetBuilder<String>? _allowedTools;
  SetBuilder<String> get allowedTools =>
      _$this._allowedTools ??= SetBuilder<String>();
  set allowedTools(SetBuilder<String>? allowedTools) =>
      _$this._allowedTools = allowedTools;

  ProjectRunnerConfigBuilder() {
    ProjectRunnerConfig._defaults(this);
  }

  ProjectRunnerConfigBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _model = $v.model;
      _allowedTools = $v.allowedTools?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProjectRunnerConfig other) {
    _$v = other as _$ProjectRunnerConfig;
  }

  @override
  void update(void Function(ProjectRunnerConfigBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProjectRunnerConfig build() => _build();

  _$ProjectRunnerConfig _build() {
    _$ProjectRunnerConfig _$result;
    try {
      _$result =
          _$v ??
          _$ProjectRunnerConfig._(
            model: model,
            allowedTools: _allowedTools?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'allowedTools';
        _allowedTools?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ProjectRunnerConfig',
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

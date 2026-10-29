// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_folder.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ProjectFolder extends ProjectFolder {
  @override
  final String name;

  factory _$ProjectFolder([void Function(ProjectFolderBuilder)? updates]) =>
      (ProjectFolderBuilder()..update(updates))._build();

  _$ProjectFolder._({required this.name}) : super._();
  @override
  ProjectFolder rebuild(void Function(ProjectFolderBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ProjectFolderBuilder toBuilder() => ProjectFolderBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProjectFolder && name == other.name;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ProjectFolder',
    )..add('name', name)).toString();
  }
}

class ProjectFolderBuilder
    implements Builder<ProjectFolder, ProjectFolderBuilder> {
  _$ProjectFolder? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  ProjectFolderBuilder() {
    ProjectFolder._defaults(this);
  }

  ProjectFolderBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProjectFolder other) {
    _$v = other as _$ProjectFolder;
  }

  @override
  void update(void Function(ProjectFolderBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProjectFolder build() => _build();

  _$ProjectFolder _build() {
    final _$result =
        _$v ??
        _$ProjectFolder._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'ProjectFolder',
            'name',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

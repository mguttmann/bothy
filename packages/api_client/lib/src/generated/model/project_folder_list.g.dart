// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_folder_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ProjectFolderList extends ProjectFolderList {
  @override
  final BuiltList<ProjectFolder> items;
  @override
  final bool reported;

  factory _$ProjectFolderList([
    void Function(ProjectFolderListBuilder)? updates,
  ]) => (ProjectFolderListBuilder()..update(updates))._build();

  _$ProjectFolderList._({required this.items, required this.reported})
    : super._();
  @override
  ProjectFolderList rebuild(void Function(ProjectFolderListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ProjectFolderListBuilder toBuilder() =>
      ProjectFolderListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProjectFolderList &&
        items == other.items &&
        reported == other.reported;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jc(_$hash, reported.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProjectFolderList')
          ..add('items', items)
          ..add('reported', reported))
        .toString();
  }
}

class ProjectFolderListBuilder
    implements Builder<ProjectFolderList, ProjectFolderListBuilder> {
  _$ProjectFolderList? _$v;

  ListBuilder<ProjectFolder>? _items;
  ListBuilder<ProjectFolder> get items =>
      _$this._items ??= ListBuilder<ProjectFolder>();
  set items(ListBuilder<ProjectFolder>? items) => _$this._items = items;

  bool? _reported;
  bool? get reported => _$this._reported;
  set reported(bool? reported) => _$this._reported = reported;

  ProjectFolderListBuilder() {
    ProjectFolderList._defaults(this);
  }

  ProjectFolderListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _reported = $v.reported;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProjectFolderList other) {
    _$v = other as _$ProjectFolderList;
  }

  @override
  void update(void Function(ProjectFolderListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProjectFolderList build() => _build();

  _$ProjectFolderList _build() {
    _$ProjectFolderList _$result;
    try {
      _$result =
          _$v ??
          _$ProjectFolderList._(
            items: items.build(),
            reported: BuiltValueNullFieldError.checkNotNull(
              reported,
              r'ProjectFolderList',
              'reported',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ProjectFolderList',
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

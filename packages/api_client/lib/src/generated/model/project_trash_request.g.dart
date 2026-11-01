// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_trash_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ProjectTrashRequest extends ProjectTrashRequest {
  @override
  final String channelId;
  @override
  final String workspaceRef;

  factory _$ProjectTrashRequest([
    void Function(ProjectTrashRequestBuilder)? updates,
  ]) => (ProjectTrashRequestBuilder()..update(updates))._build();

  _$ProjectTrashRequest._({required this.channelId, required this.workspaceRef})
    : super._();
  @override
  ProjectTrashRequest rebuild(
    void Function(ProjectTrashRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ProjectTrashRequestBuilder toBuilder() =>
      ProjectTrashRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProjectTrashRequest &&
        channelId == other.channelId &&
        workspaceRef == other.workspaceRef;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, workspaceRef.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProjectTrashRequest')
          ..add('channelId', channelId)
          ..add('workspaceRef', workspaceRef))
        .toString();
  }
}

class ProjectTrashRequestBuilder
    implements Builder<ProjectTrashRequest, ProjectTrashRequestBuilder> {
  _$ProjectTrashRequest? _$v;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _workspaceRef;
  String? get workspaceRef => _$this._workspaceRef;
  set workspaceRef(String? workspaceRef) => _$this._workspaceRef = workspaceRef;

  ProjectTrashRequestBuilder() {
    ProjectTrashRequest._defaults(this);
  }

  ProjectTrashRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channelId = $v.channelId;
      _workspaceRef = $v.workspaceRef;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProjectTrashRequest other) {
    _$v = other as _$ProjectTrashRequest;
  }

  @override
  void update(void Function(ProjectTrashRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProjectTrashRequest build() => _build();

  _$ProjectTrashRequest _build() {
    final _$result =
        _$v ??
        _$ProjectTrashRequest._(
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'ProjectTrashRequest',
            'channelId',
          ),
          workspaceRef: BuiltValueNullFieldError.checkNotNull(
            workspaceRef,
            r'ProjectTrashRequest',
            'workspaceRef',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

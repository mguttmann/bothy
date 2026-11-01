// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_folder_deletion.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ProjectFolderDeletionTrashStateEnum
_$projectFolderDeletionTrashStateEnum_requested =
    const ProjectFolderDeletionTrashStateEnum._('requested');
const ProjectFolderDeletionTrashStateEnum
_$projectFolderDeletionTrashStateEnum_trashed =
    const ProjectFolderDeletionTrashStateEnum._('trashed');
const ProjectFolderDeletionTrashStateEnum
_$projectFolderDeletionTrashStateEnum_refused =
    const ProjectFolderDeletionTrashStateEnum._('refused');
const ProjectFolderDeletionTrashStateEnum
_$projectFolderDeletionTrashStateEnum_unknownDefaultOpenApi =
    const ProjectFolderDeletionTrashStateEnum._('unknownDefaultOpenApi');

ProjectFolderDeletionTrashStateEnum
_$projectFolderDeletionTrashStateEnumValueOf(String name) {
  switch (name) {
    case 'requested':
      return _$projectFolderDeletionTrashStateEnum_requested;
    case 'trashed':
      return _$projectFolderDeletionTrashStateEnum_trashed;
    case 'refused':
      return _$projectFolderDeletionTrashStateEnum_refused;
    case 'unknownDefaultOpenApi':
      return _$projectFolderDeletionTrashStateEnum_unknownDefaultOpenApi;
    default:
      return _$projectFolderDeletionTrashStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ProjectFolderDeletionTrashStateEnum>
_$projectFolderDeletionTrashStateEnumValues =
    BuiltSet<ProjectFolderDeletionTrashStateEnum>(
      const <ProjectFolderDeletionTrashStateEnum>[
        _$projectFolderDeletionTrashStateEnum_requested,
        _$projectFolderDeletionTrashStateEnum_trashed,
        _$projectFolderDeletionTrashStateEnum_refused,
        _$projectFolderDeletionTrashStateEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<ProjectFolderDeletionTrashStateEnum>
_$projectFolderDeletionTrashStateEnumSerializer =
    _$ProjectFolderDeletionTrashStateEnumSerializer();

class _$ProjectFolderDeletionTrashStateEnumSerializer
    implements PrimitiveSerializer<ProjectFolderDeletionTrashStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'requested': 'requested',
    'trashed': 'trashed',
    'refused': 'refused',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'requested': 'requested',
    'trashed': 'trashed',
    'refused': 'refused',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    ProjectFolderDeletionTrashStateEnum,
  ];
  @override
  final String wireName = 'ProjectFolderDeletionTrashStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    ProjectFolderDeletionTrashStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ProjectFolderDeletionTrashStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ProjectFolderDeletionTrashStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ProjectFolderDeletion extends ProjectFolderDeletion {
  @override
  final String workspaceRef;
  @override
  final ProjectTrashCheck? trashCheck;
  @override
  final ProjectFolderDeletionTrashStateEnum? trashState;
  @override
  final String? trashReason;

  factory _$ProjectFolderDeletion([
    void Function(ProjectFolderDeletionBuilder)? updates,
  ]) => (ProjectFolderDeletionBuilder()..update(updates))._build();

  _$ProjectFolderDeletion._({
    required this.workspaceRef,
    this.trashCheck,
    this.trashState,
    this.trashReason,
  }) : super._();
  @override
  ProjectFolderDeletion rebuild(
    void Function(ProjectFolderDeletionBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ProjectFolderDeletionBuilder toBuilder() =>
      ProjectFolderDeletionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ProjectFolderDeletion &&
        workspaceRef == other.workspaceRef &&
        trashCheck == other.trashCheck &&
        trashState == other.trashState &&
        trashReason == other.trashReason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, workspaceRef.hashCode);
    _$hash = $jc(_$hash, trashCheck.hashCode);
    _$hash = $jc(_$hash, trashState.hashCode);
    _$hash = $jc(_$hash, trashReason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ProjectFolderDeletion')
          ..add('workspaceRef', workspaceRef)
          ..add('trashCheck', trashCheck)
          ..add('trashState', trashState)
          ..add('trashReason', trashReason))
        .toString();
  }
}

class ProjectFolderDeletionBuilder
    implements Builder<ProjectFolderDeletion, ProjectFolderDeletionBuilder> {
  _$ProjectFolderDeletion? _$v;

  String? _workspaceRef;
  String? get workspaceRef => _$this._workspaceRef;
  set workspaceRef(String? workspaceRef) => _$this._workspaceRef = workspaceRef;

  ProjectTrashCheckBuilder? _trashCheck;
  ProjectTrashCheckBuilder get trashCheck =>
      _$this._trashCheck ??= ProjectTrashCheckBuilder();
  set trashCheck(ProjectTrashCheckBuilder? trashCheck) =>
      _$this._trashCheck = trashCheck;

  ProjectFolderDeletionTrashStateEnum? _trashState;
  ProjectFolderDeletionTrashStateEnum? get trashState => _$this._trashState;
  set trashState(ProjectFolderDeletionTrashStateEnum? trashState) =>
      _$this._trashState = trashState;

  String? _trashReason;
  String? get trashReason => _$this._trashReason;
  set trashReason(String? trashReason) => _$this._trashReason = trashReason;

  ProjectFolderDeletionBuilder() {
    ProjectFolderDeletion._defaults(this);
  }

  ProjectFolderDeletionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _workspaceRef = $v.workspaceRef;
      _trashCheck = $v.trashCheck?.toBuilder();
      _trashState = $v.trashState;
      _trashReason = $v.trashReason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ProjectFolderDeletion other) {
    _$v = other as _$ProjectFolderDeletion;
  }

  @override
  void update(void Function(ProjectFolderDeletionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ProjectFolderDeletion build() => _build();

  _$ProjectFolderDeletion _build() {
    _$ProjectFolderDeletion _$result;
    try {
      _$result =
          _$v ??
          _$ProjectFolderDeletion._(
            workspaceRef: BuiltValueNullFieldError.checkNotNull(
              workspaceRef,
              r'ProjectFolderDeletion',
              'workspaceRef',
            ),
            trashCheck: _trashCheck?.build(),
            trashState: trashState,
            trashReason: trashReason,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'trashCheck';
        _trashCheck?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ProjectFolderDeletion',
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

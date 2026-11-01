// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'deletion_info.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const DeletionInfoTargetKindEnum _$deletionInfoTargetKindEnum_space =
    const DeletionInfoTargetKindEnum._('space');
const DeletionInfoTargetKindEnum _$deletionInfoTargetKindEnum_channel =
    const DeletionInfoTargetKindEnum._('channel');
const DeletionInfoTargetKindEnum _$deletionInfoTargetKindEnum_thread =
    const DeletionInfoTargetKindEnum._('thread');
const DeletionInfoTargetKindEnum
_$deletionInfoTargetKindEnum_unknownDefaultOpenApi =
    const DeletionInfoTargetKindEnum._('unknownDefaultOpenApi');

DeletionInfoTargetKindEnum _$deletionInfoTargetKindEnumValueOf(String name) {
  switch (name) {
    case 'space':
      return _$deletionInfoTargetKindEnum_space;
    case 'channel':
      return _$deletionInfoTargetKindEnum_channel;
    case 'thread':
      return _$deletionInfoTargetKindEnum_thread;
    case 'unknownDefaultOpenApi':
      return _$deletionInfoTargetKindEnum_unknownDefaultOpenApi;
    default:
      return _$deletionInfoTargetKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<DeletionInfoTargetKindEnum> _$deletionInfoTargetKindEnumValues =
    BuiltSet<DeletionInfoTargetKindEnum>(const <DeletionInfoTargetKindEnum>[
      _$deletionInfoTargetKindEnum_space,
      _$deletionInfoTargetKindEnum_channel,
      _$deletionInfoTargetKindEnum_thread,
      _$deletionInfoTargetKindEnum_unknownDefaultOpenApi,
    ]);

const DeletionInfoStateEnum _$deletionInfoStateEnum_live =
    const DeletionInfoStateEnum._('live');
const DeletionInfoStateEnum _$deletionInfoStateEnum_pending =
    const DeletionInfoStateEnum._('pending');
const DeletionInfoStateEnum _$deletionInfoStateEnum_final_ =
    const DeletionInfoStateEnum._('final_');
const DeletionInfoStateEnum _$deletionInfoStateEnum_unknownDefaultOpenApi =
    const DeletionInfoStateEnum._('unknownDefaultOpenApi');

DeletionInfoStateEnum _$deletionInfoStateEnumValueOf(String name) {
  switch (name) {
    case 'live':
      return _$deletionInfoStateEnum_live;
    case 'pending':
      return _$deletionInfoStateEnum_pending;
    case 'final_':
      return _$deletionInfoStateEnum_final_;
    case 'unknownDefaultOpenApi':
      return _$deletionInfoStateEnum_unknownDefaultOpenApi;
    default:
      return _$deletionInfoStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<DeletionInfoStateEnum> _$deletionInfoStateEnumValues =
    BuiltSet<DeletionInfoStateEnum>(const <DeletionInfoStateEnum>[
      _$deletionInfoStateEnum_live,
      _$deletionInfoStateEnum_pending,
      _$deletionInfoStateEnum_final_,
      _$deletionInfoStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<DeletionInfoTargetKindEnum> _$deletionInfoTargetKindEnumSerializer =
    _$DeletionInfoTargetKindEnumSerializer();
Serializer<DeletionInfoStateEnum> _$deletionInfoStateEnumSerializer =
    _$DeletionInfoStateEnumSerializer();

class _$DeletionInfoTargetKindEnumSerializer
    implements PrimitiveSerializer<DeletionInfoTargetKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'space': 'space',
    'channel': 'channel',
    'thread': 'thread',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'space': 'space',
    'channel': 'channel',
    'thread': 'thread',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[DeletionInfoTargetKindEnum];
  @override
  final String wireName = 'DeletionInfoTargetKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    DeletionInfoTargetKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  DeletionInfoTargetKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => DeletionInfoTargetKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$DeletionInfoStateEnumSerializer
    implements PrimitiveSerializer<DeletionInfoStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'live': 'live',
    'pending': 'pending',
    'final_': 'final',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'live': 'live',
    'pending': 'pending',
    'final': 'final_',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[DeletionInfoStateEnum];
  @override
  final String wireName = 'DeletionInfoStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    DeletionInfoStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  DeletionInfoStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => DeletionInfoStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$DeletionInfo extends DeletionInfo {
  @override
  final DeletionInfoTargetKindEnum targetKind;
  @override
  final String targetId;
  @override
  final DeletionInfoStateEnum state;
  @override
  final DateTime? deletedAt;
  @override
  final DateTime? undoUntil;
  @override
  final DeletionContents contents;
  @override
  final ProjectFolderDeletion? projectFolder;

  factory _$DeletionInfo([void Function(DeletionInfoBuilder)? updates]) =>
      (DeletionInfoBuilder()..update(updates))._build();

  _$DeletionInfo._({
    required this.targetKind,
    required this.targetId,
    required this.state,
    this.deletedAt,
    this.undoUntil,
    required this.contents,
    this.projectFolder,
  }) : super._();
  @override
  DeletionInfo rebuild(void Function(DeletionInfoBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DeletionInfoBuilder toBuilder() => DeletionInfoBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DeletionInfo &&
        targetKind == other.targetKind &&
        targetId == other.targetId &&
        state == other.state &&
        deletedAt == other.deletedAt &&
        undoUntil == other.undoUntil &&
        contents == other.contents &&
        projectFolder == other.projectFolder;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, targetKind.hashCode);
    _$hash = $jc(_$hash, targetId.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, deletedAt.hashCode);
    _$hash = $jc(_$hash, undoUntil.hashCode);
    _$hash = $jc(_$hash, contents.hashCode);
    _$hash = $jc(_$hash, projectFolder.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'DeletionInfo')
          ..add('targetKind', targetKind)
          ..add('targetId', targetId)
          ..add('state', state)
          ..add('deletedAt', deletedAt)
          ..add('undoUntil', undoUntil)
          ..add('contents', contents)
          ..add('projectFolder', projectFolder))
        .toString();
  }
}

class DeletionInfoBuilder
    implements Builder<DeletionInfo, DeletionInfoBuilder> {
  _$DeletionInfo? _$v;

  DeletionInfoTargetKindEnum? _targetKind;
  DeletionInfoTargetKindEnum? get targetKind => _$this._targetKind;
  set targetKind(DeletionInfoTargetKindEnum? targetKind) =>
      _$this._targetKind = targetKind;

  String? _targetId;
  String? get targetId => _$this._targetId;
  set targetId(String? targetId) => _$this._targetId = targetId;

  DeletionInfoStateEnum? _state;
  DeletionInfoStateEnum? get state => _$this._state;
  set state(DeletionInfoStateEnum? state) => _$this._state = state;

  DateTime? _deletedAt;
  DateTime? get deletedAt => _$this._deletedAt;
  set deletedAt(DateTime? deletedAt) => _$this._deletedAt = deletedAt;

  DateTime? _undoUntil;
  DateTime? get undoUntil => _$this._undoUntil;
  set undoUntil(DateTime? undoUntil) => _$this._undoUntil = undoUntil;

  DeletionContentsBuilder? _contents;
  DeletionContentsBuilder get contents =>
      _$this._contents ??= DeletionContentsBuilder();
  set contents(DeletionContentsBuilder? contents) =>
      _$this._contents = contents;

  ProjectFolderDeletionBuilder? _projectFolder;
  ProjectFolderDeletionBuilder get projectFolder =>
      _$this._projectFolder ??= ProjectFolderDeletionBuilder();
  set projectFolder(ProjectFolderDeletionBuilder? projectFolder) =>
      _$this._projectFolder = projectFolder;

  DeletionInfoBuilder() {
    DeletionInfo._defaults(this);
  }

  DeletionInfoBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _targetKind = $v.targetKind;
      _targetId = $v.targetId;
      _state = $v.state;
      _deletedAt = $v.deletedAt;
      _undoUntil = $v.undoUntil;
      _contents = $v.contents.toBuilder();
      _projectFolder = $v.projectFolder?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DeletionInfo other) {
    _$v = other as _$DeletionInfo;
  }

  @override
  void update(void Function(DeletionInfoBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DeletionInfo build() => _build();

  _$DeletionInfo _build() {
    _$DeletionInfo _$result;
    try {
      _$result =
          _$v ??
          _$DeletionInfo._(
            targetKind: BuiltValueNullFieldError.checkNotNull(
              targetKind,
              r'DeletionInfo',
              'targetKind',
            ),
            targetId: BuiltValueNullFieldError.checkNotNull(
              targetId,
              r'DeletionInfo',
              'targetId',
            ),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'DeletionInfo',
              'state',
            ),
            deletedAt: deletedAt,
            undoUntil: undoUntil,
            contents: contents.build(),
            projectFolder: _projectFolder?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'contents';
        contents.build();
        _$failedField = 'projectFolder';
        _projectFolder?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'DeletionInfo',
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

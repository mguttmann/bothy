// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'draft.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Draft extends Draft {
  @override
  final String id;
  @override
  final String? threadId;
  @override
  final String channelId;
  @override
  final String contentMd;
  @override
  final String? replyToMessageId;
  @override
  final BuiltList<String> fileIds;
  @override
  final int version;
  @override
  final String? updatedByDeviceId;
  @override
  final DateTime? updatedAt;

  factory _$Draft([void Function(DraftBuilder)? updates]) =>
      (DraftBuilder()..update(updates))._build();

  _$Draft._({
    required this.id,
    this.threadId,
    required this.channelId,
    required this.contentMd,
    this.replyToMessageId,
    required this.fileIds,
    required this.version,
    this.updatedByDeviceId,
    this.updatedAt,
  }) : super._();
  @override
  Draft rebuild(void Function(DraftBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DraftBuilder toBuilder() => DraftBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Draft &&
        id == other.id &&
        threadId == other.threadId &&
        channelId == other.channelId &&
        contentMd == other.contentMd &&
        replyToMessageId == other.replyToMessageId &&
        fileIds == other.fileIds &&
        version == other.version &&
        updatedByDeviceId == other.updatedByDeviceId &&
        updatedAt == other.updatedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, contentMd.hashCode);
    _$hash = $jc(_$hash, replyToMessageId.hashCode);
    _$hash = $jc(_$hash, fileIds.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, updatedByDeviceId.hashCode);
    _$hash = $jc(_$hash, updatedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Draft')
          ..add('id', id)
          ..add('threadId', threadId)
          ..add('channelId', channelId)
          ..add('contentMd', contentMd)
          ..add('replyToMessageId', replyToMessageId)
          ..add('fileIds', fileIds)
          ..add('version', version)
          ..add('updatedByDeviceId', updatedByDeviceId)
          ..add('updatedAt', updatedAt))
        .toString();
  }
}

class DraftBuilder implements Builder<Draft, DraftBuilder> {
  _$Draft? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _contentMd;
  String? get contentMd => _$this._contentMd;
  set contentMd(String? contentMd) => _$this._contentMd = contentMd;

  String? _replyToMessageId;
  String? get replyToMessageId => _$this._replyToMessageId;
  set replyToMessageId(String? replyToMessageId) =>
      _$this._replyToMessageId = replyToMessageId;

  ListBuilder<String>? _fileIds;
  ListBuilder<String> get fileIds => _$this._fileIds ??= ListBuilder<String>();
  set fileIds(ListBuilder<String>? fileIds) => _$this._fileIds = fileIds;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  String? _updatedByDeviceId;
  String? get updatedByDeviceId => _$this._updatedByDeviceId;
  set updatedByDeviceId(String? updatedByDeviceId) =>
      _$this._updatedByDeviceId = updatedByDeviceId;

  DateTime? _updatedAt;
  DateTime? get updatedAt => _$this._updatedAt;
  set updatedAt(DateTime? updatedAt) => _$this._updatedAt = updatedAt;

  DraftBuilder() {
    Draft._defaults(this);
  }

  DraftBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _contentMd = $v.contentMd;
      _replyToMessageId = $v.replyToMessageId;
      _fileIds = $v.fileIds.toBuilder();
      _version = $v.version;
      _updatedByDeviceId = $v.updatedByDeviceId;
      _updatedAt = $v.updatedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Draft other) {
    _$v = other as _$Draft;
  }

  @override
  void update(void Function(DraftBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Draft build() => _build();

  _$Draft _build() {
    _$Draft _$result;
    try {
      _$result =
          _$v ??
          _$Draft._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'Draft', 'id'),
            threadId: threadId,
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'Draft',
              'channelId',
            ),
            contentMd: BuiltValueNullFieldError.checkNotNull(
              contentMd,
              r'Draft',
              'contentMd',
            ),
            replyToMessageId: replyToMessageId,
            fileIds: fileIds.build(),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'Draft',
              'version',
            ),
            updatedByDeviceId: updatedByDeviceId,
            updatedAt: updatedAt,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'fileIds';
        fileIds.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(r'Draft', _$failedField, e.toString());
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'draft_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$DraftPut extends DraftPut {
  @override
  final String contentMd;
  @override
  final String? replyToMessageId;
  @override
  final BuiltList<String>? fileIds;
  @override
  final int? baseVersion;

  factory _$DraftPut([void Function(DraftPutBuilder)? updates]) =>
      (DraftPutBuilder()..update(updates))._build();

  _$DraftPut._({
    required this.contentMd,
    this.replyToMessageId,
    this.fileIds,
    this.baseVersion,
  }) : super._();
  @override
  DraftPut rebuild(void Function(DraftPutBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DraftPutBuilder toBuilder() => DraftPutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DraftPut &&
        contentMd == other.contentMd &&
        replyToMessageId == other.replyToMessageId &&
        fileIds == other.fileIds &&
        baseVersion == other.baseVersion;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, contentMd.hashCode);
    _$hash = $jc(_$hash, replyToMessageId.hashCode);
    _$hash = $jc(_$hash, fileIds.hashCode);
    _$hash = $jc(_$hash, baseVersion.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'DraftPut')
          ..add('contentMd', contentMd)
          ..add('replyToMessageId', replyToMessageId)
          ..add('fileIds', fileIds)
          ..add('baseVersion', baseVersion))
        .toString();
  }
}

class DraftPutBuilder implements Builder<DraftPut, DraftPutBuilder> {
  _$DraftPut? _$v;

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

  int? _baseVersion;
  int? get baseVersion => _$this._baseVersion;
  set baseVersion(int? baseVersion) => _$this._baseVersion = baseVersion;

  DraftPutBuilder() {
    DraftPut._defaults(this);
  }

  DraftPutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _contentMd = $v.contentMd;
      _replyToMessageId = $v.replyToMessageId;
      _fileIds = $v.fileIds?.toBuilder();
      _baseVersion = $v.baseVersion;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DraftPut other) {
    _$v = other as _$DraftPut;
  }

  @override
  void update(void Function(DraftPutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DraftPut build() => _build();

  _$DraftPut _build() {
    _$DraftPut _$result;
    try {
      _$result =
          _$v ??
          _$DraftPut._(
            contentMd: BuiltValueNullFieldError.checkNotNull(
              contentMd,
              r'DraftPut',
              'contentMd',
            ),
            replyToMessageId: replyToMessageId,
            fileIds: _fileIds?.build(),
            baseVersion: baseVersion,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'fileIds';
        _fileIds?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'DraftPut',
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

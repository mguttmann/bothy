// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_message_data.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportMessageData extends ImportMessageData {
  @override
  final String authorOriginKey;
  @override
  final String contentMd;
  @override
  final DateTime createdAt;
  @override
  final DateTime? editedAt;
  @override
  final String? replyToOriginKey;
  @override
  final BuiltList<String>? attachmentFileIds;

  factory _$ImportMessageData([
    void Function(ImportMessageDataBuilder)? updates,
  ]) => (ImportMessageDataBuilder()..update(updates))._build();

  _$ImportMessageData._({
    required this.authorOriginKey,
    required this.contentMd,
    required this.createdAt,
    this.editedAt,
    this.replyToOriginKey,
    this.attachmentFileIds,
  }) : super._();
  @override
  ImportMessageData rebuild(void Function(ImportMessageDataBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportMessageDataBuilder toBuilder() =>
      ImportMessageDataBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportMessageData &&
        authorOriginKey == other.authorOriginKey &&
        contentMd == other.contentMd &&
        createdAt == other.createdAt &&
        editedAt == other.editedAt &&
        replyToOriginKey == other.replyToOriginKey &&
        attachmentFileIds == other.attachmentFileIds;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, authorOriginKey.hashCode);
    _$hash = $jc(_$hash, contentMd.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, editedAt.hashCode);
    _$hash = $jc(_$hash, replyToOriginKey.hashCode);
    _$hash = $jc(_$hash, attachmentFileIds.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportMessageData')
          ..add('authorOriginKey', authorOriginKey)
          ..add('contentMd', contentMd)
          ..add('createdAt', createdAt)
          ..add('editedAt', editedAt)
          ..add('replyToOriginKey', replyToOriginKey)
          ..add('attachmentFileIds', attachmentFileIds))
        .toString();
  }
}

class ImportMessageDataBuilder
    implements Builder<ImportMessageData, ImportMessageDataBuilder> {
  _$ImportMessageData? _$v;

  String? _authorOriginKey;
  String? get authorOriginKey => _$this._authorOriginKey;
  set authorOriginKey(String? authorOriginKey) =>
      _$this._authorOriginKey = authorOriginKey;

  String? _contentMd;
  String? get contentMd => _$this._contentMd;
  set contentMd(String? contentMd) => _$this._contentMd = contentMd;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _editedAt;
  DateTime? get editedAt => _$this._editedAt;
  set editedAt(DateTime? editedAt) => _$this._editedAt = editedAt;

  String? _replyToOriginKey;
  String? get replyToOriginKey => _$this._replyToOriginKey;
  set replyToOriginKey(String? replyToOriginKey) =>
      _$this._replyToOriginKey = replyToOriginKey;

  ListBuilder<String>? _attachmentFileIds;
  ListBuilder<String> get attachmentFileIds =>
      _$this._attachmentFileIds ??= ListBuilder<String>();
  set attachmentFileIds(ListBuilder<String>? attachmentFileIds) =>
      _$this._attachmentFileIds = attachmentFileIds;

  ImportMessageDataBuilder() {
    ImportMessageData._defaults(this);
  }

  ImportMessageDataBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _authorOriginKey = $v.authorOriginKey;
      _contentMd = $v.contentMd;
      _createdAt = $v.createdAt;
      _editedAt = $v.editedAt;
      _replyToOriginKey = $v.replyToOriginKey;
      _attachmentFileIds = $v.attachmentFileIds?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportMessageData other) {
    _$v = other as _$ImportMessageData;
  }

  @override
  void update(void Function(ImportMessageDataBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportMessageData build() => _build();

  _$ImportMessageData _build() {
    _$ImportMessageData _$result;
    try {
      _$result =
          _$v ??
          _$ImportMessageData._(
            authorOriginKey: BuiltValueNullFieldError.checkNotNull(
              authorOriginKey,
              r'ImportMessageData',
              'authorOriginKey',
            ),
            contentMd: BuiltValueNullFieldError.checkNotNull(
              contentMd,
              r'ImportMessageData',
              'contentMd',
            ),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'ImportMessageData',
              'createdAt',
            ),
            editedAt: editedAt,
            replyToOriginKey: replyToOriginKey,
            attachmentFileIds: _attachmentFileIds?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'attachmentFileIds';
        _attachmentFileIds?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportMessageData',
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

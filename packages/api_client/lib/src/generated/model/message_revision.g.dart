// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'message_revision.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$MessageRevision extends MessageRevision {
  @override
  final int revision;
  @override
  final String contentMd;
  @override
  final String editedBy;
  @override
  final DateTime createdAt;

  factory _$MessageRevision([void Function(MessageRevisionBuilder)? updates]) =>
      (MessageRevisionBuilder()..update(updates))._build();

  _$MessageRevision._({
    required this.revision,
    required this.contentMd,
    required this.editedBy,
    required this.createdAt,
  }) : super._();
  @override
  MessageRevision rebuild(void Function(MessageRevisionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  MessageRevisionBuilder toBuilder() => MessageRevisionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is MessageRevision &&
        revision == other.revision &&
        contentMd == other.contentMd &&
        editedBy == other.editedBy &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, revision.hashCode);
    _$hash = $jc(_$hash, contentMd.hashCode);
    _$hash = $jc(_$hash, editedBy.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'MessageRevision')
          ..add('revision', revision)
          ..add('contentMd', contentMd)
          ..add('editedBy', editedBy)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class MessageRevisionBuilder
    implements Builder<MessageRevision, MessageRevisionBuilder> {
  _$MessageRevision? _$v;

  int? _revision;
  int? get revision => _$this._revision;
  set revision(int? revision) => _$this._revision = revision;

  String? _contentMd;
  String? get contentMd => _$this._contentMd;
  set contentMd(String? contentMd) => _$this._contentMd = contentMd;

  String? _editedBy;
  String? get editedBy => _$this._editedBy;
  set editedBy(String? editedBy) => _$this._editedBy = editedBy;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  MessageRevisionBuilder() {
    MessageRevision._defaults(this);
  }

  MessageRevisionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _revision = $v.revision;
      _contentMd = $v.contentMd;
      _editedBy = $v.editedBy;
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(MessageRevision other) {
    _$v = other as _$MessageRevision;
  }

  @override
  void update(void Function(MessageRevisionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  MessageRevision build() => _build();

  _$MessageRevision _build() {
    final _$result =
        _$v ??
        _$MessageRevision._(
          revision: BuiltValueNullFieldError.checkNotNull(
            revision,
            r'MessageRevision',
            'revision',
          ),
          contentMd: BuiltValueNullFieldError.checkNotNull(
            contentMd,
            r'MessageRevision',
            'contentMd',
          ),
          editedBy: BuiltValueNullFieldError.checkNotNull(
            editedBy,
            r'MessageRevision',
            'editedBy',
          ),
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'MessageRevision',
            'createdAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

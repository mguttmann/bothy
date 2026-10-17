// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'attachment.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Attachment extends Attachment {
  @override
  final int position;
  @override
  final File file;

  factory _$Attachment([void Function(AttachmentBuilder)? updates]) =>
      (AttachmentBuilder()..update(updates))._build();

  _$Attachment._({required this.position, required this.file}) : super._();
  @override
  Attachment rebuild(void Function(AttachmentBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AttachmentBuilder toBuilder() => AttachmentBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Attachment &&
        position == other.position &&
        file == other.file;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, position.hashCode);
    _$hash = $jc(_$hash, file.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Attachment')
          ..add('position', position)
          ..add('file', file))
        .toString();
  }
}

class AttachmentBuilder implements Builder<Attachment, AttachmentBuilder> {
  _$Attachment? _$v;

  int? _position;
  int? get position => _$this._position;
  set position(int? position) => _$this._position = position;

  FileBuilder? _file;
  FileBuilder get file => _$this._file ??= FileBuilder();
  set file(FileBuilder? file) => _$this._file = file;

  AttachmentBuilder() {
    Attachment._defaults(this);
  }

  AttachmentBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _position = $v.position;
      _file = $v.file.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Attachment other) {
    _$v = other as _$Attachment;
  }

  @override
  void update(void Function(AttachmentBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Attachment build() => _build();

  _$Attachment _build() {
    _$Attachment _$result;
    try {
      _$result =
          _$v ??
          _$Attachment._(
            position: BuiltValueNullFieldError.checkNotNull(
              position,
              r'Attachment',
              'position',
            ),
            file: file.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'file';
        file.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Attachment',
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

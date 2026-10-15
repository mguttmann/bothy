// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'file_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$FilePayload extends FilePayload {
  @override
  final File file;

  factory _$FilePayload([void Function(FilePayloadBuilder)? updates]) =>
      (FilePayloadBuilder()..update(updates))._build();

  _$FilePayload._({required this.file}) : super._();
  @override
  FilePayload rebuild(void Function(FilePayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  FilePayloadBuilder toBuilder() => FilePayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is FilePayload && file == other.file;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, file.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'FilePayload',
    )..add('file', file)).toString();
  }
}

class FilePayloadBuilder implements Builder<FilePayload, FilePayloadBuilder> {
  _$FilePayload? _$v;

  FileBuilder? _file;
  FileBuilder get file => _$this._file ??= FileBuilder();
  set file(FileBuilder? file) => _$this._file = file;

  FilePayloadBuilder() {
    FilePayload._defaults(this);
  }

  FilePayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _file = $v.file.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(FilePayload other) {
    _$v = other as _$FilePayload;
  }

  @override
  void update(void Function(FilePayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  FilePayload build() => _build();

  _$FilePayload _build() {
    _$FilePayload _$result;
    try {
      _$result = _$v ?? _$FilePayload._(file: file.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'file';
        file.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'FilePayload',
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

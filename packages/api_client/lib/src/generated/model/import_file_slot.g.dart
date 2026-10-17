// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_file_slot.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportFileSlot extends ImportFileSlot {
  @override
  final ImportItem item;
  @override
  final UploadSession? upload;

  factory _$ImportFileSlot([void Function(ImportFileSlotBuilder)? updates]) =>
      (ImportFileSlotBuilder()..update(updates))._build();

  _$ImportFileSlot._({required this.item, this.upload}) : super._();
  @override
  ImportFileSlot rebuild(void Function(ImportFileSlotBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportFileSlotBuilder toBuilder() => ImportFileSlotBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportFileSlot &&
        item == other.item &&
        upload == other.upload;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, item.hashCode);
    _$hash = $jc(_$hash, upload.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportFileSlot')
          ..add('item', item)
          ..add('upload', upload))
        .toString();
  }
}

class ImportFileSlotBuilder
    implements Builder<ImportFileSlot, ImportFileSlotBuilder> {
  _$ImportFileSlot? _$v;

  ImportItemBuilder? _item;
  ImportItemBuilder get item => _$this._item ??= ImportItemBuilder();
  set item(ImportItemBuilder? item) => _$this._item = item;

  UploadSessionBuilder? _upload;
  UploadSessionBuilder get upload => _$this._upload ??= UploadSessionBuilder();
  set upload(UploadSessionBuilder? upload) => _$this._upload = upload;

  ImportFileSlotBuilder() {
    ImportFileSlot._defaults(this);
  }

  ImportFileSlotBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _item = $v.item.toBuilder();
      _upload = $v.upload?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportFileSlot other) {
    _$v = other as _$ImportFileSlot;
  }

  @override
  void update(void Function(ImportFileSlotBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportFileSlot build() => _build();

  _$ImportFileSlot _build() {
    _$ImportFileSlot _$result;
    try {
      _$result =
          _$v ??
          _$ImportFileSlot._(item: item.build(), upload: _upload?.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'item';
        item.build();
        _$failedField = 'upload';
        _upload?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportFileSlot',
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

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_batch_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportBatchResult extends ImportBatchResult {
  @override
  final BuiltList<ImportItem> items;
  @override
  final int? unreadFloorSeq;

  factory _$ImportBatchResult([
    void Function(ImportBatchResultBuilder)? updates,
  ]) => (ImportBatchResultBuilder()..update(updates))._build();

  _$ImportBatchResult._({required this.items, this.unreadFloorSeq}) : super._();
  @override
  ImportBatchResult rebuild(void Function(ImportBatchResultBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportBatchResultBuilder toBuilder() =>
      ImportBatchResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportBatchResult &&
        items == other.items &&
        unreadFloorSeq == other.unreadFloorSeq;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jc(_$hash, unreadFloorSeq.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportBatchResult')
          ..add('items', items)
          ..add('unreadFloorSeq', unreadFloorSeq))
        .toString();
  }
}

class ImportBatchResultBuilder
    implements Builder<ImportBatchResult, ImportBatchResultBuilder> {
  _$ImportBatchResult? _$v;

  ListBuilder<ImportItem>? _items;
  ListBuilder<ImportItem> get items =>
      _$this._items ??= ListBuilder<ImportItem>();
  set items(ListBuilder<ImportItem>? items) => _$this._items = items;

  int? _unreadFloorSeq;
  int? get unreadFloorSeq => _$this._unreadFloorSeq;
  set unreadFloorSeq(int? unreadFloorSeq) =>
      _$this._unreadFloorSeq = unreadFloorSeq;

  ImportBatchResultBuilder() {
    ImportBatchResult._defaults(this);
  }

  ImportBatchResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _unreadFloorSeq = $v.unreadFloorSeq;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportBatchResult other) {
    _$v = other as _$ImportBatchResult;
  }

  @override
  void update(void Function(ImportBatchResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportBatchResult build() => _build();

  _$ImportBatchResult _build() {
    _$ImportBatchResult _$result;
    try {
      _$result =
          _$v ??
          _$ImportBatchResult._(
            items: items.build(),
            unreadFloorSeq: unreadFloorSeq,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportBatchResult',
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

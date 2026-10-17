// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_batch.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportBatch extends ImportBatch {
  @override
  final String? threadOriginKey;
  @override
  final BuiltList<ImportObject> items;

  factory _$ImportBatch([void Function(ImportBatchBuilder)? updates]) =>
      (ImportBatchBuilder()..update(updates))._build();

  _$ImportBatch._({this.threadOriginKey, required this.items}) : super._();
  @override
  ImportBatch rebuild(void Function(ImportBatchBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportBatchBuilder toBuilder() => ImportBatchBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportBatch &&
        threadOriginKey == other.threadOriginKey &&
        items == other.items;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadOriginKey.hashCode);
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportBatch')
          ..add('threadOriginKey', threadOriginKey)
          ..add('items', items))
        .toString();
  }
}

class ImportBatchBuilder implements Builder<ImportBatch, ImportBatchBuilder> {
  _$ImportBatch? _$v;

  String? _threadOriginKey;
  String? get threadOriginKey => _$this._threadOriginKey;
  set threadOriginKey(String? threadOriginKey) =>
      _$this._threadOriginKey = threadOriginKey;

  ListBuilder<ImportObject>? _items;
  ListBuilder<ImportObject> get items =>
      _$this._items ??= ListBuilder<ImportObject>();
  set items(ListBuilder<ImportObject>? items) => _$this._items = items;

  ImportBatchBuilder() {
    ImportBatch._defaults(this);
  }

  ImportBatchBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadOriginKey = $v.threadOriginKey;
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportBatch other) {
    _$v = other as _$ImportBatch;
  }

  @override
  void update(void Function(ImportBatchBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportBatch build() => _build();

  _$ImportBatch _build() {
    _$ImportBatch _$result;
    try {
      _$result =
          _$v ??
          _$ImportBatch._(
            threadOriginKey: threadOriginKey,
            items: items.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportBatch',
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

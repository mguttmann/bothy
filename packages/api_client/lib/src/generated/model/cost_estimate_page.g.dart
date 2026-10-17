// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'cost_estimate_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CostEstimatePage extends CostEstimatePage {
  @override
  final BuiltList<CostEstimate> items;
  @override
  final String? nextCursor;

  factory _$CostEstimatePage([
    void Function(CostEstimatePageBuilder)? updates,
  ]) => (CostEstimatePageBuilder()..update(updates))._build();

  _$CostEstimatePage._({required this.items, this.nextCursor}) : super._();
  @override
  CostEstimatePage rebuild(void Function(CostEstimatePageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CostEstimatePageBuilder toBuilder() =>
      CostEstimatePageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CostEstimatePage &&
        items == other.items &&
        nextCursor == other.nextCursor;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jc(_$hash, nextCursor.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CostEstimatePage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class CostEstimatePageBuilder
    implements Builder<CostEstimatePage, CostEstimatePageBuilder> {
  _$CostEstimatePage? _$v;

  ListBuilder<CostEstimate>? _items;
  ListBuilder<CostEstimate> get items =>
      _$this._items ??= ListBuilder<CostEstimate>();
  set items(ListBuilder<CostEstimate>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  CostEstimatePageBuilder() {
    CostEstimatePage._defaults(this);
  }

  CostEstimatePageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CostEstimatePage other) {
    _$v = other as _$CostEstimatePage;
  }

  @override
  void update(void Function(CostEstimatePageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CostEstimatePage build() => _build();

  _$CostEstimatePage _build() {
    _$CostEstimatePage _$result;
    try {
      _$result =
          _$v ??
          _$CostEstimatePage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CostEstimatePage',
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

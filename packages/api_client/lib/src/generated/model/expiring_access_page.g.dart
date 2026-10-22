// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'expiring_access_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ExpiringAccessPage extends ExpiringAccessPage {
  @override
  final BuiltList<ExpiringAccessItem> items;
  @override
  final AccessLabels labels;
  @override
  final String? nextCursor;

  factory _$ExpiringAccessPage([
    void Function(ExpiringAccessPageBuilder)? updates,
  ]) => (ExpiringAccessPageBuilder()..update(updates))._build();

  _$ExpiringAccessPage._({
    required this.items,
    required this.labels,
    this.nextCursor,
  }) : super._();
  @override
  ExpiringAccessPage rebuild(
    void Function(ExpiringAccessPageBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ExpiringAccessPageBuilder toBuilder() =>
      ExpiringAccessPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ExpiringAccessPage &&
        items == other.items &&
        labels == other.labels &&
        nextCursor == other.nextCursor;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jc(_$hash, labels.hashCode);
    _$hash = $jc(_$hash, nextCursor.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ExpiringAccessPage')
          ..add('items', items)
          ..add('labels', labels)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class ExpiringAccessPageBuilder
    implements Builder<ExpiringAccessPage, ExpiringAccessPageBuilder> {
  _$ExpiringAccessPage? _$v;

  ListBuilder<ExpiringAccessItem>? _items;
  ListBuilder<ExpiringAccessItem> get items =>
      _$this._items ??= ListBuilder<ExpiringAccessItem>();
  set items(ListBuilder<ExpiringAccessItem>? items) => _$this._items = items;

  AccessLabelsBuilder? _labels;
  AccessLabelsBuilder get labels => _$this._labels ??= AccessLabelsBuilder();
  set labels(AccessLabelsBuilder? labels) => _$this._labels = labels;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  ExpiringAccessPageBuilder() {
    ExpiringAccessPage._defaults(this);
  }

  ExpiringAccessPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _labels = $v.labels.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ExpiringAccessPage other) {
    _$v = other as _$ExpiringAccessPage;
  }

  @override
  void update(void Function(ExpiringAccessPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ExpiringAccessPage build() => _build();

  _$ExpiringAccessPage _build() {
    _$ExpiringAccessPage _$result;
    try {
      _$result =
          _$v ??
          _$ExpiringAccessPage._(
            items: items.build(),
            labels: labels.build(),
            nextCursor: nextCursor,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
        _$failedField = 'labels';
        labels.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ExpiringAccessPage',
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

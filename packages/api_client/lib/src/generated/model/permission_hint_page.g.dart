// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'permission_hint_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PermissionHintPage extends PermissionHintPage {
  @override
  final BuiltList<PermissionHint> items;
  @override
  final AccessLabels labels;
  @override
  final String? nextCursor;

  factory _$PermissionHintPage([
    void Function(PermissionHintPageBuilder)? updates,
  ]) => (PermissionHintPageBuilder()..update(updates))._build();

  _$PermissionHintPage._({
    required this.items,
    required this.labels,
    this.nextCursor,
  }) : super._();
  @override
  PermissionHintPage rebuild(
    void Function(PermissionHintPageBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  PermissionHintPageBuilder toBuilder() =>
      PermissionHintPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PermissionHintPage &&
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
    return (newBuiltValueToStringHelper(r'PermissionHintPage')
          ..add('items', items)
          ..add('labels', labels)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class PermissionHintPageBuilder
    implements Builder<PermissionHintPage, PermissionHintPageBuilder> {
  _$PermissionHintPage? _$v;

  ListBuilder<PermissionHint>? _items;
  ListBuilder<PermissionHint> get items =>
      _$this._items ??= ListBuilder<PermissionHint>();
  set items(ListBuilder<PermissionHint>? items) => _$this._items = items;

  AccessLabelsBuilder? _labels;
  AccessLabelsBuilder get labels => _$this._labels ??= AccessLabelsBuilder();
  set labels(AccessLabelsBuilder? labels) => _$this._labels = labels;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  PermissionHintPageBuilder() {
    PermissionHintPage._defaults(this);
  }

  PermissionHintPageBuilder get _$this {
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
  void replace(PermissionHintPage other) {
    _$v = other as _$PermissionHintPage;
  }

  @override
  void update(void Function(PermissionHintPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PermissionHintPage build() => _build();

  _$PermissionHintPage _build() {
    _$PermissionHintPage _$result;
    try {
      _$result =
          _$v ??
          _$PermissionHintPage._(
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
          r'PermissionHintPage',
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

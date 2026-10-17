// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'feature_flag_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$FeatureFlagList extends FeatureFlagList {
  @override
  final BuiltList<FeatureFlag> items;

  factory _$FeatureFlagList([void Function(FeatureFlagListBuilder)? updates]) =>
      (FeatureFlagListBuilder()..update(updates))._build();

  _$FeatureFlagList._({required this.items}) : super._();
  @override
  FeatureFlagList rebuild(void Function(FeatureFlagListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  FeatureFlagListBuilder toBuilder() => FeatureFlagListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is FeatureFlagList && items == other.items;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'FeatureFlagList',
    )..add('items', items)).toString();
  }
}

class FeatureFlagListBuilder
    implements Builder<FeatureFlagList, FeatureFlagListBuilder> {
  _$FeatureFlagList? _$v;

  ListBuilder<FeatureFlag>? _items;
  ListBuilder<FeatureFlag> get items =>
      _$this._items ??= ListBuilder<FeatureFlag>();
  set items(ListBuilder<FeatureFlag>? items) => _$this._items = items;

  FeatureFlagListBuilder() {
    FeatureFlagList._defaults(this);
  }

  FeatureFlagListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(FeatureFlagList other) {
    _$v = other as _$FeatureFlagList;
  }

  @override
  void update(void Function(FeatureFlagListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  FeatureFlagList build() => _build();

  _$FeatureFlagList _build() {
    _$FeatureFlagList _$result;
    try {
      _$result = _$v ?? _$FeatureFlagList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'FeatureFlagList',
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

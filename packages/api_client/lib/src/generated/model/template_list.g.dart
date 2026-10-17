// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'template_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TemplateList extends TemplateList {
  @override
  final BuiltList<Template> items;

  factory _$TemplateList([void Function(TemplateListBuilder)? updates]) =>
      (TemplateListBuilder()..update(updates))._build();

  _$TemplateList._({required this.items}) : super._();
  @override
  TemplateList rebuild(void Function(TemplateListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TemplateListBuilder toBuilder() => TemplateListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TemplateList && items == other.items;
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
      r'TemplateList',
    )..add('items', items)).toString();
  }
}

class TemplateListBuilder
    implements Builder<TemplateList, TemplateListBuilder> {
  _$TemplateList? _$v;

  ListBuilder<Template>? _items;
  ListBuilder<Template> get items => _$this._items ??= ListBuilder<Template>();
  set items(ListBuilder<Template>? items) => _$this._items = items;

  TemplateListBuilder() {
    TemplateList._defaults(this);
  }

  TemplateListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TemplateList other) {
    _$v = other as _$TemplateList;
  }

  @override
  void update(void Function(TemplateListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TemplateList build() => _build();

  _$TemplateList _build() {
    _$TemplateList _$result;
    try {
      _$result = _$v ?? _$TemplateList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'TemplateList',
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

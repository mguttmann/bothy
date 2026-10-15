// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'identity_provider_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$IdentityProviderList extends IdentityProviderList {
  @override
  final BuiltList<IdentityProvider> items;

  factory _$IdentityProviderList([
    void Function(IdentityProviderListBuilder)? updates,
  ]) => (IdentityProviderListBuilder()..update(updates))._build();

  _$IdentityProviderList._({required this.items}) : super._();
  @override
  IdentityProviderList rebuild(
    void Function(IdentityProviderListBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  IdentityProviderListBuilder toBuilder() =>
      IdentityProviderListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IdentityProviderList && items == other.items;
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
      r'IdentityProviderList',
    )..add('items', items)).toString();
  }
}

class IdentityProviderListBuilder
    implements Builder<IdentityProviderList, IdentityProviderListBuilder> {
  _$IdentityProviderList? _$v;

  ListBuilder<IdentityProvider>? _items;
  ListBuilder<IdentityProvider> get items =>
      _$this._items ??= ListBuilder<IdentityProvider>();
  set items(ListBuilder<IdentityProvider>? items) => _$this._items = items;

  IdentityProviderListBuilder() {
    IdentityProviderList._defaults(this);
  }

  IdentityProviderListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IdentityProviderList other) {
    _$v = other as _$IdentityProviderList;
  }

  @override
  void update(void Function(IdentityProviderListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IdentityProviderList build() => _build();

  _$IdentityProviderList _build() {
    _$IdentityProviderList _$result;
    try {
      _$result = _$v ?? _$IdentityProviderList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'IdentityProviderList',
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

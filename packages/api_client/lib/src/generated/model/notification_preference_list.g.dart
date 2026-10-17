// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_preference_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$NotificationPreferenceList extends NotificationPreferenceList {
  @override
  final BuiltList<NotificationPreference> items;

  factory _$NotificationPreferenceList([
    void Function(NotificationPreferenceListBuilder)? updates,
  ]) => (NotificationPreferenceListBuilder()..update(updates))._build();

  _$NotificationPreferenceList._({required this.items}) : super._();
  @override
  NotificationPreferenceList rebuild(
    void Function(NotificationPreferenceListBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  NotificationPreferenceListBuilder toBuilder() =>
      NotificationPreferenceListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is NotificationPreferenceList && items == other.items;
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
      r'NotificationPreferenceList',
    )..add('items', items)).toString();
  }
}

class NotificationPreferenceListBuilder
    implements
        Builder<NotificationPreferenceList, NotificationPreferenceListBuilder> {
  _$NotificationPreferenceList? _$v;

  ListBuilder<NotificationPreference>? _items;
  ListBuilder<NotificationPreference> get items =>
      _$this._items ??= ListBuilder<NotificationPreference>();
  set items(ListBuilder<NotificationPreference>? items) =>
      _$this._items = items;

  NotificationPreferenceListBuilder() {
    NotificationPreferenceList._defaults(this);
  }

  NotificationPreferenceListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(NotificationPreferenceList other) {
    _$v = other as _$NotificationPreferenceList;
  }

  @override
  void update(void Function(NotificationPreferenceListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  NotificationPreferenceList build() => _build();

  _$NotificationPreferenceList _build() {
    _$NotificationPreferenceList _$result;
    try {
      _$result = _$v ?? _$NotificationPreferenceList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'NotificationPreferenceList',
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

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'quiet_hours_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$QuietHoursList extends QuietHoursList {
  @override
  final BuiltList<QuietHoursRule> items;

  factory _$QuietHoursList([void Function(QuietHoursListBuilder)? updates]) =>
      (QuietHoursListBuilder()..update(updates))._build();

  _$QuietHoursList._({required this.items}) : super._();
  @override
  QuietHoursList rebuild(void Function(QuietHoursListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  QuietHoursListBuilder toBuilder() => QuietHoursListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is QuietHoursList && items == other.items;
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
      r'QuietHoursList',
    )..add('items', items)).toString();
  }
}

class QuietHoursListBuilder
    implements Builder<QuietHoursList, QuietHoursListBuilder> {
  _$QuietHoursList? _$v;

  ListBuilder<QuietHoursRule>? _items;
  ListBuilder<QuietHoursRule> get items =>
      _$this._items ??= ListBuilder<QuietHoursRule>();
  set items(ListBuilder<QuietHoursRule>? items) => _$this._items = items;

  QuietHoursListBuilder() {
    QuietHoursList._defaults(this);
  }

  QuietHoursListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(QuietHoursList other) {
    _$v = other as _$QuietHoursList;
  }

  @override
  void update(void Function(QuietHoursListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  QuietHoursList build() => _build();

  _$QuietHoursList _build() {
    _$QuietHoursList _$result;
    try {
      _$result = _$v ?? _$QuietHoursList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'QuietHoursList',
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

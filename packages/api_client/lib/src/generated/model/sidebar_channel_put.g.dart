// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sidebar_channel_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SidebarChannelPut extends SidebarChannelPut {
  @override
  final BuiltList<String>? threadOrder;
  @override
  final bool? hidden;

  factory _$SidebarChannelPut([
    void Function(SidebarChannelPutBuilder)? updates,
  ]) => (SidebarChannelPutBuilder()..update(updates))._build();

  _$SidebarChannelPut._({this.threadOrder, this.hidden}) : super._();
  @override
  SidebarChannelPut rebuild(void Function(SidebarChannelPutBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SidebarChannelPutBuilder toBuilder() =>
      SidebarChannelPutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SidebarChannelPut &&
        threadOrder == other.threadOrder &&
        hidden == other.hidden;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadOrder.hashCode);
    _$hash = $jc(_$hash, hidden.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SidebarChannelPut')
          ..add('threadOrder', threadOrder)
          ..add('hidden', hidden))
        .toString();
  }
}

class SidebarChannelPutBuilder
    implements Builder<SidebarChannelPut, SidebarChannelPutBuilder> {
  _$SidebarChannelPut? _$v;

  ListBuilder<String>? _threadOrder;
  ListBuilder<String> get threadOrder =>
      _$this._threadOrder ??= ListBuilder<String>();
  set threadOrder(ListBuilder<String>? threadOrder) =>
      _$this._threadOrder = threadOrder;

  bool? _hidden;
  bool? get hidden => _$this._hidden;
  set hidden(bool? hidden) => _$this._hidden = hidden;

  SidebarChannelPutBuilder() {
    SidebarChannelPut._defaults(this);
  }

  SidebarChannelPutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadOrder = $v.threadOrder?.toBuilder();
      _hidden = $v.hidden;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SidebarChannelPut other) {
    _$v = other as _$SidebarChannelPut;
  }

  @override
  void update(void Function(SidebarChannelPutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SidebarChannelPut build() => _build();

  _$SidebarChannelPut _build() {
    _$SidebarChannelPut _$result;
    try {
      _$result =
          _$v ??
          _$SidebarChannelPut._(
            threadOrder: _threadOrder?.build(),
            hidden: hidden,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'threadOrder';
        _threadOrder?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SidebarChannelPut',
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

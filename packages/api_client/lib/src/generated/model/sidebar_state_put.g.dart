// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sidebar_state_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SidebarStatePut extends SidebarStatePut {
  @override
  final bool collapsed;
  @override
  final String? sortKey;
  @override
  final bool? useGlobalOrder;
  @override
  final BuiltList<String>? channelOrder;

  factory _$SidebarStatePut([void Function(SidebarStatePutBuilder)? updates]) =>
      (SidebarStatePutBuilder()..update(updates))._build();

  _$SidebarStatePut._({
    required this.collapsed,
    this.sortKey,
    this.useGlobalOrder,
    this.channelOrder,
  }) : super._();
  @override
  SidebarStatePut rebuild(void Function(SidebarStatePutBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SidebarStatePutBuilder toBuilder() => SidebarStatePutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SidebarStatePut &&
        collapsed == other.collapsed &&
        sortKey == other.sortKey &&
        useGlobalOrder == other.useGlobalOrder &&
        channelOrder == other.channelOrder;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, collapsed.hashCode);
    _$hash = $jc(_$hash, sortKey.hashCode);
    _$hash = $jc(_$hash, useGlobalOrder.hashCode);
    _$hash = $jc(_$hash, channelOrder.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SidebarStatePut')
          ..add('collapsed', collapsed)
          ..add('sortKey', sortKey)
          ..add('useGlobalOrder', useGlobalOrder)
          ..add('channelOrder', channelOrder))
        .toString();
  }
}

class SidebarStatePutBuilder
    implements Builder<SidebarStatePut, SidebarStatePutBuilder> {
  _$SidebarStatePut? _$v;

  bool? _collapsed;
  bool? get collapsed => _$this._collapsed;
  set collapsed(bool? collapsed) => _$this._collapsed = collapsed;

  String? _sortKey;
  String? get sortKey => _$this._sortKey;
  set sortKey(String? sortKey) => _$this._sortKey = sortKey;

  bool? _useGlobalOrder;
  bool? get useGlobalOrder => _$this._useGlobalOrder;
  set useGlobalOrder(bool? useGlobalOrder) =>
      _$this._useGlobalOrder = useGlobalOrder;

  ListBuilder<String>? _channelOrder;
  ListBuilder<String> get channelOrder =>
      _$this._channelOrder ??= ListBuilder<String>();
  set channelOrder(ListBuilder<String>? channelOrder) =>
      _$this._channelOrder = channelOrder;

  SidebarStatePutBuilder() {
    SidebarStatePut._defaults(this);
  }

  SidebarStatePutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _collapsed = $v.collapsed;
      _sortKey = $v.sortKey;
      _useGlobalOrder = $v.useGlobalOrder;
      _channelOrder = $v.channelOrder?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SidebarStatePut other) {
    _$v = other as _$SidebarStatePut;
  }

  @override
  void update(void Function(SidebarStatePutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SidebarStatePut build() => _build();

  _$SidebarStatePut _build() {
    _$SidebarStatePut _$result;
    try {
      _$result =
          _$v ??
          _$SidebarStatePut._(
            collapsed: BuiltValueNullFieldError.checkNotNull(
              collapsed,
              r'SidebarStatePut',
              'collapsed',
            ),
            sortKey: sortKey,
            useGlobalOrder: useGlobalOrder,
            channelOrder: _channelOrder?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'channelOrder';
        _channelOrder?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SidebarStatePut',
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

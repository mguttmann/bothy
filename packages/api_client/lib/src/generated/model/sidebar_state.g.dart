// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sidebar_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SidebarState extends SidebarState {
  @override
  final String spaceId;
  @override
  final bool collapsed;
  @override
  final String? sortKey;
  @override
  final BuiltList<String>? channelOrder;
  @override
  final int version;
  @override
  final String? updatedByDeviceId;

  factory _$SidebarState([void Function(SidebarStateBuilder)? updates]) =>
      (SidebarStateBuilder()..update(updates))._build();

  _$SidebarState._({
    required this.spaceId,
    required this.collapsed,
    this.sortKey,
    this.channelOrder,
    required this.version,
    this.updatedByDeviceId,
  }) : super._();
  @override
  SidebarState rebuild(void Function(SidebarStateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SidebarStateBuilder toBuilder() => SidebarStateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SidebarState &&
        spaceId == other.spaceId &&
        collapsed == other.collapsed &&
        sortKey == other.sortKey &&
        channelOrder == other.channelOrder &&
        version == other.version &&
        updatedByDeviceId == other.updatedByDeviceId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, spaceId.hashCode);
    _$hash = $jc(_$hash, collapsed.hashCode);
    _$hash = $jc(_$hash, sortKey.hashCode);
    _$hash = $jc(_$hash, channelOrder.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, updatedByDeviceId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SidebarState')
          ..add('spaceId', spaceId)
          ..add('collapsed', collapsed)
          ..add('sortKey', sortKey)
          ..add('channelOrder', channelOrder)
          ..add('version', version)
          ..add('updatedByDeviceId', updatedByDeviceId))
        .toString();
  }
}

class SidebarStateBuilder
    implements Builder<SidebarState, SidebarStateBuilder> {
  _$SidebarState? _$v;

  String? _spaceId;
  String? get spaceId => _$this._spaceId;
  set spaceId(String? spaceId) => _$this._spaceId = spaceId;

  bool? _collapsed;
  bool? get collapsed => _$this._collapsed;
  set collapsed(bool? collapsed) => _$this._collapsed = collapsed;

  String? _sortKey;
  String? get sortKey => _$this._sortKey;
  set sortKey(String? sortKey) => _$this._sortKey = sortKey;

  ListBuilder<String>? _channelOrder;
  ListBuilder<String> get channelOrder =>
      _$this._channelOrder ??= ListBuilder<String>();
  set channelOrder(ListBuilder<String>? channelOrder) =>
      _$this._channelOrder = channelOrder;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  String? _updatedByDeviceId;
  String? get updatedByDeviceId => _$this._updatedByDeviceId;
  set updatedByDeviceId(String? updatedByDeviceId) =>
      _$this._updatedByDeviceId = updatedByDeviceId;

  SidebarStateBuilder() {
    SidebarState._defaults(this);
  }

  SidebarStateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _spaceId = $v.spaceId;
      _collapsed = $v.collapsed;
      _sortKey = $v.sortKey;
      _channelOrder = $v.channelOrder?.toBuilder();
      _version = $v.version;
      _updatedByDeviceId = $v.updatedByDeviceId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SidebarState other) {
    _$v = other as _$SidebarState;
  }

  @override
  void update(void Function(SidebarStateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SidebarState build() => _build();

  _$SidebarState _build() {
    _$SidebarState _$result;
    try {
      _$result =
          _$v ??
          _$SidebarState._(
            spaceId: BuiltValueNullFieldError.checkNotNull(
              spaceId,
              r'SidebarState',
              'spaceId',
            ),
            collapsed: BuiltValueNullFieldError.checkNotNull(
              collapsed,
              r'SidebarState',
              'collapsed',
            ),
            sortKey: sortKey,
            channelOrder: _channelOrder?.build(),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'SidebarState',
              'version',
            ),
            updatedByDeviceId: updatedByDeviceId,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'channelOrder';
        _channelOrder?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SidebarState',
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

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sidebar_channel_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SidebarChannelState extends SidebarChannelState {
  @override
  final String channelId;
  @override
  final BuiltList<String> threadOrder;
  @override
  final int version;
  @override
  final String? updatedByDeviceId;

  factory _$SidebarChannelState([
    void Function(SidebarChannelStateBuilder)? updates,
  ]) => (SidebarChannelStateBuilder()..update(updates))._build();

  _$SidebarChannelState._({
    required this.channelId,
    required this.threadOrder,
    required this.version,
    this.updatedByDeviceId,
  }) : super._();
  @override
  SidebarChannelState rebuild(
    void Function(SidebarChannelStateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  SidebarChannelStateBuilder toBuilder() =>
      SidebarChannelStateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SidebarChannelState &&
        channelId == other.channelId &&
        threadOrder == other.threadOrder &&
        version == other.version &&
        updatedByDeviceId == other.updatedByDeviceId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, threadOrder.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, updatedByDeviceId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SidebarChannelState')
          ..add('channelId', channelId)
          ..add('threadOrder', threadOrder)
          ..add('version', version)
          ..add('updatedByDeviceId', updatedByDeviceId))
        .toString();
  }
}

class SidebarChannelStateBuilder
    implements Builder<SidebarChannelState, SidebarChannelStateBuilder> {
  _$SidebarChannelState? _$v;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  ListBuilder<String>? _threadOrder;
  ListBuilder<String> get threadOrder =>
      _$this._threadOrder ??= ListBuilder<String>();
  set threadOrder(ListBuilder<String>? threadOrder) =>
      _$this._threadOrder = threadOrder;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  String? _updatedByDeviceId;
  String? get updatedByDeviceId => _$this._updatedByDeviceId;
  set updatedByDeviceId(String? updatedByDeviceId) =>
      _$this._updatedByDeviceId = updatedByDeviceId;

  SidebarChannelStateBuilder() {
    SidebarChannelState._defaults(this);
  }

  SidebarChannelStateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channelId = $v.channelId;
      _threadOrder = $v.threadOrder.toBuilder();
      _version = $v.version;
      _updatedByDeviceId = $v.updatedByDeviceId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SidebarChannelState other) {
    _$v = other as _$SidebarChannelState;
  }

  @override
  void update(void Function(SidebarChannelStateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SidebarChannelState build() => _build();

  _$SidebarChannelState _build() {
    _$SidebarChannelState _$result;
    try {
      _$result =
          _$v ??
          _$SidebarChannelState._(
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'SidebarChannelState',
              'channelId',
            ),
            threadOrder: threadOrder.build(),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'SidebarChannelState',
              'version',
            ),
            updatedByDeviceId: updatedByDeviceId,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'threadOrder';
        threadOrder.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SidebarChannelState',
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

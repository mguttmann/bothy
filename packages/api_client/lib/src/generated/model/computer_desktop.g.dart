// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'computer_desktop.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ComputerDesktop extends ComputerDesktop {
  @override
  final String hostId;
  @override
  final String name;
  @override
  final HostPlatform platform;
  @override
  final bool online;
  @override
  final bool locked;
  @override
  final bool busy;

  factory _$ComputerDesktop([void Function(ComputerDesktopBuilder)? updates]) =>
      (ComputerDesktopBuilder()..update(updates))._build();

  _$ComputerDesktop._({
    required this.hostId,
    required this.name,
    required this.platform,
    required this.online,
    required this.locked,
    required this.busy,
  }) : super._();
  @override
  ComputerDesktop rebuild(void Function(ComputerDesktopBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ComputerDesktopBuilder toBuilder() => ComputerDesktopBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ComputerDesktop &&
        hostId == other.hostId &&
        name == other.name &&
        platform == other.platform &&
        online == other.online &&
        locked == other.locked &&
        busy == other.busy;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, hostId.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, platform.hashCode);
    _$hash = $jc(_$hash, online.hashCode);
    _$hash = $jc(_$hash, locked.hashCode);
    _$hash = $jc(_$hash, busy.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ComputerDesktop')
          ..add('hostId', hostId)
          ..add('name', name)
          ..add('platform', platform)
          ..add('online', online)
          ..add('locked', locked)
          ..add('busy', busy))
        .toString();
  }
}

class ComputerDesktopBuilder
    implements Builder<ComputerDesktop, ComputerDesktopBuilder> {
  _$ComputerDesktop? _$v;

  String? _hostId;
  String? get hostId => _$this._hostId;
  set hostId(String? hostId) => _$this._hostId = hostId;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  HostPlatform? _platform;
  HostPlatform? get platform => _$this._platform;
  set platform(HostPlatform? platform) => _$this._platform = platform;

  bool? _online;
  bool? get online => _$this._online;
  set online(bool? online) => _$this._online = online;

  bool? _locked;
  bool? get locked => _$this._locked;
  set locked(bool? locked) => _$this._locked = locked;

  bool? _busy;
  bool? get busy => _$this._busy;
  set busy(bool? busy) => _$this._busy = busy;

  ComputerDesktopBuilder() {
    ComputerDesktop._defaults(this);
  }

  ComputerDesktopBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _hostId = $v.hostId;
      _name = $v.name;
      _platform = $v.platform;
      _online = $v.online;
      _locked = $v.locked;
      _busy = $v.busy;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ComputerDesktop other) {
    _$v = other as _$ComputerDesktop;
  }

  @override
  void update(void Function(ComputerDesktopBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ComputerDesktop build() => _build();

  _$ComputerDesktop _build() {
    final _$result =
        _$v ??
        _$ComputerDesktop._(
          hostId: BuiltValueNullFieldError.checkNotNull(
            hostId,
            r'ComputerDesktop',
            'hostId',
          ),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'ComputerDesktop',
            'name',
          ),
          platform: BuiltValueNullFieldError.checkNotNull(
            platform,
            r'ComputerDesktop',
            'platform',
          ),
          online: BuiltValueNullFieldError.checkNotNull(
            online,
            r'ComputerDesktop',
            'online',
          ),
          locked: BuiltValueNullFieldError.checkNotNull(
            locked,
            r'ComputerDesktop',
            'locked',
          ),
          busy: BuiltValueNullFieldError.checkNotNull(
            busy,
            r'ComputerDesktop',
            'busy',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

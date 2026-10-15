// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_ready.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostReady extends HostReady {
  @override
  final String hostId;
  @override
  final HostState state;

  factory _$HostReady([void Function(HostReadyBuilder)? updates]) =>
      (HostReadyBuilder()..update(updates))._build();

  _$HostReady._({required this.hostId, required this.state}) : super._();
  @override
  HostReady rebuild(void Function(HostReadyBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostReadyBuilder toBuilder() => HostReadyBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostReady && hostId == other.hostId && state == other.state;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, hostId.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostReady')
          ..add('hostId', hostId)
          ..add('state', state))
        .toString();
  }
}

class HostReadyBuilder implements Builder<HostReady, HostReadyBuilder> {
  _$HostReady? _$v;

  String? _hostId;
  String? get hostId => _$this._hostId;
  set hostId(String? hostId) => _$this._hostId = hostId;

  HostState? _state;
  HostState? get state => _$this._state;
  set state(HostState? state) => _$this._state = state;

  HostReadyBuilder() {
    HostReady._defaults(this);
  }

  HostReadyBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _hostId = $v.hostId;
      _state = $v.state;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostReady other) {
    _$v = other as _$HostReady;
  }

  @override
  void update(void Function(HostReadyBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostReady build() => _build();

  _$HostReady _build() {
    final _$result =
        _$v ??
        _$HostReady._(
          hostId: BuiltValueNullFieldError.checkNotNull(
            hostId,
            r'HostReady',
            'hostId',
          ),
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'HostReady',
            'state',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

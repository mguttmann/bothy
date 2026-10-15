// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_presence_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostPresencePayload extends HostPresencePayload {
  @override
  final String hostId;
  @override
  final bool connected;

  factory _$HostPresencePayload([
    void Function(HostPresencePayloadBuilder)? updates,
  ]) => (HostPresencePayloadBuilder()..update(updates))._build();

  _$HostPresencePayload._({required this.hostId, required this.connected})
    : super._();
  @override
  HostPresencePayload rebuild(
    void Function(HostPresencePayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  HostPresencePayloadBuilder toBuilder() =>
      HostPresencePayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostPresencePayload &&
        hostId == other.hostId &&
        connected == other.connected;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, hostId.hashCode);
    _$hash = $jc(_$hash, connected.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostPresencePayload')
          ..add('hostId', hostId)
          ..add('connected', connected))
        .toString();
  }
}

class HostPresencePayloadBuilder
    implements Builder<HostPresencePayload, HostPresencePayloadBuilder> {
  _$HostPresencePayload? _$v;

  String? _hostId;
  String? get hostId => _$this._hostId;
  set hostId(String? hostId) => _$this._hostId = hostId;

  bool? _connected;
  bool? get connected => _$this._connected;
  set connected(bool? connected) => _$this._connected = connected;

  HostPresencePayloadBuilder() {
    HostPresencePayload._defaults(this);
  }

  HostPresencePayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _hostId = $v.hostId;
      _connected = $v.connected;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostPresencePayload other) {
    _$v = other as _$HostPresencePayload;
  }

  @override
  void update(void Function(HostPresencePayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostPresencePayload build() => _build();

  _$HostPresencePayload _build() {
    final _$result =
        _$v ??
        _$HostPresencePayload._(
          hostId: BuiltValueNullFieldError.checkNotNull(
            hostId,
            r'HostPresencePayload',
            'hostId',
          ),
          connected: BuiltValueNullFieldError.checkNotNull(
            connected,
            r'HostPresencePayload',
            'connected',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

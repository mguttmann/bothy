// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'computer_session_open.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ComputerSessionOpen extends ComputerSessionOpen {
  @override
  final String hostId;
  @override
  final String clientEphemeralKeyHash;
  @override
  final bool? pinChanged;

  factory _$ComputerSessionOpen([
    void Function(ComputerSessionOpenBuilder)? updates,
  ]) => (ComputerSessionOpenBuilder()..update(updates))._build();

  _$ComputerSessionOpen._({
    required this.hostId,
    required this.clientEphemeralKeyHash,
    this.pinChanged,
  }) : super._();
  @override
  ComputerSessionOpen rebuild(
    void Function(ComputerSessionOpenBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ComputerSessionOpenBuilder toBuilder() =>
      ComputerSessionOpenBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ComputerSessionOpen &&
        hostId == other.hostId &&
        clientEphemeralKeyHash == other.clientEphemeralKeyHash &&
        pinChanged == other.pinChanged;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, hostId.hashCode);
    _$hash = $jc(_$hash, clientEphemeralKeyHash.hashCode);
    _$hash = $jc(_$hash, pinChanged.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ComputerSessionOpen')
          ..add('hostId', hostId)
          ..add('clientEphemeralKeyHash', clientEphemeralKeyHash)
          ..add('pinChanged', pinChanged))
        .toString();
  }
}

class ComputerSessionOpenBuilder
    implements Builder<ComputerSessionOpen, ComputerSessionOpenBuilder> {
  _$ComputerSessionOpen? _$v;

  String? _hostId;
  String? get hostId => _$this._hostId;
  set hostId(String? hostId) => _$this._hostId = hostId;

  String? _clientEphemeralKeyHash;
  String? get clientEphemeralKeyHash => _$this._clientEphemeralKeyHash;
  set clientEphemeralKeyHash(String? clientEphemeralKeyHash) =>
      _$this._clientEphemeralKeyHash = clientEphemeralKeyHash;

  bool? _pinChanged;
  bool? get pinChanged => _$this._pinChanged;
  set pinChanged(bool? pinChanged) => _$this._pinChanged = pinChanged;

  ComputerSessionOpenBuilder() {
    ComputerSessionOpen._defaults(this);
  }

  ComputerSessionOpenBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _hostId = $v.hostId;
      _clientEphemeralKeyHash = $v.clientEphemeralKeyHash;
      _pinChanged = $v.pinChanged;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ComputerSessionOpen other) {
    _$v = other as _$ComputerSessionOpen;
  }

  @override
  void update(void Function(ComputerSessionOpenBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ComputerSessionOpen build() => _build();

  _$ComputerSessionOpen _build() {
    final _$result =
        _$v ??
        _$ComputerSessionOpen._(
          hostId: BuiltValueNullFieldError.checkNotNull(
            hostId,
            r'ComputerSessionOpen',
            'hostId',
          ),
          clientEphemeralKeyHash: BuiltValueNullFieldError.checkNotNull(
            clientEphemeralKeyHash,
            r'ComputerSessionOpen',
            'clientEphemeralKeyHash',
          ),
          pinChanged: pinChanged,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

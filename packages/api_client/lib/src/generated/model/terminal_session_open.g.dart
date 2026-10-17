// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'terminal_session_open.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TerminalSessionOpen extends TerminalSessionOpen {
  @override
  final String profile;
  @override
  final int? workdirIndex;
  @override
  final int cols;
  @override
  final int rows;
  @override
  final String clientEphemeralKeyHash;
  @override
  final bool? pinChanged;

  factory _$TerminalSessionOpen([
    void Function(TerminalSessionOpenBuilder)? updates,
  ]) => (TerminalSessionOpenBuilder()..update(updates))._build();

  _$TerminalSessionOpen._({
    required this.profile,
    this.workdirIndex,
    required this.cols,
    required this.rows,
    required this.clientEphemeralKeyHash,
    this.pinChanged,
  }) : super._();
  @override
  TerminalSessionOpen rebuild(
    void Function(TerminalSessionOpenBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  TerminalSessionOpenBuilder toBuilder() =>
      TerminalSessionOpenBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TerminalSessionOpen &&
        profile == other.profile &&
        workdirIndex == other.workdirIndex &&
        cols == other.cols &&
        rows == other.rows &&
        clientEphemeralKeyHash == other.clientEphemeralKeyHash &&
        pinChanged == other.pinChanged;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, profile.hashCode);
    _$hash = $jc(_$hash, workdirIndex.hashCode);
    _$hash = $jc(_$hash, cols.hashCode);
    _$hash = $jc(_$hash, rows.hashCode);
    _$hash = $jc(_$hash, clientEphemeralKeyHash.hashCode);
    _$hash = $jc(_$hash, pinChanged.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TerminalSessionOpen')
          ..add('profile', profile)
          ..add('workdirIndex', workdirIndex)
          ..add('cols', cols)
          ..add('rows', rows)
          ..add('clientEphemeralKeyHash', clientEphemeralKeyHash)
          ..add('pinChanged', pinChanged))
        .toString();
  }
}

class TerminalSessionOpenBuilder
    implements Builder<TerminalSessionOpen, TerminalSessionOpenBuilder> {
  _$TerminalSessionOpen? _$v;

  String? _profile;
  String? get profile => _$this._profile;
  set profile(String? profile) => _$this._profile = profile;

  int? _workdirIndex;
  int? get workdirIndex => _$this._workdirIndex;
  set workdirIndex(int? workdirIndex) => _$this._workdirIndex = workdirIndex;

  int? _cols;
  int? get cols => _$this._cols;
  set cols(int? cols) => _$this._cols = cols;

  int? _rows;
  int? get rows => _$this._rows;
  set rows(int? rows) => _$this._rows = rows;

  String? _clientEphemeralKeyHash;
  String? get clientEphemeralKeyHash => _$this._clientEphemeralKeyHash;
  set clientEphemeralKeyHash(String? clientEphemeralKeyHash) =>
      _$this._clientEphemeralKeyHash = clientEphemeralKeyHash;

  bool? _pinChanged;
  bool? get pinChanged => _$this._pinChanged;
  set pinChanged(bool? pinChanged) => _$this._pinChanged = pinChanged;

  TerminalSessionOpenBuilder() {
    TerminalSessionOpen._defaults(this);
  }

  TerminalSessionOpenBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _profile = $v.profile;
      _workdirIndex = $v.workdirIndex;
      _cols = $v.cols;
      _rows = $v.rows;
      _clientEphemeralKeyHash = $v.clientEphemeralKeyHash;
      _pinChanged = $v.pinChanged;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TerminalSessionOpen other) {
    _$v = other as _$TerminalSessionOpen;
  }

  @override
  void update(void Function(TerminalSessionOpenBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TerminalSessionOpen build() => _build();

  _$TerminalSessionOpen _build() {
    final _$result =
        _$v ??
        _$TerminalSessionOpen._(
          profile: BuiltValueNullFieldError.checkNotNull(
            profile,
            r'TerminalSessionOpen',
            'profile',
          ),
          workdirIndex: workdirIndex,
          cols: BuiltValueNullFieldError.checkNotNull(
            cols,
            r'TerminalSessionOpen',
            'cols',
          ),
          rows: BuiltValueNullFieldError.checkNotNull(
            rows,
            r'TerminalSessionOpen',
            'rows',
          ),
          clientEphemeralKeyHash: BuiltValueNullFieldError.checkNotNull(
            clientEphemeralKeyHash,
            r'TerminalSessionOpen',
            'clientEphemeralKeyHash',
          ),
          pinChanged: pinChanged,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

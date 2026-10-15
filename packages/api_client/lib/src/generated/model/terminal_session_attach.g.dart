// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'terminal_session_attach.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TerminalSessionAttach extends TerminalSessionAttach {
  @override
  final int cols;
  @override
  final int rows;
  @override
  final String clientEphemeralKeyHash;
  @override
  final bool? pinChanged;

  factory _$TerminalSessionAttach([
    void Function(TerminalSessionAttachBuilder)? updates,
  ]) => (TerminalSessionAttachBuilder()..update(updates))._build();

  _$TerminalSessionAttach._({
    required this.cols,
    required this.rows,
    required this.clientEphemeralKeyHash,
    this.pinChanged,
  }) : super._();
  @override
  TerminalSessionAttach rebuild(
    void Function(TerminalSessionAttachBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  TerminalSessionAttachBuilder toBuilder() =>
      TerminalSessionAttachBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TerminalSessionAttach &&
        cols == other.cols &&
        rows == other.rows &&
        clientEphemeralKeyHash == other.clientEphemeralKeyHash &&
        pinChanged == other.pinChanged;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, cols.hashCode);
    _$hash = $jc(_$hash, rows.hashCode);
    _$hash = $jc(_$hash, clientEphemeralKeyHash.hashCode);
    _$hash = $jc(_$hash, pinChanged.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TerminalSessionAttach')
          ..add('cols', cols)
          ..add('rows', rows)
          ..add('clientEphemeralKeyHash', clientEphemeralKeyHash)
          ..add('pinChanged', pinChanged))
        .toString();
  }
}

class TerminalSessionAttachBuilder
    implements Builder<TerminalSessionAttach, TerminalSessionAttachBuilder> {
  _$TerminalSessionAttach? _$v;

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

  TerminalSessionAttachBuilder() {
    TerminalSessionAttach._defaults(this);
  }

  TerminalSessionAttachBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _cols = $v.cols;
      _rows = $v.rows;
      _clientEphemeralKeyHash = $v.clientEphemeralKeyHash;
      _pinChanged = $v.pinChanged;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TerminalSessionAttach other) {
    _$v = other as _$TerminalSessionAttach;
  }

  @override
  void update(void Function(TerminalSessionAttachBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TerminalSessionAttach build() => _build();

  _$TerminalSessionAttach _build() {
    final _$result =
        _$v ??
        _$TerminalSessionAttach._(
          cols: BuiltValueNullFieldError.checkNotNull(
            cols,
            r'TerminalSessionAttach',
            'cols',
          ),
          rows: BuiltValueNullFieldError.checkNotNull(
            rows,
            r'TerminalSessionAttach',
            'rows',
          ),
          clientEphemeralKeyHash: BuiltValueNullFieldError.checkNotNull(
            clientEphemeralKeyHash,
            r'TerminalSessionAttach',
            'clientEphemeralKeyHash',
          ),
          pinChanged: pinChanged,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

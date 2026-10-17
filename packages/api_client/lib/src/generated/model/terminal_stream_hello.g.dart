// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'terminal_stream_hello.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TerminalStreamHello extends TerminalStreamHello {
  @override
  final BuiltList<int> protocolVersions;
  @override
  final String dpopNonce;
  @override
  final DateTime serverTime;

  factory _$TerminalStreamHello([
    void Function(TerminalStreamHelloBuilder)? updates,
  ]) => (TerminalStreamHelloBuilder()..update(updates))._build();

  _$TerminalStreamHello._({
    required this.protocolVersions,
    required this.dpopNonce,
    required this.serverTime,
  }) : super._();
  @override
  TerminalStreamHello rebuild(
    void Function(TerminalStreamHelloBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  TerminalStreamHelloBuilder toBuilder() =>
      TerminalStreamHelloBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TerminalStreamHello &&
        protocolVersions == other.protocolVersions &&
        dpopNonce == other.dpopNonce &&
        serverTime == other.serverTime;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, protocolVersions.hashCode);
    _$hash = $jc(_$hash, dpopNonce.hashCode);
    _$hash = $jc(_$hash, serverTime.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TerminalStreamHello')
          ..add('protocolVersions', protocolVersions)
          ..add('dpopNonce', dpopNonce)
          ..add('serverTime', serverTime))
        .toString();
  }
}

class TerminalStreamHelloBuilder
    implements Builder<TerminalStreamHello, TerminalStreamHelloBuilder> {
  _$TerminalStreamHello? _$v;

  ListBuilder<int>? _protocolVersions;
  ListBuilder<int> get protocolVersions =>
      _$this._protocolVersions ??= ListBuilder<int>();
  set protocolVersions(ListBuilder<int>? protocolVersions) =>
      _$this._protocolVersions = protocolVersions;

  String? _dpopNonce;
  String? get dpopNonce => _$this._dpopNonce;
  set dpopNonce(String? dpopNonce) => _$this._dpopNonce = dpopNonce;

  DateTime? _serverTime;
  DateTime? get serverTime => _$this._serverTime;
  set serverTime(DateTime? serverTime) => _$this._serverTime = serverTime;

  TerminalStreamHelloBuilder() {
    TerminalStreamHello._defaults(this);
  }

  TerminalStreamHelloBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _protocolVersions = $v.protocolVersions.toBuilder();
      _dpopNonce = $v.dpopNonce;
      _serverTime = $v.serverTime;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TerminalStreamHello other) {
    _$v = other as _$TerminalStreamHello;
  }

  @override
  void update(void Function(TerminalStreamHelloBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TerminalStreamHello build() => _build();

  _$TerminalStreamHello _build() {
    _$TerminalStreamHello _$result;
    try {
      _$result =
          _$v ??
          _$TerminalStreamHello._(
            protocolVersions: protocolVersions.build(),
            dpopNonce: BuiltValueNullFieldError.checkNotNull(
              dpopNonce,
              r'TerminalStreamHello',
              'dpopNonce',
            ),
            serverTime: BuiltValueNullFieldError.checkNotNull(
              serverTime,
              r'TerminalStreamHello',
              'serverTime',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'protocolVersions';
        protocolVersions.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'TerminalStreamHello',
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

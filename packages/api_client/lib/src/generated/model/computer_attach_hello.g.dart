// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'computer_attach_hello.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ComputerAttachHello extends ComputerAttachHello {
  @override
  final BuiltList<int> protocolVersions;
  @override
  final DateTime serverTime;

  factory _$ComputerAttachHello([
    void Function(ComputerAttachHelloBuilder)? updates,
  ]) => (ComputerAttachHelloBuilder()..update(updates))._build();

  _$ComputerAttachHello._({
    required this.protocolVersions,
    required this.serverTime,
  }) : super._();
  @override
  ComputerAttachHello rebuild(
    void Function(ComputerAttachHelloBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ComputerAttachHelloBuilder toBuilder() =>
      ComputerAttachHelloBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ComputerAttachHello &&
        protocolVersions == other.protocolVersions &&
        serverTime == other.serverTime;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, protocolVersions.hashCode);
    _$hash = $jc(_$hash, serverTime.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ComputerAttachHello')
          ..add('protocolVersions', protocolVersions)
          ..add('serverTime', serverTime))
        .toString();
  }
}

class ComputerAttachHelloBuilder
    implements Builder<ComputerAttachHello, ComputerAttachHelloBuilder> {
  _$ComputerAttachHello? _$v;

  ListBuilder<int>? _protocolVersions;
  ListBuilder<int> get protocolVersions =>
      _$this._protocolVersions ??= ListBuilder<int>();
  set protocolVersions(ListBuilder<int>? protocolVersions) =>
      _$this._protocolVersions = protocolVersions;

  DateTime? _serverTime;
  DateTime? get serverTime => _$this._serverTime;
  set serverTime(DateTime? serverTime) => _$this._serverTime = serverTime;

  ComputerAttachHelloBuilder() {
    ComputerAttachHello._defaults(this);
  }

  ComputerAttachHelloBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _protocolVersions = $v.protocolVersions.toBuilder();
      _serverTime = $v.serverTime;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ComputerAttachHello other) {
    _$v = other as _$ComputerAttachHello;
  }

  @override
  void update(void Function(ComputerAttachHelloBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ComputerAttachHello build() => _build();

  _$ComputerAttachHello _build() {
    _$ComputerAttachHello _$result;
    try {
      _$result =
          _$v ??
          _$ComputerAttachHello._(
            protocolVersions: protocolVersions.build(),
            serverTime: BuiltValueNullFieldError.checkNotNull(
              serverTime,
              r'ComputerAttachHello',
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
          r'ComputerAttachHello',
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

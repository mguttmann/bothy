// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_computer_app.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostComputerApp extends HostComputerApp {
  @override
  final String id;
  @override
  final String name;
  @override
  final String signer;

  factory _$HostComputerApp([void Function(HostComputerAppBuilder)? updates]) =>
      (HostComputerAppBuilder()..update(updates))._build();

  _$HostComputerApp._({
    required this.id,
    required this.name,
    required this.signer,
  }) : super._();
  @override
  HostComputerApp rebuild(void Function(HostComputerAppBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostComputerAppBuilder toBuilder() => HostComputerAppBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostComputerApp &&
        id == other.id &&
        name == other.name &&
        signer == other.signer;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, signer.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostComputerApp')
          ..add('id', id)
          ..add('name', name)
          ..add('signer', signer))
        .toString();
  }
}

class HostComputerAppBuilder
    implements Builder<HostComputerApp, HostComputerAppBuilder> {
  _$HostComputerApp? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _signer;
  String? get signer => _$this._signer;
  set signer(String? signer) => _$this._signer = signer;

  HostComputerAppBuilder() {
    HostComputerApp._defaults(this);
  }

  HostComputerAppBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _name = $v.name;
      _signer = $v.signer;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostComputerApp other) {
    _$v = other as _$HostComputerApp;
  }

  @override
  void update(void Function(HostComputerAppBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostComputerApp build() => _build();

  _$HostComputerApp _build() {
    final _$result =
        _$v ??
        _$HostComputerApp._(
          id: BuiltValueNullFieldError.checkNotNull(
            id,
            r'HostComputerApp',
            'id',
          ),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'HostComputerApp',
            'name',
          ),
          signer: BuiltValueNullFieldError.checkNotNull(
            signer,
            r'HostComputerApp',
            'signer',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

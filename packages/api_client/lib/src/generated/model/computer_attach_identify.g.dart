// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'computer_attach_identify.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ComputerAttachIdentify extends ComputerAttachIdentify {
  @override
  final int protocolVersion;
  @override
  final String token;

  factory _$ComputerAttachIdentify([
    void Function(ComputerAttachIdentifyBuilder)? updates,
  ]) => (ComputerAttachIdentifyBuilder()..update(updates))._build();

  _$ComputerAttachIdentify._({
    required this.protocolVersion,
    required this.token,
  }) : super._();
  @override
  ComputerAttachIdentify rebuild(
    void Function(ComputerAttachIdentifyBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ComputerAttachIdentifyBuilder toBuilder() =>
      ComputerAttachIdentifyBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ComputerAttachIdentify &&
        protocolVersion == other.protocolVersion &&
        token == other.token;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, protocolVersion.hashCode);
    _$hash = $jc(_$hash, token.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ComputerAttachIdentify')
          ..add('protocolVersion', protocolVersion)
          ..add('token', token))
        .toString();
  }
}

class ComputerAttachIdentifyBuilder
    implements Builder<ComputerAttachIdentify, ComputerAttachIdentifyBuilder> {
  _$ComputerAttachIdentify? _$v;

  int? _protocolVersion;
  int? get protocolVersion => _$this._protocolVersion;
  set protocolVersion(int? protocolVersion) =>
      _$this._protocolVersion = protocolVersion;

  String? _token;
  String? get token => _$this._token;
  set token(String? token) => _$this._token = token;

  ComputerAttachIdentifyBuilder() {
    ComputerAttachIdentify._defaults(this);
  }

  ComputerAttachIdentifyBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _protocolVersion = $v.protocolVersion;
      _token = $v.token;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ComputerAttachIdentify other) {
    _$v = other as _$ComputerAttachIdentify;
  }

  @override
  void update(void Function(ComputerAttachIdentifyBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ComputerAttachIdentify build() => _build();

  _$ComputerAttachIdentify _build() {
    final _$result =
        _$v ??
        _$ComputerAttachIdentify._(
          protocolVersion: BuiltValueNullFieldError.checkNotNull(
            protocolVersion,
            r'ComputerAttachIdentify',
            'protocolVersion',
          ),
          token: BuiltValueNullFieldError.checkNotNull(
            token,
            r'ComputerAttachIdentify',
            'token',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

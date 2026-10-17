// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_identify.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostIdentify extends HostIdentify {
  @override
  final int protocolVersion;
  @override
  final String hostId;
  @override
  final String proof;
  @override
  final String hostVersion;
  @override
  final String? sessionId;
  @override
  final BuiltList<String>? liveSessions;

  factory _$HostIdentify([void Function(HostIdentifyBuilder)? updates]) =>
      (HostIdentifyBuilder()..update(updates))._build();

  _$HostIdentify._({
    required this.protocolVersion,
    required this.hostId,
    required this.proof,
    required this.hostVersion,
    this.sessionId,
    this.liveSessions,
  }) : super._();
  @override
  HostIdentify rebuild(void Function(HostIdentifyBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostIdentifyBuilder toBuilder() => HostIdentifyBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostIdentify &&
        protocolVersion == other.protocolVersion &&
        hostId == other.hostId &&
        proof == other.proof &&
        hostVersion == other.hostVersion &&
        sessionId == other.sessionId &&
        liveSessions == other.liveSessions;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, protocolVersion.hashCode);
    _$hash = $jc(_$hash, hostId.hashCode);
    _$hash = $jc(_$hash, proof.hashCode);
    _$hash = $jc(_$hash, hostVersion.hashCode);
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jc(_$hash, liveSessions.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostIdentify')
          ..add('protocolVersion', protocolVersion)
          ..add('hostId', hostId)
          ..add('proof', proof)
          ..add('hostVersion', hostVersion)
          ..add('sessionId', sessionId)
          ..add('liveSessions', liveSessions))
        .toString();
  }
}

class HostIdentifyBuilder
    implements Builder<HostIdentify, HostIdentifyBuilder> {
  _$HostIdentify? _$v;

  int? _protocolVersion;
  int? get protocolVersion => _$this._protocolVersion;
  set protocolVersion(int? protocolVersion) =>
      _$this._protocolVersion = protocolVersion;

  String? _hostId;
  String? get hostId => _$this._hostId;
  set hostId(String? hostId) => _$this._hostId = hostId;

  String? _proof;
  String? get proof => _$this._proof;
  set proof(String? proof) => _$this._proof = proof;

  String? _hostVersion;
  String? get hostVersion => _$this._hostVersion;
  set hostVersion(String? hostVersion) => _$this._hostVersion = hostVersion;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  ListBuilder<String>? _liveSessions;
  ListBuilder<String> get liveSessions =>
      _$this._liveSessions ??= ListBuilder<String>();
  set liveSessions(ListBuilder<String>? liveSessions) =>
      _$this._liveSessions = liveSessions;

  HostIdentifyBuilder() {
    HostIdentify._defaults(this);
  }

  HostIdentifyBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _protocolVersion = $v.protocolVersion;
      _hostId = $v.hostId;
      _proof = $v.proof;
      _hostVersion = $v.hostVersion;
      _sessionId = $v.sessionId;
      _liveSessions = $v.liveSessions?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostIdentify other) {
    _$v = other as _$HostIdentify;
  }

  @override
  void update(void Function(HostIdentifyBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostIdentify build() => _build();

  _$HostIdentify _build() {
    _$HostIdentify _$result;
    try {
      _$result =
          _$v ??
          _$HostIdentify._(
            protocolVersion: BuiltValueNullFieldError.checkNotNull(
              protocolVersion,
              r'HostIdentify',
              'protocolVersion',
            ),
            hostId: BuiltValueNullFieldError.checkNotNull(
              hostId,
              r'HostIdentify',
              'hostId',
            ),
            proof: BuiltValueNullFieldError.checkNotNull(
              proof,
              r'HostIdentify',
              'proof',
            ),
            hostVersion: BuiltValueNullFieldError.checkNotNull(
              hostVersion,
              r'HostIdentify',
              'hostVersion',
            ),
            sessionId: sessionId,
            liveSessions: _liveSessions?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'liveSessions';
        _liveSessions?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostIdentify',
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

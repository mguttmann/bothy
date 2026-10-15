// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_pair_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostPairResult extends HostPairResult {
  @override
  final String hostId;
  @override
  final String keyFingerprint;
  @override
  final String instanceId;

  factory _$HostPairResult([void Function(HostPairResultBuilder)? updates]) =>
      (HostPairResultBuilder()..update(updates))._build();

  _$HostPairResult._({
    required this.hostId,
    required this.keyFingerprint,
    required this.instanceId,
  }) : super._();
  @override
  HostPairResult rebuild(void Function(HostPairResultBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostPairResultBuilder toBuilder() => HostPairResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostPairResult &&
        hostId == other.hostId &&
        keyFingerprint == other.keyFingerprint &&
        instanceId == other.instanceId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, hostId.hashCode);
    _$hash = $jc(_$hash, keyFingerprint.hashCode);
    _$hash = $jc(_$hash, instanceId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostPairResult')
          ..add('hostId', hostId)
          ..add('keyFingerprint', keyFingerprint)
          ..add('instanceId', instanceId))
        .toString();
  }
}

class HostPairResultBuilder
    implements Builder<HostPairResult, HostPairResultBuilder> {
  _$HostPairResult? _$v;

  String? _hostId;
  String? get hostId => _$this._hostId;
  set hostId(String? hostId) => _$this._hostId = hostId;

  String? _keyFingerprint;
  String? get keyFingerprint => _$this._keyFingerprint;
  set keyFingerprint(String? keyFingerprint) =>
      _$this._keyFingerprint = keyFingerprint;

  String? _instanceId;
  String? get instanceId => _$this._instanceId;
  set instanceId(String? instanceId) => _$this._instanceId = instanceId;

  HostPairResultBuilder() {
    HostPairResult._defaults(this);
  }

  HostPairResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _hostId = $v.hostId;
      _keyFingerprint = $v.keyFingerprint;
      _instanceId = $v.instanceId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostPairResult other) {
    _$v = other as _$HostPairResult;
  }

  @override
  void update(void Function(HostPairResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostPairResult build() => _build();

  _$HostPairResult _build() {
    final _$result =
        _$v ??
        _$HostPairResult._(
          hostId: BuiltValueNullFieldError.checkNotNull(
            hostId,
            r'HostPairResult',
            'hostId',
          ),
          keyFingerprint: BuiltValueNullFieldError.checkNotNull(
            keyFingerprint,
            r'HostPairResult',
            'keyFingerprint',
          ),
          instanceId: BuiltValueNullFieldError.checkNotNull(
            instanceId,
            r'HostPairResult',
            'instanceId',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

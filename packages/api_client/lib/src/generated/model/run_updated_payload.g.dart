// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_updated_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunUpdatedPayload extends RunUpdatedPayload {
  @override
  final String runId;
  @override
  final int? baseVersion;
  @override
  final int version;
  @override
  final BuiltList<JsonPatchOperation>? ops;
  @override
  final Run? snapshot;

  factory _$RunUpdatedPayload([
    void Function(RunUpdatedPayloadBuilder)? updates,
  ]) => (RunUpdatedPayloadBuilder()..update(updates))._build();

  _$RunUpdatedPayload._({
    required this.runId,
    this.baseVersion,
    required this.version,
    this.ops,
    this.snapshot,
  }) : super._();
  @override
  RunUpdatedPayload rebuild(void Function(RunUpdatedPayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunUpdatedPayloadBuilder toBuilder() =>
      RunUpdatedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunUpdatedPayload &&
        runId == other.runId &&
        baseVersion == other.baseVersion &&
        version == other.version &&
        ops == other.ops &&
        snapshot == other.snapshot;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, baseVersion.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, ops.hashCode);
    _$hash = $jc(_$hash, snapshot.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunUpdatedPayload')
          ..add('runId', runId)
          ..add('baseVersion', baseVersion)
          ..add('version', version)
          ..add('ops', ops)
          ..add('snapshot', snapshot))
        .toString();
  }
}

class RunUpdatedPayloadBuilder
    implements Builder<RunUpdatedPayload, RunUpdatedPayloadBuilder> {
  _$RunUpdatedPayload? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  int? _baseVersion;
  int? get baseVersion => _$this._baseVersion;
  set baseVersion(int? baseVersion) => _$this._baseVersion = baseVersion;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  ListBuilder<JsonPatchOperation>? _ops;
  ListBuilder<JsonPatchOperation> get ops =>
      _$this._ops ??= ListBuilder<JsonPatchOperation>();
  set ops(ListBuilder<JsonPatchOperation>? ops) => _$this._ops = ops;

  RunBuilder? _snapshot;
  RunBuilder get snapshot => _$this._snapshot ??= RunBuilder();
  set snapshot(RunBuilder? snapshot) => _$this._snapshot = snapshot;

  RunUpdatedPayloadBuilder() {
    RunUpdatedPayload._defaults(this);
  }

  RunUpdatedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _baseVersion = $v.baseVersion;
      _version = $v.version;
      _ops = $v.ops?.toBuilder();
      _snapshot = $v.snapshot?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunUpdatedPayload other) {
    _$v = other as _$RunUpdatedPayload;
  }

  @override
  void update(void Function(RunUpdatedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunUpdatedPayload build() => _build();

  _$RunUpdatedPayload _build() {
    _$RunUpdatedPayload _$result;
    try {
      _$result =
          _$v ??
          _$RunUpdatedPayload._(
            runId: BuiltValueNullFieldError.checkNotNull(
              runId,
              r'RunUpdatedPayload',
              'runId',
            ),
            baseVersion: baseVersion,
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'RunUpdatedPayload',
              'version',
            ),
            ops: _ops?.build(),
            snapshot: _snapshot?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'ops';
        _ops?.build();
        _$failedField = 'snapshot';
        _snapshot?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunUpdatedPayload',
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

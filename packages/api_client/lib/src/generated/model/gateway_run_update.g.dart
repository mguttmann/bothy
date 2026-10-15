// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_run_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayRunUpdate extends GatewayRunUpdate {
  @override
  final String runId;
  @override
  final int baseVersion;
  @override
  final BuiltList<JsonPatchOperation> ops;
  @override
  final String? ref;

  factory _$GatewayRunUpdate([
    void Function(GatewayRunUpdateBuilder)? updates,
  ]) => (GatewayRunUpdateBuilder()..update(updates))._build();

  _$GatewayRunUpdate._({
    required this.runId,
    required this.baseVersion,
    required this.ops,
    this.ref,
  }) : super._();
  @override
  GatewayRunUpdate rebuild(void Function(GatewayRunUpdateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayRunUpdateBuilder toBuilder() =>
      GatewayRunUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayRunUpdate &&
        runId == other.runId &&
        baseVersion == other.baseVersion &&
        ops == other.ops &&
        ref == other.ref;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, baseVersion.hashCode);
    _$hash = $jc(_$hash, ops.hashCode);
    _$hash = $jc(_$hash, ref.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayRunUpdate')
          ..add('runId', runId)
          ..add('baseVersion', baseVersion)
          ..add('ops', ops)
          ..add('ref', ref))
        .toString();
  }
}

class GatewayRunUpdateBuilder
    implements Builder<GatewayRunUpdate, GatewayRunUpdateBuilder> {
  _$GatewayRunUpdate? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  int? _baseVersion;
  int? get baseVersion => _$this._baseVersion;
  set baseVersion(int? baseVersion) => _$this._baseVersion = baseVersion;

  ListBuilder<JsonPatchOperation>? _ops;
  ListBuilder<JsonPatchOperation> get ops =>
      _$this._ops ??= ListBuilder<JsonPatchOperation>();
  set ops(ListBuilder<JsonPatchOperation>? ops) => _$this._ops = ops;

  String? _ref;
  String? get ref => _$this._ref;
  set ref(String? ref) => _$this._ref = ref;

  GatewayRunUpdateBuilder() {
    GatewayRunUpdate._defaults(this);
  }

  GatewayRunUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _baseVersion = $v.baseVersion;
      _ops = $v.ops.toBuilder();
      _ref = $v.ref;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayRunUpdate other) {
    _$v = other as _$GatewayRunUpdate;
  }

  @override
  void update(void Function(GatewayRunUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayRunUpdate build() => _build();

  _$GatewayRunUpdate _build() {
    _$GatewayRunUpdate _$result;
    try {
      _$result =
          _$v ??
          _$GatewayRunUpdate._(
            runId: BuiltValueNullFieldError.checkNotNull(
              runId,
              r'GatewayRunUpdate',
              'runId',
            ),
            baseVersion: BuiltValueNullFieldError.checkNotNull(
              baseVersion,
              r'GatewayRunUpdate',
              'baseVersion',
            ),
            ops: ops.build(),
            ref: ref,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'ops';
        ops.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GatewayRunUpdate',
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

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_patch.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunPatch extends RunPatch {
  @override
  final int baseVersion;
  @override
  final BuiltList<JsonPatchOperation> ops;

  factory _$RunPatch([void Function(RunPatchBuilder)? updates]) =>
      (RunPatchBuilder()..update(updates))._build();

  _$RunPatch._({required this.baseVersion, required this.ops}) : super._();
  @override
  RunPatch rebuild(void Function(RunPatchBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunPatchBuilder toBuilder() => RunPatchBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunPatch &&
        baseVersion == other.baseVersion &&
        ops == other.ops;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, baseVersion.hashCode);
    _$hash = $jc(_$hash, ops.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunPatch')
          ..add('baseVersion', baseVersion)
          ..add('ops', ops))
        .toString();
  }
}

class RunPatchBuilder implements Builder<RunPatch, RunPatchBuilder> {
  _$RunPatch? _$v;

  int? _baseVersion;
  int? get baseVersion => _$this._baseVersion;
  set baseVersion(int? baseVersion) => _$this._baseVersion = baseVersion;

  ListBuilder<JsonPatchOperation>? _ops;
  ListBuilder<JsonPatchOperation> get ops =>
      _$this._ops ??= ListBuilder<JsonPatchOperation>();
  set ops(ListBuilder<JsonPatchOperation>? ops) => _$this._ops = ops;

  RunPatchBuilder() {
    RunPatch._defaults(this);
  }

  RunPatchBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _baseVersion = $v.baseVersion;
      _ops = $v.ops.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunPatch other) {
    _$v = other as _$RunPatch;
  }

  @override
  void update(void Function(RunPatchBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunPatch build() => _build();

  _$RunPatch _build() {
    _$RunPatch _$result;
    try {
      _$result =
          _$v ??
          _$RunPatch._(
            baseVersion: BuiltValueNullFieldError.checkNotNull(
              baseVersion,
              r'RunPatch',
              'baseVersion',
            ),
            ops: ops.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'ops';
        ops.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunPatch',
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

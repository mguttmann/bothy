// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_patch_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunPatchResult extends RunPatchResult {
  @override
  final int version;

  factory _$RunPatchResult([void Function(RunPatchResultBuilder)? updates]) =>
      (RunPatchResultBuilder()..update(updates))._build();

  _$RunPatchResult._({required this.version}) : super._();
  @override
  RunPatchResult rebuild(void Function(RunPatchResultBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunPatchResultBuilder toBuilder() => RunPatchResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunPatchResult && version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'RunPatchResult',
    )..add('version', version)).toString();
  }
}

class RunPatchResultBuilder
    implements Builder<RunPatchResult, RunPatchResultBuilder> {
  _$RunPatchResult? _$v;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  RunPatchResultBuilder() {
    RunPatchResult._defaults(this);
  }

  RunPatchResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunPatchResult other) {
    _$v = other as _$RunPatchResult;
  }

  @override
  void update(void Function(RunPatchResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunPatchResult build() => _build();

  _$RunPatchResult _build() {
    final _$result =
        _$v ??
        _$RunPatchResult._(
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'RunPatchResult',
            'version',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

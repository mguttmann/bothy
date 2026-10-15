// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'evaluated_flag.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$EvaluatedFlag extends EvaluatedFlag {
  @override
  final String key;
  @override
  final bool enabled;

  factory _$EvaluatedFlag([void Function(EvaluatedFlagBuilder)? updates]) =>
      (EvaluatedFlagBuilder()..update(updates))._build();

  _$EvaluatedFlag._({required this.key, required this.enabled}) : super._();
  @override
  EvaluatedFlag rebuild(void Function(EvaluatedFlagBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  EvaluatedFlagBuilder toBuilder() => EvaluatedFlagBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is EvaluatedFlag &&
        key == other.key &&
        enabled == other.enabled;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, key.hashCode);
    _$hash = $jc(_$hash, enabled.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'EvaluatedFlag')
          ..add('key', key)
          ..add('enabled', enabled))
        .toString();
  }
}

class EvaluatedFlagBuilder
    implements Builder<EvaluatedFlag, EvaluatedFlagBuilder> {
  _$EvaluatedFlag? _$v;

  String? _key;
  String? get key => _$this._key;
  set key(String? key) => _$this._key = key;

  bool? _enabled;
  bool? get enabled => _$this._enabled;
  set enabled(bool? enabled) => _$this._enabled = enabled;

  EvaluatedFlagBuilder() {
    EvaluatedFlag._defaults(this);
  }

  EvaluatedFlagBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _key = $v.key;
      _enabled = $v.enabled;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(EvaluatedFlag other) {
    _$v = other as _$EvaluatedFlag;
  }

  @override
  void update(void Function(EvaluatedFlagBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  EvaluatedFlag build() => _build();

  _$EvaluatedFlag _build() {
    final _$result =
        _$v ??
        _$EvaluatedFlag._(
          key: BuiltValueNullFieldError.checkNotNull(
            key,
            r'EvaluatedFlag',
            'key',
          ),
          enabled: BuiltValueNullFieldError.checkNotNull(
            enabled,
            r'EvaluatedFlag',
            'enabled',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

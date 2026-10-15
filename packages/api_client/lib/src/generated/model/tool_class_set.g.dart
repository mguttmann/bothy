// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'tool_class_set.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ToolClassSet extends ToolClassSet {
  @override
  final ToolClass toolClass;
  @override
  final String reason;

  factory _$ToolClassSet([void Function(ToolClassSetBuilder)? updates]) =>
      (ToolClassSetBuilder()..update(updates))._build();

  _$ToolClassSet._({required this.toolClass, required this.reason}) : super._();
  @override
  ToolClassSet rebuild(void Function(ToolClassSetBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ToolClassSetBuilder toBuilder() => ToolClassSetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ToolClassSet &&
        toolClass == other.toolClass &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, toolClass.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ToolClassSet')
          ..add('toolClass', toolClass)
          ..add('reason', reason))
        .toString();
  }
}

class ToolClassSetBuilder
    implements Builder<ToolClassSet, ToolClassSetBuilder> {
  _$ToolClassSet? _$v;

  ToolClass? _toolClass;
  ToolClass? get toolClass => _$this._toolClass;
  set toolClass(ToolClass? toolClass) => _$this._toolClass = toolClass;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  ToolClassSetBuilder() {
    ToolClassSet._defaults(this);
  }

  ToolClassSetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _toolClass = $v.toolClass;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ToolClassSet other) {
    _$v = other as _$ToolClassSet;
  }

  @override
  void update(void Function(ToolClassSetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ToolClassSet build() => _build();

  _$ToolClassSet _build() {
    final _$result =
        _$v ??
        _$ToolClassSet._(
          toolClass: BuiltValueNullFieldError.checkNotNull(
            toolClass,
            r'ToolClassSet',
            'toolClass',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'ToolClassSet',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

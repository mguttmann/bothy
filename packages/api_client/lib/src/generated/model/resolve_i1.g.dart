// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'resolve_i1.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ResolveI1 extends ResolveI1 {
  @override
  final BuiltSet<String> switchToOrganization;
  @override
  final String reason;

  factory _$ResolveI1([void Function(ResolveI1Builder)? updates]) =>
      (ResolveI1Builder()..update(updates))._build();

  _$ResolveI1._({required this.switchToOrganization, required this.reason})
    : super._();
  @override
  ResolveI1 rebuild(void Function(ResolveI1Builder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ResolveI1Builder toBuilder() => ResolveI1Builder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ResolveI1 &&
        switchToOrganization == other.switchToOrganization &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, switchToOrganization.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ResolveI1')
          ..add('switchToOrganization', switchToOrganization)
          ..add('reason', reason))
        .toString();
  }
}

class ResolveI1Builder implements Builder<ResolveI1, ResolveI1Builder> {
  _$ResolveI1? _$v;

  SetBuilder<String>? _switchToOrganization;
  SetBuilder<String> get switchToOrganization =>
      _$this._switchToOrganization ??= SetBuilder<String>();
  set switchToOrganization(SetBuilder<String>? switchToOrganization) =>
      _$this._switchToOrganization = switchToOrganization;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  ResolveI1Builder() {
    ResolveI1._defaults(this);
  }

  ResolveI1Builder get _$this {
    final $v = _$v;
    if ($v != null) {
      _switchToOrganization = $v.switchToOrganization.toBuilder();
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ResolveI1 other) {
    _$v = other as _$ResolveI1;
  }

  @override
  void update(void Function(ResolveI1Builder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ResolveI1 build() => _build();

  _$ResolveI1 _build() {
    _$ResolveI1 _$result;
    try {
      _$result =
          _$v ??
          _$ResolveI1._(
            switchToOrganization: switchToOrganization.build(),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'ResolveI1',
              'reason',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'switchToOrganization';
        switchToOrganization.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ResolveI1',
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

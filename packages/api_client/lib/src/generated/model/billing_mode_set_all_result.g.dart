// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'billing_mode_set_all_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$BillingModeSetAllResult extends BillingModeSetAllResult {
  @override
  final BuiltList<String> changedAgentIds;

  factory _$BillingModeSetAllResult([
    void Function(BillingModeSetAllResultBuilder)? updates,
  ]) => (BillingModeSetAllResultBuilder()..update(updates))._build();

  _$BillingModeSetAllResult._({required this.changedAgentIds}) : super._();
  @override
  BillingModeSetAllResult rebuild(
    void Function(BillingModeSetAllResultBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  BillingModeSetAllResultBuilder toBuilder() =>
      BillingModeSetAllResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is BillingModeSetAllResult &&
        changedAgentIds == other.changedAgentIds;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, changedAgentIds.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'BillingModeSetAllResult',
    )..add('changedAgentIds', changedAgentIds)).toString();
  }
}

class BillingModeSetAllResultBuilder
    implements
        Builder<BillingModeSetAllResult, BillingModeSetAllResultBuilder> {
  _$BillingModeSetAllResult? _$v;

  ListBuilder<String>? _changedAgentIds;
  ListBuilder<String> get changedAgentIds =>
      _$this._changedAgentIds ??= ListBuilder<String>();
  set changedAgentIds(ListBuilder<String>? changedAgentIds) =>
      _$this._changedAgentIds = changedAgentIds;

  BillingModeSetAllResultBuilder() {
    BillingModeSetAllResult._defaults(this);
  }

  BillingModeSetAllResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _changedAgentIds = $v.changedAgentIds.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(BillingModeSetAllResult other) {
    _$v = other as _$BillingModeSetAllResult;
  }

  @override
  void update(void Function(BillingModeSetAllResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  BillingModeSetAllResult build() => _build();

  _$BillingModeSetAllResult _build() {
    _$BillingModeSetAllResult _$result;
    try {
      _$result =
          _$v ??
          _$BillingModeSetAllResult._(changedAgentIds: changedAgentIds.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'changedAgentIds';
        changedAgentIds.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'BillingModeSetAllResult',
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

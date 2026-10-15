// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'billing_mode_set.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$BillingModeSet extends BillingModeSet {
  @override
  final BillingMode billingMode;
  @override
  final String? credentialHolderId;
  @override
  final String reason;

  factory _$BillingModeSet([void Function(BillingModeSetBuilder)? updates]) =>
      (BillingModeSetBuilder()..update(updates))._build();

  _$BillingModeSet._({
    required this.billingMode,
    this.credentialHolderId,
    required this.reason,
  }) : super._();
  @override
  BillingModeSet rebuild(void Function(BillingModeSetBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  BillingModeSetBuilder toBuilder() => BillingModeSetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is BillingModeSet &&
        billingMode == other.billingMode &&
        credentialHolderId == other.credentialHolderId &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, credentialHolderId.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'BillingModeSet')
          ..add('billingMode', billingMode)
          ..add('credentialHolderId', credentialHolderId)
          ..add('reason', reason))
        .toString();
  }
}

class BillingModeSetBuilder
    implements Builder<BillingModeSet, BillingModeSetBuilder> {
  _$BillingModeSet? _$v;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  String? _credentialHolderId;
  String? get credentialHolderId => _$this._credentialHolderId;
  set credentialHolderId(String? credentialHolderId) =>
      _$this._credentialHolderId = credentialHolderId;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  BillingModeSetBuilder() {
    BillingModeSet._defaults(this);
  }

  BillingModeSetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _billingMode = $v.billingMode;
      _credentialHolderId = $v.credentialHolderId;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(BillingModeSet other) {
    _$v = other as _$BillingModeSet;
  }

  @override
  void update(void Function(BillingModeSetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  BillingModeSet build() => _build();

  _$BillingModeSet _build() {
    final _$result =
        _$v ??
        _$BillingModeSet._(
          billingMode: BuiltValueNullFieldError.checkNotNull(
            billingMode,
            r'BillingModeSet',
            'billingMode',
          ),
          credentialHolderId: credentialHolderId,
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'BillingModeSet',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

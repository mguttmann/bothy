// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'billing_mode_set_all.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$BillingModeSetAll extends BillingModeSetAll {
  @override
  final BillingMode billingMode;
  @override
  final String? credentialHolderId;
  @override
  final BuiltList<String>? agentIds;
  @override
  final String reason;

  factory _$BillingModeSetAll([
    void Function(BillingModeSetAllBuilder)? updates,
  ]) => (BillingModeSetAllBuilder()..update(updates))._build();

  _$BillingModeSetAll._({
    required this.billingMode,
    this.credentialHolderId,
    this.agentIds,
    required this.reason,
  }) : super._();
  @override
  BillingModeSetAll rebuild(void Function(BillingModeSetAllBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  BillingModeSetAllBuilder toBuilder() =>
      BillingModeSetAllBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is BillingModeSetAll &&
        billingMode == other.billingMode &&
        credentialHolderId == other.credentialHolderId &&
        agentIds == other.agentIds &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, credentialHolderId.hashCode);
    _$hash = $jc(_$hash, agentIds.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'BillingModeSetAll')
          ..add('billingMode', billingMode)
          ..add('credentialHolderId', credentialHolderId)
          ..add('agentIds', agentIds)
          ..add('reason', reason))
        .toString();
  }
}

class BillingModeSetAllBuilder
    implements Builder<BillingModeSetAll, BillingModeSetAllBuilder> {
  _$BillingModeSetAll? _$v;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  String? _credentialHolderId;
  String? get credentialHolderId => _$this._credentialHolderId;
  set credentialHolderId(String? credentialHolderId) =>
      _$this._credentialHolderId = credentialHolderId;

  ListBuilder<String>? _agentIds;
  ListBuilder<String> get agentIds =>
      _$this._agentIds ??= ListBuilder<String>();
  set agentIds(ListBuilder<String>? agentIds) => _$this._agentIds = agentIds;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  BillingModeSetAllBuilder() {
    BillingModeSetAll._defaults(this);
  }

  BillingModeSetAllBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _billingMode = $v.billingMode;
      _credentialHolderId = $v.credentialHolderId;
      _agentIds = $v.agentIds?.toBuilder();
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(BillingModeSetAll other) {
    _$v = other as _$BillingModeSetAll;
  }

  @override
  void update(void Function(BillingModeSetAllBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  BillingModeSetAll build() => _build();

  _$BillingModeSetAll _build() {
    _$BillingModeSetAll _$result;
    try {
      _$result =
          _$v ??
          _$BillingModeSetAll._(
            billingMode: BuiltValueNullFieldError.checkNotNull(
              billingMode,
              r'BillingModeSetAll',
              'billingMode',
            ),
            credentialHolderId: credentialHolderId,
            agentIds: _agentIds?.build(),
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'BillingModeSetAll',
              'reason',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'agentIds';
        _agentIds?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'BillingModeSetAll',
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

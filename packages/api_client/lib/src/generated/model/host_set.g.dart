// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_set.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostSet extends HostSet {
  @override
  final String hostedBy;
  @override
  final String reason;

  factory _$HostSet([void Function(HostSetBuilder)? updates]) =>
      (HostSetBuilder()..update(updates))._build();

  _$HostSet._({required this.hostedBy, required this.reason}) : super._();
  @override
  HostSet rebuild(void Function(HostSetBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostSetBuilder toBuilder() => HostSetBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostSet &&
        hostedBy == other.hostedBy &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, hostedBy.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostSet')
          ..add('hostedBy', hostedBy)
          ..add('reason', reason))
        .toString();
  }
}

class HostSetBuilder implements Builder<HostSet, HostSetBuilder> {
  _$HostSet? _$v;

  String? _hostedBy;
  String? get hostedBy => _$this._hostedBy;
  set hostedBy(String? hostedBy) => _$this._hostedBy = hostedBy;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  HostSetBuilder() {
    HostSet._defaults(this);
  }

  HostSetBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _hostedBy = $v.hostedBy;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostSet other) {
    _$v = other as _$HostSet;
  }

  @override
  void update(void Function(HostSetBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostSet build() => _build();

  _$HostSet _build() {
    final _$result =
        _$v ??
        _$HostSet._(
          hostedBy: BuiltValueNullFieldError.checkNotNull(
            hostedBy,
            r'HostSet',
            'hostedBy',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'HostSet',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

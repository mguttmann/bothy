// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'reason_body.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ReasonBody extends ReasonBody {
  @override
  final String reason;

  factory _$ReasonBody([void Function(ReasonBodyBuilder)? updates]) =>
      (ReasonBodyBuilder()..update(updates))._build();

  _$ReasonBody._({required this.reason}) : super._();
  @override
  ReasonBody rebuild(void Function(ReasonBodyBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ReasonBodyBuilder toBuilder() => ReasonBodyBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ReasonBody && reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ReasonBody',
    )..add('reason', reason)).toString();
  }
}

class ReasonBodyBuilder implements Builder<ReasonBody, ReasonBodyBuilder> {
  _$ReasonBody? _$v;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  ReasonBodyBuilder() {
    ReasonBody._defaults(this);
  }

  ReasonBodyBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ReasonBody other) {
    _$v = other as _$ReasonBody;
  }

  @override
  void update(void Function(ReasonBodyBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ReasonBody build() => _build();

  _$ReasonBody _build() {
    final _$result =
        _$v ??
        _$ReasonBody._(
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'ReasonBody',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

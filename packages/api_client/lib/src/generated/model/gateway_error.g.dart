// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_error.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayError extends GatewayError {
  @override
  final String op;
  @override
  final String code;
  @override
  final String? detail;
  @override
  final String? ref;

  factory _$GatewayError([void Function(GatewayErrorBuilder)? updates]) =>
      (GatewayErrorBuilder()..update(updates))._build();

  _$GatewayError._({
    required this.op,
    required this.code,
    this.detail,
    this.ref,
  }) : super._();
  @override
  GatewayError rebuild(void Function(GatewayErrorBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayErrorBuilder toBuilder() => GatewayErrorBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayError &&
        op == other.op &&
        code == other.code &&
        detail == other.detail &&
        ref == other.ref;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, op.hashCode);
    _$hash = $jc(_$hash, code.hashCode);
    _$hash = $jc(_$hash, detail.hashCode);
    _$hash = $jc(_$hash, ref.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayError')
          ..add('op', op)
          ..add('code', code)
          ..add('detail', detail)
          ..add('ref', ref))
        .toString();
  }
}

class GatewayErrorBuilder
    implements Builder<GatewayError, GatewayErrorBuilder> {
  _$GatewayError? _$v;

  String? _op;
  String? get op => _$this._op;
  set op(String? op) => _$this._op = op;

  String? _code;
  String? get code => _$this._code;
  set code(String? code) => _$this._code = code;

  String? _detail;
  String? get detail => _$this._detail;
  set detail(String? detail) => _$this._detail = detail;

  String? _ref;
  String? get ref => _$this._ref;
  set ref(String? ref) => _$this._ref = ref;

  GatewayErrorBuilder() {
    GatewayError._defaults(this);
  }

  GatewayErrorBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _op = $v.op;
      _code = $v.code;
      _detail = $v.detail;
      _ref = $v.ref;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayError other) {
    _$v = other as _$GatewayError;
  }

  @override
  void update(void Function(GatewayErrorBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayError build() => _build();

  _$GatewayError _build() {
    final _$result =
        _$v ??
        _$GatewayError._(
          op: BuiltValueNullFieldError.checkNotNull(op, r'GatewayError', 'op'),
          code: BuiltValueNullFieldError.checkNotNull(
            code,
            r'GatewayError',
            'code',
          ),
          detail: detail,
          ref: ref,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

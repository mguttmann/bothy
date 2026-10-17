// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'field_error.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$FieldError extends FieldError {
  @override
  final String pointer;
  @override
  final String code;
  @override
  final String? detail;

  factory _$FieldError([void Function(FieldErrorBuilder)? updates]) =>
      (FieldErrorBuilder()..update(updates))._build();

  _$FieldError._({required this.pointer, required this.code, this.detail})
    : super._();
  @override
  FieldError rebuild(void Function(FieldErrorBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  FieldErrorBuilder toBuilder() => FieldErrorBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is FieldError &&
        pointer == other.pointer &&
        code == other.code &&
        detail == other.detail;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, pointer.hashCode);
    _$hash = $jc(_$hash, code.hashCode);
    _$hash = $jc(_$hash, detail.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'FieldError')
          ..add('pointer', pointer)
          ..add('code', code)
          ..add('detail', detail))
        .toString();
  }
}

class FieldErrorBuilder implements Builder<FieldError, FieldErrorBuilder> {
  _$FieldError? _$v;

  String? _pointer;
  String? get pointer => _$this._pointer;
  set pointer(String? pointer) => _$this._pointer = pointer;

  String? _code;
  String? get code => _$this._code;
  set code(String? code) => _$this._code = code;

  String? _detail;
  String? get detail => _$this._detail;
  set detail(String? detail) => _$this._detail = detail;

  FieldErrorBuilder() {
    FieldError._defaults(this);
  }

  FieldErrorBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _pointer = $v.pointer;
      _code = $v.code;
      _detail = $v.detail;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(FieldError other) {
    _$v = other as _$FieldError;
  }

  @override
  void update(void Function(FieldErrorBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  FieldError build() => _build();

  _$FieldError _build() {
    final _$result =
        _$v ??
        _$FieldError._(
          pointer: BuiltValueNullFieldError.checkNotNull(
            pointer,
            r'FieldError',
            'pointer',
          ),
          code: BuiltValueNullFieldError.checkNotNull(
            code,
            r'FieldError',
            'code',
          ),
          detail: detail,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

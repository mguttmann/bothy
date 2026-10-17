// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'price_close.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PriceClose extends PriceClose {
  @override
  final DateTime validTo;

  factory _$PriceClose([void Function(PriceCloseBuilder)? updates]) =>
      (PriceCloseBuilder()..update(updates))._build();

  _$PriceClose._({required this.validTo}) : super._();
  @override
  PriceClose rebuild(void Function(PriceCloseBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PriceCloseBuilder toBuilder() => PriceCloseBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PriceClose && validTo == other.validTo;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, validTo.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'PriceClose',
    )..add('validTo', validTo)).toString();
  }
}

class PriceCloseBuilder implements Builder<PriceClose, PriceCloseBuilder> {
  _$PriceClose? _$v;

  DateTime? _validTo;
  DateTime? get validTo => _$this._validTo;
  set validTo(DateTime? validTo) => _$this._validTo = validTo;

  PriceCloseBuilder() {
    PriceClose._defaults(this);
  }

  PriceCloseBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _validTo = $v.validTo;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PriceClose other) {
    _$v = other as _$PriceClose;
  }

  @override
  void update(void Function(PriceCloseBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PriceClose build() => _build();

  _$PriceClose _build() {
    final _$result =
        _$v ??
        _$PriceClose._(
          validTo: BuiltValueNullFieldError.checkNotNull(
            validTo,
            r'PriceClose',
            'validTo',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

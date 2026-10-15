// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'branding_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$BrandingUpdate extends BrandingUpdate {
  @override
  final String productName;
  @override
  final String? accentColor;

  factory _$BrandingUpdate([void Function(BrandingUpdateBuilder)? updates]) =>
      (BrandingUpdateBuilder()..update(updates))._build();

  _$BrandingUpdate._({required this.productName, this.accentColor}) : super._();
  @override
  BrandingUpdate rebuild(void Function(BrandingUpdateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  BrandingUpdateBuilder toBuilder() => BrandingUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is BrandingUpdate &&
        productName == other.productName &&
        accentColor == other.accentColor;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, productName.hashCode);
    _$hash = $jc(_$hash, accentColor.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'BrandingUpdate')
          ..add('productName', productName)
          ..add('accentColor', accentColor))
        .toString();
  }
}

class BrandingUpdateBuilder
    implements Builder<BrandingUpdate, BrandingUpdateBuilder> {
  _$BrandingUpdate? _$v;

  String? _productName;
  String? get productName => _$this._productName;
  set productName(String? productName) => _$this._productName = productName;

  String? _accentColor;
  String? get accentColor => _$this._accentColor;
  set accentColor(String? accentColor) => _$this._accentColor = accentColor;

  BrandingUpdateBuilder() {
    BrandingUpdate._defaults(this);
  }

  BrandingUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _productName = $v.productName;
      _accentColor = $v.accentColor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(BrandingUpdate other) {
    _$v = other as _$BrandingUpdate;
  }

  @override
  void update(void Function(BrandingUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  BrandingUpdate build() => _build();

  _$BrandingUpdate _build() {
    final _$result =
        _$v ??
        _$BrandingUpdate._(
          productName: BuiltValueNullFieldError.checkNotNull(
            productName,
            r'BrandingUpdate',
            'productName',
          ),
          accentColor: accentColor,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

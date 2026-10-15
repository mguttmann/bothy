// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'public_branding.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PublicBranding extends PublicBranding {
  @override
  final String productName;
  @override
  final String? accentColor;
  @override
  final String? logoUrl;

  factory _$PublicBranding([void Function(PublicBrandingBuilder)? updates]) =>
      (PublicBrandingBuilder()..update(updates))._build();

  _$PublicBranding._({
    required this.productName,
    this.accentColor,
    this.logoUrl,
  }) : super._();
  @override
  PublicBranding rebuild(void Function(PublicBrandingBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PublicBrandingBuilder toBuilder() => PublicBrandingBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PublicBranding &&
        productName == other.productName &&
        accentColor == other.accentColor &&
        logoUrl == other.logoUrl;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, productName.hashCode);
    _$hash = $jc(_$hash, accentColor.hashCode);
    _$hash = $jc(_$hash, logoUrl.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'PublicBranding')
          ..add('productName', productName)
          ..add('accentColor', accentColor)
          ..add('logoUrl', logoUrl))
        .toString();
  }
}

class PublicBrandingBuilder
    implements Builder<PublicBranding, PublicBrandingBuilder> {
  _$PublicBranding? _$v;

  String? _productName;
  String? get productName => _$this._productName;
  set productName(String? productName) => _$this._productName = productName;

  String? _accentColor;
  String? get accentColor => _$this._accentColor;
  set accentColor(String? accentColor) => _$this._accentColor = accentColor;

  String? _logoUrl;
  String? get logoUrl => _$this._logoUrl;
  set logoUrl(String? logoUrl) => _$this._logoUrl = logoUrl;

  PublicBrandingBuilder() {
    PublicBranding._defaults(this);
  }

  PublicBrandingBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _productName = $v.productName;
      _accentColor = $v.accentColor;
      _logoUrl = $v.logoUrl;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PublicBranding other) {
    _$v = other as _$PublicBranding;
  }

  @override
  void update(void Function(PublicBrandingBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PublicBranding build() => _build();

  _$PublicBranding _build() {
    final _$result =
        _$v ??
        _$PublicBranding._(
          productName: BuiltValueNullFieldError.checkNotNull(
            productName,
            r'PublicBranding',
            'productName',
          ),
          accentColor: accentColor,
          logoUrl: logoUrl,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

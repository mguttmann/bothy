// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'branding.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Branding extends Branding {
  @override
  final String productName;
  @override
  final String? accentColor;
  @override
  final String? logoFileId;
  @override
  final int version;

  factory _$Branding([void Function(BrandingBuilder)? updates]) =>
      (BrandingBuilder()..update(updates))._build();

  _$Branding._({
    required this.productName,
    this.accentColor,
    this.logoFileId,
    required this.version,
  }) : super._();
  @override
  Branding rebuild(void Function(BrandingBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  BrandingBuilder toBuilder() => BrandingBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Branding &&
        productName == other.productName &&
        accentColor == other.accentColor &&
        logoFileId == other.logoFileId &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, productName.hashCode);
    _$hash = $jc(_$hash, accentColor.hashCode);
    _$hash = $jc(_$hash, logoFileId.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Branding')
          ..add('productName', productName)
          ..add('accentColor', accentColor)
          ..add('logoFileId', logoFileId)
          ..add('version', version))
        .toString();
  }
}

class BrandingBuilder implements Builder<Branding, BrandingBuilder> {
  _$Branding? _$v;

  String? _productName;
  String? get productName => _$this._productName;
  set productName(String? productName) => _$this._productName = productName;

  String? _accentColor;
  String? get accentColor => _$this._accentColor;
  set accentColor(String? accentColor) => _$this._accentColor = accentColor;

  String? _logoFileId;
  String? get logoFileId => _$this._logoFileId;
  set logoFileId(String? logoFileId) => _$this._logoFileId = logoFileId;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  BrandingBuilder() {
    Branding._defaults(this);
  }

  BrandingBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _productName = $v.productName;
      _accentColor = $v.accentColor;
      _logoFileId = $v.logoFileId;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Branding other) {
    _$v = other as _$Branding;
  }

  @override
  void update(void Function(BrandingBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Branding build() => _build();

  _$Branding _build() {
    final _$result =
        _$v ??
        _$Branding._(
          productName: BuiltValueNullFieldError.checkNotNull(
            productName,
            r'Branding',
            'productName',
          ),
          accentColor: accentColor,
          logoFileId: logoFileId,
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'Branding',
            'version',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

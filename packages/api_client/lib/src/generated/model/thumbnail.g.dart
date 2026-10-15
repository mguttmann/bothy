// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thumbnail.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ThumbnailVariantEnum _$thumbnailVariantEnum_small =
    const ThumbnailVariantEnum._('small');
const ThumbnailVariantEnum _$thumbnailVariantEnum_medium =
    const ThumbnailVariantEnum._('medium');
const ThumbnailVariantEnum _$thumbnailVariantEnum_large =
    const ThumbnailVariantEnum._('large');
const ThumbnailVariantEnum _$thumbnailVariantEnum_poster =
    const ThumbnailVariantEnum._('poster');
const ThumbnailVariantEnum _$thumbnailVariantEnum_unknownDefaultOpenApi =
    const ThumbnailVariantEnum._('unknownDefaultOpenApi');

ThumbnailVariantEnum _$thumbnailVariantEnumValueOf(String name) {
  switch (name) {
    case 'small':
      return _$thumbnailVariantEnum_small;
    case 'medium':
      return _$thumbnailVariantEnum_medium;
    case 'large':
      return _$thumbnailVariantEnum_large;
    case 'poster':
      return _$thumbnailVariantEnum_poster;
    case 'unknownDefaultOpenApi':
      return _$thumbnailVariantEnum_unknownDefaultOpenApi;
    default:
      return _$thumbnailVariantEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ThumbnailVariantEnum> _$thumbnailVariantEnumValues =
    BuiltSet<ThumbnailVariantEnum>(const <ThumbnailVariantEnum>[
      _$thumbnailVariantEnum_small,
      _$thumbnailVariantEnum_medium,
      _$thumbnailVariantEnum_large,
      _$thumbnailVariantEnum_poster,
      _$thumbnailVariantEnum_unknownDefaultOpenApi,
    ]);

Serializer<ThumbnailVariantEnum> _$thumbnailVariantEnumSerializer =
    _$ThumbnailVariantEnumSerializer();

class _$ThumbnailVariantEnumSerializer
    implements PrimitiveSerializer<ThumbnailVariantEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'small': 'small',
    'medium': 'medium',
    'large': 'large',
    'poster': 'poster',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'small': 'small',
    'medium': 'medium',
    'large': 'large',
    'poster': 'poster',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ThumbnailVariantEnum];
  @override
  final String wireName = 'ThumbnailVariantEnum';

  @override
  Object serialize(
    Serializers serializers,
    ThumbnailVariantEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ThumbnailVariantEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ThumbnailVariantEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$Thumbnail extends Thumbnail {
  @override
  final ThumbnailVariantEnum variant;
  @override
  final String mimeType;
  @override
  final int width;
  @override
  final int height;

  factory _$Thumbnail([void Function(ThumbnailBuilder)? updates]) =>
      (ThumbnailBuilder()..update(updates))._build();

  _$Thumbnail._({
    required this.variant,
    required this.mimeType,
    required this.width,
    required this.height,
  }) : super._();
  @override
  Thumbnail rebuild(void Function(ThumbnailBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ThumbnailBuilder toBuilder() => ThumbnailBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Thumbnail &&
        variant == other.variant &&
        mimeType == other.mimeType &&
        width == other.width &&
        height == other.height;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, variant.hashCode);
    _$hash = $jc(_$hash, mimeType.hashCode);
    _$hash = $jc(_$hash, width.hashCode);
    _$hash = $jc(_$hash, height.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Thumbnail')
          ..add('variant', variant)
          ..add('mimeType', mimeType)
          ..add('width', width)
          ..add('height', height))
        .toString();
  }
}

class ThumbnailBuilder implements Builder<Thumbnail, ThumbnailBuilder> {
  _$Thumbnail? _$v;

  ThumbnailVariantEnum? _variant;
  ThumbnailVariantEnum? get variant => _$this._variant;
  set variant(ThumbnailVariantEnum? variant) => _$this._variant = variant;

  String? _mimeType;
  String? get mimeType => _$this._mimeType;
  set mimeType(String? mimeType) => _$this._mimeType = mimeType;

  int? _width;
  int? get width => _$this._width;
  set width(int? width) => _$this._width = width;

  int? _height;
  int? get height => _$this._height;
  set height(int? height) => _$this._height = height;

  ThumbnailBuilder() {
    Thumbnail._defaults(this);
  }

  ThumbnailBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _variant = $v.variant;
      _mimeType = $v.mimeType;
      _width = $v.width;
      _height = $v.height;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Thumbnail other) {
    _$v = other as _$Thumbnail;
  }

  @override
  void update(void Function(ThumbnailBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Thumbnail build() => _build();

  _$Thumbnail _build() {
    final _$result =
        _$v ??
        _$Thumbnail._(
          variant: BuiltValueNullFieldError.checkNotNull(
            variant,
            r'Thumbnail',
            'variant',
          ),
          mimeType: BuiltValueNullFieldError.checkNotNull(
            mimeType,
            r'Thumbnail',
            'mimeType',
          ),
          width: BuiltValueNullFieldError.checkNotNull(
            width,
            r'Thumbnail',
            'width',
          ),
          height: BuiltValueNullFieldError.checkNotNull(
            height,
            r'Thumbnail',
            'height',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'file_ticket_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const FileTicketCreateVariantEnum _$fileTicketCreateVariantEnum_original =
    const FileTicketCreateVariantEnum._('original');
const FileTicketCreateVariantEnum _$fileTicketCreateVariantEnum_small =
    const FileTicketCreateVariantEnum._('small');
const FileTicketCreateVariantEnum _$fileTicketCreateVariantEnum_medium =
    const FileTicketCreateVariantEnum._('medium');
const FileTicketCreateVariantEnum _$fileTicketCreateVariantEnum_large =
    const FileTicketCreateVariantEnum._('large');
const FileTicketCreateVariantEnum _$fileTicketCreateVariantEnum_poster =
    const FileTicketCreateVariantEnum._('poster');
const FileTicketCreateVariantEnum
_$fileTicketCreateVariantEnum_unknownDefaultOpenApi =
    const FileTicketCreateVariantEnum._('unknownDefaultOpenApi');

FileTicketCreateVariantEnum _$fileTicketCreateVariantEnumValueOf(String name) {
  switch (name) {
    case 'original':
      return _$fileTicketCreateVariantEnum_original;
    case 'small':
      return _$fileTicketCreateVariantEnum_small;
    case 'medium':
      return _$fileTicketCreateVariantEnum_medium;
    case 'large':
      return _$fileTicketCreateVariantEnum_large;
    case 'poster':
      return _$fileTicketCreateVariantEnum_poster;
    case 'unknownDefaultOpenApi':
      return _$fileTicketCreateVariantEnum_unknownDefaultOpenApi;
    default:
      return _$fileTicketCreateVariantEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<FileTicketCreateVariantEnum>
_$fileTicketCreateVariantEnumValues = BuiltSet<FileTicketCreateVariantEnum>(
  const <FileTicketCreateVariantEnum>[
    _$fileTicketCreateVariantEnum_original,
    _$fileTicketCreateVariantEnum_small,
    _$fileTicketCreateVariantEnum_medium,
    _$fileTicketCreateVariantEnum_large,
    _$fileTicketCreateVariantEnum_poster,
    _$fileTicketCreateVariantEnum_unknownDefaultOpenApi,
  ],
);

Serializer<FileTicketCreateVariantEnum>
_$fileTicketCreateVariantEnumSerializer =
    _$FileTicketCreateVariantEnumSerializer();

class _$FileTicketCreateVariantEnumSerializer
    implements PrimitiveSerializer<FileTicketCreateVariantEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'original': 'original',
    'small': 'small',
    'medium': 'medium',
    'large': 'large',
    'poster': 'poster',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'original': 'original',
    'small': 'small',
    'medium': 'medium',
    'large': 'large',
    'poster': 'poster',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[FileTicketCreateVariantEnum];
  @override
  final String wireName = 'FileTicketCreateVariantEnum';

  @override
  Object serialize(
    Serializers serializers,
    FileTicketCreateVariantEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  FileTicketCreateVariantEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => FileTicketCreateVariantEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$FileTicketCreate extends FileTicketCreate {
  @override
  final FileTicketCreateVariantEnum variant;

  factory _$FileTicketCreate([
    void Function(FileTicketCreateBuilder)? updates,
  ]) => (FileTicketCreateBuilder()..update(updates))._build();

  _$FileTicketCreate._({required this.variant}) : super._();
  @override
  FileTicketCreate rebuild(void Function(FileTicketCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  FileTicketCreateBuilder toBuilder() =>
      FileTicketCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is FileTicketCreate && variant == other.variant;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, variant.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'FileTicketCreate',
    )..add('variant', variant)).toString();
  }
}

class FileTicketCreateBuilder
    implements Builder<FileTicketCreate, FileTicketCreateBuilder> {
  _$FileTicketCreate? _$v;

  FileTicketCreateVariantEnum? _variant;
  FileTicketCreateVariantEnum? get variant => _$this._variant;
  set variant(FileTicketCreateVariantEnum? variant) =>
      _$this._variant = variant;

  FileTicketCreateBuilder() {
    FileTicketCreate._defaults(this);
  }

  FileTicketCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _variant = $v.variant;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(FileTicketCreate other) {
    _$v = other as _$FileTicketCreate;
  }

  @override
  void update(void Function(FileTicketCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  FileTicketCreate build() => _build();

  _$FileTicketCreate _build() {
    final _$result =
        _$v ??
        _$FileTicketCreate._(
          variant: BuiltValueNullFieldError.checkNotNull(
            variant,
            r'FileTicketCreate',
            'variant',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

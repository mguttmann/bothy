// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_object.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ImportObjectItemKindEnum _$importObjectItemKindEnum_space =
    const ImportObjectItemKindEnum._('space');
const ImportObjectItemKindEnum _$importObjectItemKindEnum_channel =
    const ImportObjectItemKindEnum._('channel');
const ImportObjectItemKindEnum _$importObjectItemKindEnum_forumTag =
    const ImportObjectItemKindEnum._('forumTag');
const ImportObjectItemKindEnum _$importObjectItemKindEnum_thread =
    const ImportObjectItemKindEnum._('thread');
const ImportObjectItemKindEnum _$importObjectItemKindEnum_message =
    const ImportObjectItemKindEnum._('message');
const ImportObjectItemKindEnum _$importObjectItemKindEnum_user =
    const ImportObjectItemKindEnum._('user');
const ImportObjectItemKindEnum _$importObjectItemKindEnum_pin =
    const ImportObjectItemKindEnum._('pin');
const ImportObjectItemKindEnum
_$importObjectItemKindEnum_unknownDefaultOpenApi =
    const ImportObjectItemKindEnum._('unknownDefaultOpenApi');

ImportObjectItemKindEnum _$importObjectItemKindEnumValueOf(String name) {
  switch (name) {
    case 'space':
      return _$importObjectItemKindEnum_space;
    case 'channel':
      return _$importObjectItemKindEnum_channel;
    case 'forumTag':
      return _$importObjectItemKindEnum_forumTag;
    case 'thread':
      return _$importObjectItemKindEnum_thread;
    case 'message':
      return _$importObjectItemKindEnum_message;
    case 'user':
      return _$importObjectItemKindEnum_user;
    case 'pin':
      return _$importObjectItemKindEnum_pin;
    case 'unknownDefaultOpenApi':
      return _$importObjectItemKindEnum_unknownDefaultOpenApi;
    default:
      return _$importObjectItemKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ImportObjectItemKindEnum> _$importObjectItemKindEnumValues =
    BuiltSet<ImportObjectItemKindEnum>(const <ImportObjectItemKindEnum>[
      _$importObjectItemKindEnum_space,
      _$importObjectItemKindEnum_channel,
      _$importObjectItemKindEnum_forumTag,
      _$importObjectItemKindEnum_thread,
      _$importObjectItemKindEnum_message,
      _$importObjectItemKindEnum_user,
      _$importObjectItemKindEnum_pin,
      _$importObjectItemKindEnum_unknownDefaultOpenApi,
    ]);

Serializer<ImportObjectItemKindEnum> _$importObjectItemKindEnumSerializer =
    _$ImportObjectItemKindEnumSerializer();

class _$ImportObjectItemKindEnumSerializer
    implements PrimitiveSerializer<ImportObjectItemKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'space': 'space',
    'channel': 'channel',
    'forumTag': 'forum_tag',
    'thread': 'thread',
    'message': 'message',
    'user': 'user',
    'pin': 'pin',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'space': 'space',
    'channel': 'channel',
    'forum_tag': 'forumTag',
    'thread': 'thread',
    'message': 'message',
    'user': 'user',
    'pin': 'pin',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ImportObjectItemKindEnum];
  @override
  final String wireName = 'ImportObjectItemKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    ImportObjectItemKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ImportObjectItemKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ImportObjectItemKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ImportObject extends ImportObject {
  @override
  final String originKey;
  @override
  final ImportObjectItemKindEnum itemKind;
  @override
  final String? contentSha256;
  @override
  final BuiltMap<String, JsonObject?> data;

  factory _$ImportObject([void Function(ImportObjectBuilder)? updates]) =>
      (ImportObjectBuilder()..update(updates))._build();

  _$ImportObject._({
    required this.originKey,
    required this.itemKind,
    this.contentSha256,
    required this.data,
  }) : super._();
  @override
  ImportObject rebuild(void Function(ImportObjectBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportObjectBuilder toBuilder() => ImportObjectBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportObject &&
        originKey == other.originKey &&
        itemKind == other.itemKind &&
        contentSha256 == other.contentSha256 &&
        data == other.data;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, originKey.hashCode);
    _$hash = $jc(_$hash, itemKind.hashCode);
    _$hash = $jc(_$hash, contentSha256.hashCode);
    _$hash = $jc(_$hash, data.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportObject')
          ..add('originKey', originKey)
          ..add('itemKind', itemKind)
          ..add('contentSha256', contentSha256)
          ..add('data', data))
        .toString();
  }
}

class ImportObjectBuilder
    implements Builder<ImportObject, ImportObjectBuilder> {
  _$ImportObject? _$v;

  String? _originKey;
  String? get originKey => _$this._originKey;
  set originKey(String? originKey) => _$this._originKey = originKey;

  ImportObjectItemKindEnum? _itemKind;
  ImportObjectItemKindEnum? get itemKind => _$this._itemKind;
  set itemKind(ImportObjectItemKindEnum? itemKind) =>
      _$this._itemKind = itemKind;

  String? _contentSha256;
  String? get contentSha256 => _$this._contentSha256;
  set contentSha256(String? contentSha256) =>
      _$this._contentSha256 = contentSha256;

  MapBuilder<String, JsonObject?>? _data;
  MapBuilder<String, JsonObject?> get data =>
      _$this._data ??= MapBuilder<String, JsonObject?>();
  set data(MapBuilder<String, JsonObject?>? data) => _$this._data = data;

  ImportObjectBuilder() {
    ImportObject._defaults(this);
  }

  ImportObjectBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _originKey = $v.originKey;
      _itemKind = $v.itemKind;
      _contentSha256 = $v.contentSha256;
      _data = $v.data.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportObject other) {
    _$v = other as _$ImportObject;
  }

  @override
  void update(void Function(ImportObjectBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportObject build() => _build();

  _$ImportObject _build() {
    _$ImportObject _$result;
    try {
      _$result =
          _$v ??
          _$ImportObject._(
            originKey: BuiltValueNullFieldError.checkNotNull(
              originKey,
              r'ImportObject',
              'originKey',
            ),
            itemKind: BuiltValueNullFieldError.checkNotNull(
              itemKind,
              r'ImportObject',
              'itemKind',
            ),
            contentSha256: contentSha256,
            data: data.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'data';
        data.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportObject',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

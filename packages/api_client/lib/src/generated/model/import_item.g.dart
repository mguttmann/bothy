// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_item.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ImportItemItemKindEnum _$importItemItemKindEnum_space =
    const ImportItemItemKindEnum._('space');
const ImportItemItemKindEnum _$importItemItemKindEnum_channel =
    const ImportItemItemKindEnum._('channel');
const ImportItemItemKindEnum _$importItemItemKindEnum_forumTag =
    const ImportItemItemKindEnum._('forumTag');
const ImportItemItemKindEnum _$importItemItemKindEnum_thread =
    const ImportItemItemKindEnum._('thread');
const ImportItemItemKindEnum _$importItemItemKindEnum_message =
    const ImportItemItemKindEnum._('message');
const ImportItemItemKindEnum _$importItemItemKindEnum_file =
    const ImportItemItemKindEnum._('file');
const ImportItemItemKindEnum _$importItemItemKindEnum_user =
    const ImportItemItemKindEnum._('user');
const ImportItemItemKindEnum _$importItemItemKindEnum_reaction =
    const ImportItemItemKindEnum._('reaction');
const ImportItemItemKindEnum _$importItemItemKindEnum_pin =
    const ImportItemItemKindEnum._('pin');
const ImportItemItemKindEnum _$importItemItemKindEnum_unknownDefaultOpenApi =
    const ImportItemItemKindEnum._('unknownDefaultOpenApi');

ImportItemItemKindEnum _$importItemItemKindEnumValueOf(String name) {
  switch (name) {
    case 'space':
      return _$importItemItemKindEnum_space;
    case 'channel':
      return _$importItemItemKindEnum_channel;
    case 'forumTag':
      return _$importItemItemKindEnum_forumTag;
    case 'thread':
      return _$importItemItemKindEnum_thread;
    case 'message':
      return _$importItemItemKindEnum_message;
    case 'file':
      return _$importItemItemKindEnum_file;
    case 'user':
      return _$importItemItemKindEnum_user;
    case 'reaction':
      return _$importItemItemKindEnum_reaction;
    case 'pin':
      return _$importItemItemKindEnum_pin;
    case 'unknownDefaultOpenApi':
      return _$importItemItemKindEnum_unknownDefaultOpenApi;
    default:
      return _$importItemItemKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ImportItemItemKindEnum> _$importItemItemKindEnumValues =
    BuiltSet<ImportItemItemKindEnum>(const <ImportItemItemKindEnum>[
      _$importItemItemKindEnum_space,
      _$importItemItemKindEnum_channel,
      _$importItemItemKindEnum_forumTag,
      _$importItemItemKindEnum_thread,
      _$importItemItemKindEnum_message,
      _$importItemItemKindEnum_file,
      _$importItemItemKindEnum_user,
      _$importItemItemKindEnum_reaction,
      _$importItemItemKindEnum_pin,
      _$importItemItemKindEnum_unknownDefaultOpenApi,
    ]);

Serializer<ImportItemItemKindEnum> _$importItemItemKindEnumSerializer =
    _$ImportItemItemKindEnumSerializer();

class _$ImportItemItemKindEnumSerializer
    implements PrimitiveSerializer<ImportItemItemKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'space': 'space',
    'channel': 'channel',
    'forumTag': 'forum_tag',
    'thread': 'thread',
    'message': 'message',
    'file': 'file',
    'user': 'user',
    'reaction': 'reaction',
    'pin': 'pin',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'space': 'space',
    'channel': 'channel',
    'forum_tag': 'forumTag',
    'thread': 'thread',
    'message': 'message',
    'file': 'file',
    'user': 'user',
    'reaction': 'reaction',
    'pin': 'pin',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ImportItemItemKindEnum];
  @override
  final String wireName = 'ImportItemItemKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    ImportItemItemKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ImportItemItemKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ImportItemItemKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ImportItem extends ImportItem {
  @override
  final String id;
  @override
  final String originKey;
  @override
  final ImportItemItemKindEnum itemKind;
  @override
  final String? targetKind;
  @override
  final String? targetId;
  @override
  final ImportItemState state;
  @override
  final String? error;
  @override
  final DateTime createdAt;

  factory _$ImportItem([void Function(ImportItemBuilder)? updates]) =>
      (ImportItemBuilder()..update(updates))._build();

  _$ImportItem._({
    required this.id,
    required this.originKey,
    required this.itemKind,
    this.targetKind,
    this.targetId,
    required this.state,
    this.error,
    required this.createdAt,
  }) : super._();
  @override
  ImportItem rebuild(void Function(ImportItemBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportItemBuilder toBuilder() => ImportItemBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportItem &&
        id == other.id &&
        originKey == other.originKey &&
        itemKind == other.itemKind &&
        targetKind == other.targetKind &&
        targetId == other.targetId &&
        state == other.state &&
        error == other.error &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, originKey.hashCode);
    _$hash = $jc(_$hash, itemKind.hashCode);
    _$hash = $jc(_$hash, targetKind.hashCode);
    _$hash = $jc(_$hash, targetId.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, error.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportItem')
          ..add('id', id)
          ..add('originKey', originKey)
          ..add('itemKind', itemKind)
          ..add('targetKind', targetKind)
          ..add('targetId', targetId)
          ..add('state', state)
          ..add('error', error)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class ImportItemBuilder implements Builder<ImportItem, ImportItemBuilder> {
  _$ImportItem? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _originKey;
  String? get originKey => _$this._originKey;
  set originKey(String? originKey) => _$this._originKey = originKey;

  ImportItemItemKindEnum? _itemKind;
  ImportItemItemKindEnum? get itemKind => _$this._itemKind;
  set itemKind(ImportItemItemKindEnum? itemKind) => _$this._itemKind = itemKind;

  String? _targetKind;
  String? get targetKind => _$this._targetKind;
  set targetKind(String? targetKind) => _$this._targetKind = targetKind;

  String? _targetId;
  String? get targetId => _$this._targetId;
  set targetId(String? targetId) => _$this._targetId = targetId;

  ImportItemState? _state;
  ImportItemState? get state => _$this._state;
  set state(ImportItemState? state) => _$this._state = state;

  String? _error;
  String? get error => _$this._error;
  set error(String? error) => _$this._error = error;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  ImportItemBuilder() {
    ImportItem._defaults(this);
  }

  ImportItemBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _originKey = $v.originKey;
      _itemKind = $v.itemKind;
      _targetKind = $v.targetKind;
      _targetId = $v.targetId;
      _state = $v.state;
      _error = $v.error;
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportItem other) {
    _$v = other as _$ImportItem;
  }

  @override
  void update(void Function(ImportItemBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportItem build() => _build();

  _$ImportItem _build() {
    final _$result =
        _$v ??
        _$ImportItem._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'ImportItem', 'id'),
          originKey: BuiltValueNullFieldError.checkNotNull(
            originKey,
            r'ImportItem',
            'originKey',
          ),
          itemKind: BuiltValueNullFieldError.checkNotNull(
            itemKind,
            r'ImportItem',
            'itemKind',
          ),
          targetKind: targetKind,
          targetId: targetId,
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'ImportItem',
            'state',
          ),
          error: error,
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'ImportItem',
            'createdAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

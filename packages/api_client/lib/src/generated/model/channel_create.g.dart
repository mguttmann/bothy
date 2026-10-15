// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ChannelCreateKindEnum _$channelCreateKindEnum_text =
    const ChannelCreateKindEnum._('text');
const ChannelCreateKindEnum _$channelCreateKindEnum_forum =
    const ChannelCreateKindEnum._('forum');
const ChannelCreateKindEnum _$channelCreateKindEnum_unknownDefaultOpenApi =
    const ChannelCreateKindEnum._('unknownDefaultOpenApi');

ChannelCreateKindEnum _$channelCreateKindEnumValueOf(String name) {
  switch (name) {
    case 'text':
      return _$channelCreateKindEnum_text;
    case 'forum':
      return _$channelCreateKindEnum_forum;
    case 'unknownDefaultOpenApi':
      return _$channelCreateKindEnum_unknownDefaultOpenApi;
    default:
      return _$channelCreateKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ChannelCreateKindEnum> _$channelCreateKindEnumValues =
    BuiltSet<ChannelCreateKindEnum>(const <ChannelCreateKindEnum>[
      _$channelCreateKindEnum_text,
      _$channelCreateKindEnum_forum,
      _$channelCreateKindEnum_unknownDefaultOpenApi,
    ]);

Serializer<ChannelCreateKindEnum> _$channelCreateKindEnumSerializer =
    _$ChannelCreateKindEnumSerializer();

class _$ChannelCreateKindEnumSerializer
    implements PrimitiveSerializer<ChannelCreateKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'text': 'text',
    'forum': 'forum',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'text': 'text',
    'forum': 'forum',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ChannelCreateKindEnum];
  @override
  final String wireName = 'ChannelCreateKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    ChannelCreateKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ChannelCreateKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ChannelCreateKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ChannelCreate extends ChannelCreate {
  @override
  final ChannelCreateKindEnum kind;
  @override
  final String name;
  @override
  final String? topic;
  @override
  final ForumSort? forumDefaultSort;
  @override
  final String? afterChannelId;

  factory _$ChannelCreate([void Function(ChannelCreateBuilder)? updates]) =>
      (ChannelCreateBuilder()..update(updates))._build();

  _$ChannelCreate._({
    required this.kind,
    required this.name,
    this.topic,
    this.forumDefaultSort,
    this.afterChannelId,
  }) : super._();
  @override
  ChannelCreate rebuild(void Function(ChannelCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ChannelCreateBuilder toBuilder() => ChannelCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelCreate &&
        kind == other.kind &&
        name == other.name &&
        topic == other.topic &&
        forumDefaultSort == other.forumDefaultSort &&
        afterChannelId == other.afterChannelId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, topic.hashCode);
    _$hash = $jc(_$hash, forumDefaultSort.hashCode);
    _$hash = $jc(_$hash, afterChannelId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ChannelCreate')
          ..add('kind', kind)
          ..add('name', name)
          ..add('topic', topic)
          ..add('forumDefaultSort', forumDefaultSort)
          ..add('afterChannelId', afterChannelId))
        .toString();
  }
}

class ChannelCreateBuilder
    implements Builder<ChannelCreate, ChannelCreateBuilder> {
  _$ChannelCreate? _$v;

  ChannelCreateKindEnum? _kind;
  ChannelCreateKindEnum? get kind => _$this._kind;
  set kind(ChannelCreateKindEnum? kind) => _$this._kind = kind;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _topic;
  String? get topic => _$this._topic;
  set topic(String? topic) => _$this._topic = topic;

  ForumSort? _forumDefaultSort;
  ForumSort? get forumDefaultSort => _$this._forumDefaultSort;
  set forumDefaultSort(ForumSort? forumDefaultSort) =>
      _$this._forumDefaultSort = forumDefaultSort;

  String? _afterChannelId;
  String? get afterChannelId => _$this._afterChannelId;
  set afterChannelId(String? afterChannelId) =>
      _$this._afterChannelId = afterChannelId;

  ChannelCreateBuilder() {
    ChannelCreate._defaults(this);
  }

  ChannelCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _name = $v.name;
      _topic = $v.topic;
      _forumDefaultSort = $v.forumDefaultSort;
      _afterChannelId = $v.afterChannelId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelCreate other) {
    _$v = other as _$ChannelCreate;
  }

  @override
  void update(void Function(ChannelCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelCreate build() => _build();

  _$ChannelCreate _build() {
    final _$result =
        _$v ??
        _$ChannelCreate._(
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'ChannelCreate',
            'kind',
          ),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'ChannelCreate',
            'name',
          ),
          topic: topic,
          forumDefaultSort: forumDefaultSort,
          afterChannelId: afterChannelId,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

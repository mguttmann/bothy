// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ThreadKindEnum _$threadKindEnum_main = const ThreadKindEnum._('main');
const ThreadKindEnum _$threadKindEnum_thread = const ThreadKindEnum._('thread');
const ThreadKindEnum _$threadKindEnum_unknownDefaultOpenApi =
    const ThreadKindEnum._('unknownDefaultOpenApi');

ThreadKindEnum _$threadKindEnumValueOf(String name) {
  switch (name) {
    case 'main':
      return _$threadKindEnum_main;
    case 'thread':
      return _$threadKindEnum_thread;
    case 'unknownDefaultOpenApi':
      return _$threadKindEnum_unknownDefaultOpenApi;
    default:
      return _$threadKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ThreadKindEnum> _$threadKindEnumValues =
    BuiltSet<ThreadKindEnum>(const <ThreadKindEnum>[
      _$threadKindEnum_main,
      _$threadKindEnum_thread,
      _$threadKindEnum_unknownDefaultOpenApi,
    ]);

const ThreadTitleSourceEnum _$threadTitleSourceEnum_auto =
    const ThreadTitleSourceEnum._('auto');
const ThreadTitleSourceEnum _$threadTitleSourceEnum_manual =
    const ThreadTitleSourceEnum._('manual');
const ThreadTitleSourceEnum _$threadTitleSourceEnum_unknownDefaultOpenApi =
    const ThreadTitleSourceEnum._('unknownDefaultOpenApi');

ThreadTitleSourceEnum _$threadTitleSourceEnumValueOf(String name) {
  switch (name) {
    case 'auto':
      return _$threadTitleSourceEnum_auto;
    case 'manual':
      return _$threadTitleSourceEnum_manual;
    case 'unknownDefaultOpenApi':
      return _$threadTitleSourceEnum_unknownDefaultOpenApi;
    default:
      return _$threadTitleSourceEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ThreadTitleSourceEnum> _$threadTitleSourceEnumValues =
    BuiltSet<ThreadTitleSourceEnum>(const <ThreadTitleSourceEnum>[
      _$threadTitleSourceEnum_auto,
      _$threadTitleSourceEnum_manual,
      _$threadTitleSourceEnum_unknownDefaultOpenApi,
    ]);

Serializer<ThreadKindEnum> _$threadKindEnumSerializer =
    _$ThreadKindEnumSerializer();
Serializer<ThreadTitleSourceEnum> _$threadTitleSourceEnumSerializer =
    _$ThreadTitleSourceEnumSerializer();

class _$ThreadKindEnumSerializer
    implements PrimitiveSerializer<ThreadKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'main': 'main',
    'thread': 'thread',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'main': 'main',
    'thread': 'thread',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ThreadKindEnum];
  @override
  final String wireName = 'ThreadKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    ThreadKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ThreadKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ThreadKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ThreadTitleSourceEnumSerializer
    implements PrimitiveSerializer<ThreadTitleSourceEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'auto': 'auto',
    'manual': 'manual',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'auto': 'auto',
    'manual': 'manual',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ThreadTitleSourceEnum];
  @override
  final String wireName = 'ThreadTitleSourceEnum';

  @override
  Object serialize(
    Serializers serializers,
    ThreadTitleSourceEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ThreadTitleSourceEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ThreadTitleSourceEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$Thread extends Thread {
  @override
  final String id;
  @override
  final String channelId;
  @override
  final ThreadKindEnum kind;
  @override
  final String? parentMessageId;
  @override
  final String title;
  @override
  final ThreadTitleSourceEnum titleSource;
  @override
  final ThreadState state;
  @override
  final DateTime? closedAt;
  @override
  final String? closedBy;
  @override
  final String? operatorAgentId;
  @override
  final BillingMode? billingMode;
  @override
  final int lastMessageSeq;
  @override
  final int? unreadFloorSeq;
  @override
  final DateTime lastActivityAt;
  @override
  final int messageCount;
  @override
  final String createdBy;
  @override
  final DateTime createdAt;
  @override
  final DateTime? movedAt;
  @override
  final int version;
  @override
  final BuiltList<String> tagIds;

  factory _$Thread([void Function(ThreadBuilder)? updates]) =>
      (ThreadBuilder()..update(updates))._build();

  _$Thread._({
    required this.id,
    required this.channelId,
    required this.kind,
    this.parentMessageId,
    required this.title,
    required this.titleSource,
    required this.state,
    this.closedAt,
    this.closedBy,
    this.operatorAgentId,
    this.billingMode,
    required this.lastMessageSeq,
    this.unreadFloorSeq,
    required this.lastActivityAt,
    required this.messageCount,
    required this.createdBy,
    required this.createdAt,
    this.movedAt,
    required this.version,
    required this.tagIds,
  }) : super._();
  @override
  Thread rebuild(void Function(ThreadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ThreadBuilder toBuilder() => ThreadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Thread &&
        id == other.id &&
        channelId == other.channelId &&
        kind == other.kind &&
        parentMessageId == other.parentMessageId &&
        title == other.title &&
        titleSource == other.titleSource &&
        state == other.state &&
        closedAt == other.closedAt &&
        closedBy == other.closedBy &&
        operatorAgentId == other.operatorAgentId &&
        billingMode == other.billingMode &&
        lastMessageSeq == other.lastMessageSeq &&
        unreadFloorSeq == other.unreadFloorSeq &&
        lastActivityAt == other.lastActivityAt &&
        messageCount == other.messageCount &&
        createdBy == other.createdBy &&
        createdAt == other.createdAt &&
        movedAt == other.movedAt &&
        version == other.version &&
        tagIds == other.tagIds;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, parentMessageId.hashCode);
    _$hash = $jc(_$hash, title.hashCode);
    _$hash = $jc(_$hash, titleSource.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, closedAt.hashCode);
    _$hash = $jc(_$hash, closedBy.hashCode);
    _$hash = $jc(_$hash, operatorAgentId.hashCode);
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, lastMessageSeq.hashCode);
    _$hash = $jc(_$hash, unreadFloorSeq.hashCode);
    _$hash = $jc(_$hash, lastActivityAt.hashCode);
    _$hash = $jc(_$hash, messageCount.hashCode);
    _$hash = $jc(_$hash, createdBy.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, movedAt.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, tagIds.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Thread')
          ..add('id', id)
          ..add('channelId', channelId)
          ..add('kind', kind)
          ..add('parentMessageId', parentMessageId)
          ..add('title', title)
          ..add('titleSource', titleSource)
          ..add('state', state)
          ..add('closedAt', closedAt)
          ..add('closedBy', closedBy)
          ..add('operatorAgentId', operatorAgentId)
          ..add('billingMode', billingMode)
          ..add('lastMessageSeq', lastMessageSeq)
          ..add('unreadFloorSeq', unreadFloorSeq)
          ..add('lastActivityAt', lastActivityAt)
          ..add('messageCount', messageCount)
          ..add('createdBy', createdBy)
          ..add('createdAt', createdAt)
          ..add('movedAt', movedAt)
          ..add('version', version)
          ..add('tagIds', tagIds))
        .toString();
  }
}

class ThreadBuilder implements Builder<Thread, ThreadBuilder> {
  _$Thread? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  ThreadKindEnum? _kind;
  ThreadKindEnum? get kind => _$this._kind;
  set kind(ThreadKindEnum? kind) => _$this._kind = kind;

  String? _parentMessageId;
  String? get parentMessageId => _$this._parentMessageId;
  set parentMessageId(String? parentMessageId) =>
      _$this._parentMessageId = parentMessageId;

  String? _title;
  String? get title => _$this._title;
  set title(String? title) => _$this._title = title;

  ThreadTitleSourceEnum? _titleSource;
  ThreadTitleSourceEnum? get titleSource => _$this._titleSource;
  set titleSource(ThreadTitleSourceEnum? titleSource) =>
      _$this._titleSource = titleSource;

  ThreadState? _state;
  ThreadState? get state => _$this._state;
  set state(ThreadState? state) => _$this._state = state;

  DateTime? _closedAt;
  DateTime? get closedAt => _$this._closedAt;
  set closedAt(DateTime? closedAt) => _$this._closedAt = closedAt;

  String? _closedBy;
  String? get closedBy => _$this._closedBy;
  set closedBy(String? closedBy) => _$this._closedBy = closedBy;

  String? _operatorAgentId;
  String? get operatorAgentId => _$this._operatorAgentId;
  set operatorAgentId(String? operatorAgentId) =>
      _$this._operatorAgentId = operatorAgentId;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  int? _lastMessageSeq;
  int? get lastMessageSeq => _$this._lastMessageSeq;
  set lastMessageSeq(int? lastMessageSeq) =>
      _$this._lastMessageSeq = lastMessageSeq;

  int? _unreadFloorSeq;
  int? get unreadFloorSeq => _$this._unreadFloorSeq;
  set unreadFloorSeq(int? unreadFloorSeq) =>
      _$this._unreadFloorSeq = unreadFloorSeq;

  DateTime? _lastActivityAt;
  DateTime? get lastActivityAt => _$this._lastActivityAt;
  set lastActivityAt(DateTime? lastActivityAt) =>
      _$this._lastActivityAt = lastActivityAt;

  int? _messageCount;
  int? get messageCount => _$this._messageCount;
  set messageCount(int? messageCount) => _$this._messageCount = messageCount;

  String? _createdBy;
  String? get createdBy => _$this._createdBy;
  set createdBy(String? createdBy) => _$this._createdBy = createdBy;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _movedAt;
  DateTime? get movedAt => _$this._movedAt;
  set movedAt(DateTime? movedAt) => _$this._movedAt = movedAt;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  ListBuilder<String>? _tagIds;
  ListBuilder<String> get tagIds => _$this._tagIds ??= ListBuilder<String>();
  set tagIds(ListBuilder<String>? tagIds) => _$this._tagIds = tagIds;

  ThreadBuilder() {
    Thread._defaults(this);
  }

  ThreadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _channelId = $v.channelId;
      _kind = $v.kind;
      _parentMessageId = $v.parentMessageId;
      _title = $v.title;
      _titleSource = $v.titleSource;
      _state = $v.state;
      _closedAt = $v.closedAt;
      _closedBy = $v.closedBy;
      _operatorAgentId = $v.operatorAgentId;
      _billingMode = $v.billingMode;
      _lastMessageSeq = $v.lastMessageSeq;
      _unreadFloorSeq = $v.unreadFloorSeq;
      _lastActivityAt = $v.lastActivityAt;
      _messageCount = $v.messageCount;
      _createdBy = $v.createdBy;
      _createdAt = $v.createdAt;
      _movedAt = $v.movedAt;
      _version = $v.version;
      _tagIds = $v.tagIds.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Thread other) {
    _$v = other as _$Thread;
  }

  @override
  void update(void Function(ThreadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Thread build() => _build();

  _$Thread _build() {
    _$Thread _$result;
    try {
      _$result =
          _$v ??
          _$Thread._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'Thread', 'id'),
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'Thread',
              'channelId',
            ),
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'Thread',
              'kind',
            ),
            parentMessageId: parentMessageId,
            title: BuiltValueNullFieldError.checkNotNull(
              title,
              r'Thread',
              'title',
            ),
            titleSource: BuiltValueNullFieldError.checkNotNull(
              titleSource,
              r'Thread',
              'titleSource',
            ),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'Thread',
              'state',
            ),
            closedAt: closedAt,
            closedBy: closedBy,
            operatorAgentId: operatorAgentId,
            billingMode: billingMode,
            lastMessageSeq: BuiltValueNullFieldError.checkNotNull(
              lastMessageSeq,
              r'Thread',
              'lastMessageSeq',
            ),
            unreadFloorSeq: unreadFloorSeq,
            lastActivityAt: BuiltValueNullFieldError.checkNotNull(
              lastActivityAt,
              r'Thread',
              'lastActivityAt',
            ),
            messageCount: BuiltValueNullFieldError.checkNotNull(
              messageCount,
              r'Thread',
              'messageCount',
            ),
            createdBy: BuiltValueNullFieldError.checkNotNull(
              createdBy,
              r'Thread',
              'createdBy',
            ),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'Thread',
              'createdAt',
            ),
            movedAt: movedAt,
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'Thread',
              'version',
            ),
            tagIds: tagIds.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'tagIds';
        tagIds.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Thread',
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

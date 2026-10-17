// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'message.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const MessageSystemCodeEnum _$messageSystemCodeEnum_threadPeriodReopened =
    const MessageSystemCodeEnum._('threadPeriodReopened');
const MessageSystemCodeEnum
_$messageSystemCodeEnum_agentSessionPeriodResumeFailed =
    const MessageSystemCodeEnum._('agentSessionPeriodResumeFailed');
const MessageSystemCodeEnum _$messageSystemCodeEnum_unknownDefaultOpenApi =
    const MessageSystemCodeEnum._('unknownDefaultOpenApi');

MessageSystemCodeEnum _$messageSystemCodeEnumValueOf(String name) {
  switch (name) {
    case 'threadPeriodReopened':
      return _$messageSystemCodeEnum_threadPeriodReopened;
    case 'agentSessionPeriodResumeFailed':
      return _$messageSystemCodeEnum_agentSessionPeriodResumeFailed;
    case 'unknownDefaultOpenApi':
      return _$messageSystemCodeEnum_unknownDefaultOpenApi;
    default:
      return _$messageSystemCodeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<MessageSystemCodeEnum> _$messageSystemCodeEnumValues =
    BuiltSet<MessageSystemCodeEnum>(const <MessageSystemCodeEnum>[
      _$messageSystemCodeEnum_threadPeriodReopened,
      _$messageSystemCodeEnum_agentSessionPeriodResumeFailed,
      _$messageSystemCodeEnum_unknownDefaultOpenApi,
    ]);

Serializer<MessageSystemCodeEnum> _$messageSystemCodeEnumSerializer =
    _$MessageSystemCodeEnumSerializer();

class _$MessageSystemCodeEnumSerializer
    implements PrimitiveSerializer<MessageSystemCodeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'threadPeriodReopened': 'thread.reopened',
    'agentSessionPeriodResumeFailed': 'agent_session.resume_failed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'thread.reopened': 'threadPeriodReopened',
    'agent_session.resume_failed': 'agentSessionPeriodResumeFailed',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[MessageSystemCodeEnum];
  @override
  final String wireName = 'MessageSystemCodeEnum';

  @override
  Object serialize(
    Serializers serializers,
    MessageSystemCodeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  MessageSystemCodeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => MessageSystemCodeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$Message extends Message {
  @override
  final String id;
  @override
  final String threadId;
  @override
  final String channelId;
  @override
  final int threadSeq;
  @override
  final PrincipalRef author;
  @override
  final String clientMsgId;
  @override
  final MessageOrigin origin;
  @override
  final MessageKind kind;
  @override
  final MessageSystemCodeEnum? systemCode;
  @override
  final String contentMd;
  @override
  final int contentFormat;
  @override
  final bool? contentTruncated;
  @override
  final String? replyToMessageId;
  @override
  final String? runId;
  @override
  final TriggerState? triggerState;
  @override
  final DateTime? triggerStateAt;
  @override
  final String? triggerStateReason;
  @override
  final bool silent;
  @override
  final BuiltList<Attachment> attachments;
  @override
  final BuiltList<String> mentionIds;
  @override
  final BuiltList<ReactionSummary> reactions;
  @override
  final BuiltList<String>? myReactions;
  @override
  final bool pinned;
  @override
  final BuiltList<Component> components;
  @override
  final int version;
  @override
  final DateTime? editedAt;
  @override
  final DateTime? deletedAt;
  @override
  final DateTime createdAt;

  factory _$Message([void Function(MessageBuilder)? updates]) =>
      (MessageBuilder()..update(updates))._build();

  _$Message._({
    required this.id,
    required this.threadId,
    required this.channelId,
    required this.threadSeq,
    required this.author,
    required this.clientMsgId,
    required this.origin,
    required this.kind,
    this.systemCode,
    required this.contentMd,
    required this.contentFormat,
    this.contentTruncated,
    this.replyToMessageId,
    this.runId,
    this.triggerState,
    this.triggerStateAt,
    this.triggerStateReason,
    required this.silent,
    required this.attachments,
    required this.mentionIds,
    required this.reactions,
    this.myReactions,
    required this.pinned,
    required this.components,
    required this.version,
    this.editedAt,
    this.deletedAt,
    required this.createdAt,
  }) : super._();
  @override
  Message rebuild(void Function(MessageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  MessageBuilder toBuilder() => MessageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Message &&
        id == other.id &&
        threadId == other.threadId &&
        channelId == other.channelId &&
        threadSeq == other.threadSeq &&
        author == other.author &&
        clientMsgId == other.clientMsgId &&
        origin == other.origin &&
        kind == other.kind &&
        systemCode == other.systemCode &&
        contentMd == other.contentMd &&
        contentFormat == other.contentFormat &&
        contentTruncated == other.contentTruncated &&
        replyToMessageId == other.replyToMessageId &&
        runId == other.runId &&
        triggerState == other.triggerState &&
        triggerStateAt == other.triggerStateAt &&
        triggerStateReason == other.triggerStateReason &&
        silent == other.silent &&
        attachments == other.attachments &&
        mentionIds == other.mentionIds &&
        reactions == other.reactions &&
        myReactions == other.myReactions &&
        pinned == other.pinned &&
        components == other.components &&
        version == other.version &&
        editedAt == other.editedAt &&
        deletedAt == other.deletedAt &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, threadSeq.hashCode);
    _$hash = $jc(_$hash, author.hashCode);
    _$hash = $jc(_$hash, clientMsgId.hashCode);
    _$hash = $jc(_$hash, origin.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, systemCode.hashCode);
    _$hash = $jc(_$hash, contentMd.hashCode);
    _$hash = $jc(_$hash, contentFormat.hashCode);
    _$hash = $jc(_$hash, contentTruncated.hashCode);
    _$hash = $jc(_$hash, replyToMessageId.hashCode);
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, triggerState.hashCode);
    _$hash = $jc(_$hash, triggerStateAt.hashCode);
    _$hash = $jc(_$hash, triggerStateReason.hashCode);
    _$hash = $jc(_$hash, silent.hashCode);
    _$hash = $jc(_$hash, attachments.hashCode);
    _$hash = $jc(_$hash, mentionIds.hashCode);
    _$hash = $jc(_$hash, reactions.hashCode);
    _$hash = $jc(_$hash, myReactions.hashCode);
    _$hash = $jc(_$hash, pinned.hashCode);
    _$hash = $jc(_$hash, components.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, editedAt.hashCode);
    _$hash = $jc(_$hash, deletedAt.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Message')
          ..add('id', id)
          ..add('threadId', threadId)
          ..add('channelId', channelId)
          ..add('threadSeq', threadSeq)
          ..add('author', author)
          ..add('clientMsgId', clientMsgId)
          ..add('origin', origin)
          ..add('kind', kind)
          ..add('systemCode', systemCode)
          ..add('contentMd', contentMd)
          ..add('contentFormat', contentFormat)
          ..add('contentTruncated', contentTruncated)
          ..add('replyToMessageId', replyToMessageId)
          ..add('runId', runId)
          ..add('triggerState', triggerState)
          ..add('triggerStateAt', triggerStateAt)
          ..add('triggerStateReason', triggerStateReason)
          ..add('silent', silent)
          ..add('attachments', attachments)
          ..add('mentionIds', mentionIds)
          ..add('reactions', reactions)
          ..add('myReactions', myReactions)
          ..add('pinned', pinned)
          ..add('components', components)
          ..add('version', version)
          ..add('editedAt', editedAt)
          ..add('deletedAt', deletedAt)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class MessageBuilder implements Builder<Message, MessageBuilder> {
  _$Message? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  int? _threadSeq;
  int? get threadSeq => _$this._threadSeq;
  set threadSeq(int? threadSeq) => _$this._threadSeq = threadSeq;

  PrincipalRefBuilder? _author;
  PrincipalRefBuilder get author => _$this._author ??= PrincipalRefBuilder();
  set author(PrincipalRefBuilder? author) => _$this._author = author;

  String? _clientMsgId;
  String? get clientMsgId => _$this._clientMsgId;
  set clientMsgId(String? clientMsgId) => _$this._clientMsgId = clientMsgId;

  MessageOrigin? _origin;
  MessageOrigin? get origin => _$this._origin;
  set origin(MessageOrigin? origin) => _$this._origin = origin;

  MessageKind? _kind;
  MessageKind? get kind => _$this._kind;
  set kind(MessageKind? kind) => _$this._kind = kind;

  MessageSystemCodeEnum? _systemCode;
  MessageSystemCodeEnum? get systemCode => _$this._systemCode;
  set systemCode(MessageSystemCodeEnum? systemCode) =>
      _$this._systemCode = systemCode;

  String? _contentMd;
  String? get contentMd => _$this._contentMd;
  set contentMd(String? contentMd) => _$this._contentMd = contentMd;

  int? _contentFormat;
  int? get contentFormat => _$this._contentFormat;
  set contentFormat(int? contentFormat) =>
      _$this._contentFormat = contentFormat;

  bool? _contentTruncated;
  bool? get contentTruncated => _$this._contentTruncated;
  set contentTruncated(bool? contentTruncated) =>
      _$this._contentTruncated = contentTruncated;

  String? _replyToMessageId;
  String? get replyToMessageId => _$this._replyToMessageId;
  set replyToMessageId(String? replyToMessageId) =>
      _$this._replyToMessageId = replyToMessageId;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  TriggerState? _triggerState;
  TriggerState? get triggerState => _$this._triggerState;
  set triggerState(TriggerState? triggerState) =>
      _$this._triggerState = triggerState;

  DateTime? _triggerStateAt;
  DateTime? get triggerStateAt => _$this._triggerStateAt;
  set triggerStateAt(DateTime? triggerStateAt) =>
      _$this._triggerStateAt = triggerStateAt;

  String? _triggerStateReason;
  String? get triggerStateReason => _$this._triggerStateReason;
  set triggerStateReason(String? triggerStateReason) =>
      _$this._triggerStateReason = triggerStateReason;

  bool? _silent;
  bool? get silent => _$this._silent;
  set silent(bool? silent) => _$this._silent = silent;

  ListBuilder<Attachment>? _attachments;
  ListBuilder<Attachment> get attachments =>
      _$this._attachments ??= ListBuilder<Attachment>();
  set attachments(ListBuilder<Attachment>? attachments) =>
      _$this._attachments = attachments;

  ListBuilder<String>? _mentionIds;
  ListBuilder<String> get mentionIds =>
      _$this._mentionIds ??= ListBuilder<String>();
  set mentionIds(ListBuilder<String>? mentionIds) =>
      _$this._mentionIds = mentionIds;

  ListBuilder<ReactionSummary>? _reactions;
  ListBuilder<ReactionSummary> get reactions =>
      _$this._reactions ??= ListBuilder<ReactionSummary>();
  set reactions(ListBuilder<ReactionSummary>? reactions) =>
      _$this._reactions = reactions;

  ListBuilder<String>? _myReactions;
  ListBuilder<String> get myReactions =>
      _$this._myReactions ??= ListBuilder<String>();
  set myReactions(ListBuilder<String>? myReactions) =>
      _$this._myReactions = myReactions;

  bool? _pinned;
  bool? get pinned => _$this._pinned;
  set pinned(bool? pinned) => _$this._pinned = pinned;

  ListBuilder<Component>? _components;
  ListBuilder<Component> get components =>
      _$this._components ??= ListBuilder<Component>();
  set components(ListBuilder<Component>? components) =>
      _$this._components = components;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  DateTime? _editedAt;
  DateTime? get editedAt => _$this._editedAt;
  set editedAt(DateTime? editedAt) => _$this._editedAt = editedAt;

  DateTime? _deletedAt;
  DateTime? get deletedAt => _$this._deletedAt;
  set deletedAt(DateTime? deletedAt) => _$this._deletedAt = deletedAt;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  MessageBuilder() {
    Message._defaults(this);
  }

  MessageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _threadSeq = $v.threadSeq;
      _author = $v.author.toBuilder();
      _clientMsgId = $v.clientMsgId;
      _origin = $v.origin;
      _kind = $v.kind;
      _systemCode = $v.systemCode;
      _contentMd = $v.contentMd;
      _contentFormat = $v.contentFormat;
      _contentTruncated = $v.contentTruncated;
      _replyToMessageId = $v.replyToMessageId;
      _runId = $v.runId;
      _triggerState = $v.triggerState;
      _triggerStateAt = $v.triggerStateAt;
      _triggerStateReason = $v.triggerStateReason;
      _silent = $v.silent;
      _attachments = $v.attachments.toBuilder();
      _mentionIds = $v.mentionIds.toBuilder();
      _reactions = $v.reactions.toBuilder();
      _myReactions = $v.myReactions?.toBuilder();
      _pinned = $v.pinned;
      _components = $v.components.toBuilder();
      _version = $v.version;
      _editedAt = $v.editedAt;
      _deletedAt = $v.deletedAt;
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Message other) {
    _$v = other as _$Message;
  }

  @override
  void update(void Function(MessageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Message build() => _build();

  _$Message _build() {
    _$Message _$result;
    try {
      _$result =
          _$v ??
          _$Message._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'Message', 'id'),
            threadId: BuiltValueNullFieldError.checkNotNull(
              threadId,
              r'Message',
              'threadId',
            ),
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'Message',
              'channelId',
            ),
            threadSeq: BuiltValueNullFieldError.checkNotNull(
              threadSeq,
              r'Message',
              'threadSeq',
            ),
            author: author.build(),
            clientMsgId: BuiltValueNullFieldError.checkNotNull(
              clientMsgId,
              r'Message',
              'clientMsgId',
            ),
            origin: BuiltValueNullFieldError.checkNotNull(
              origin,
              r'Message',
              'origin',
            ),
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'Message',
              'kind',
            ),
            systemCode: systemCode,
            contentMd: BuiltValueNullFieldError.checkNotNull(
              contentMd,
              r'Message',
              'contentMd',
            ),
            contentFormat: BuiltValueNullFieldError.checkNotNull(
              contentFormat,
              r'Message',
              'contentFormat',
            ),
            contentTruncated: contentTruncated,
            replyToMessageId: replyToMessageId,
            runId: runId,
            triggerState: triggerState,
            triggerStateAt: triggerStateAt,
            triggerStateReason: triggerStateReason,
            silent: BuiltValueNullFieldError.checkNotNull(
              silent,
              r'Message',
              'silent',
            ),
            attachments: attachments.build(),
            mentionIds: mentionIds.build(),
            reactions: reactions.build(),
            myReactions: _myReactions?.build(),
            pinned: BuiltValueNullFieldError.checkNotNull(
              pinned,
              r'Message',
              'pinned',
            ),
            components: components.build(),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'Message',
              'version',
            ),
            editedAt: editedAt,
            deletedAt: deletedAt,
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'Message',
              'createdAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'author';
        author.build();

        _$failedField = 'attachments';
        attachments.build();
        _$failedField = 'mentionIds';
        mentionIds.build();
        _$failedField = 'reactions';
        reactions.build();
        _$failedField = 'myReactions';
        _myReactions?.build();

        _$failedField = 'components';
        components.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Message',
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

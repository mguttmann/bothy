// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_context_message.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunContextMessageRoleEnum _$runContextMessageRoleEnum_user =
    const RunContextMessageRoleEnum._('user');
const RunContextMessageRoleEnum _$runContextMessageRoleEnum_assistant =
    const RunContextMessageRoleEnum._('assistant');
const RunContextMessageRoleEnum _$runContextMessageRoleEnum_other =
    const RunContextMessageRoleEnum._('other');
const RunContextMessageRoleEnum
_$runContextMessageRoleEnum_unknownDefaultOpenApi =
    const RunContextMessageRoleEnum._('unknownDefaultOpenApi');

RunContextMessageRoleEnum _$runContextMessageRoleEnumValueOf(String name) {
  switch (name) {
    case 'user':
      return _$runContextMessageRoleEnum_user;
    case 'assistant':
      return _$runContextMessageRoleEnum_assistant;
    case 'other':
      return _$runContextMessageRoleEnum_other;
    case 'unknownDefaultOpenApi':
      return _$runContextMessageRoleEnum_unknownDefaultOpenApi;
    default:
      return _$runContextMessageRoleEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunContextMessageRoleEnum> _$runContextMessageRoleEnumValues =
    BuiltSet<RunContextMessageRoleEnum>(const <RunContextMessageRoleEnum>[
      _$runContextMessageRoleEnum_user,
      _$runContextMessageRoleEnum_assistant,
      _$runContextMessageRoleEnum_other,
      _$runContextMessageRoleEnum_unknownDefaultOpenApi,
    ]);

const RunContextMessageTrustEnum _$runContextMessageTrustEnum_trigger =
    const RunContextMessageTrustEnum._('trigger');
const RunContextMessageTrustEnum _$runContextMessageTrustEnum_authorized =
    const RunContextMessageTrustEnum._('authorized');
const RunContextMessageTrustEnum _$runContextMessageTrustEnum_self =
    const RunContextMessageTrustEnum._('self');
const RunContextMessageTrustEnum _$runContextMessageTrustEnum_unverified =
    const RunContextMessageTrustEnum._('unverified');
const RunContextMessageTrustEnum _$runContextMessageTrustEnum_system =
    const RunContextMessageTrustEnum._('system');
const RunContextMessageTrustEnum
_$runContextMessageTrustEnum_unknownDefaultOpenApi =
    const RunContextMessageTrustEnum._('unknownDefaultOpenApi');

RunContextMessageTrustEnum _$runContextMessageTrustEnumValueOf(String name) {
  switch (name) {
    case 'trigger':
      return _$runContextMessageTrustEnum_trigger;
    case 'authorized':
      return _$runContextMessageTrustEnum_authorized;
    case 'self':
      return _$runContextMessageTrustEnum_self;
    case 'unverified':
      return _$runContextMessageTrustEnum_unverified;
    case 'system':
      return _$runContextMessageTrustEnum_system;
    case 'unknownDefaultOpenApi':
      return _$runContextMessageTrustEnum_unknownDefaultOpenApi;
    default:
      return _$runContextMessageTrustEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunContextMessageTrustEnum> _$runContextMessageTrustEnumValues =
    BuiltSet<RunContextMessageTrustEnum>(const <RunContextMessageTrustEnum>[
      _$runContextMessageTrustEnum_trigger,
      _$runContextMessageTrustEnum_authorized,
      _$runContextMessageTrustEnum_self,
      _$runContextMessageTrustEnum_unverified,
      _$runContextMessageTrustEnum_system,
      _$runContextMessageTrustEnum_unknownDefaultOpenApi,
    ]);

Serializer<RunContextMessageRoleEnum> _$runContextMessageRoleEnumSerializer =
    _$RunContextMessageRoleEnumSerializer();
Serializer<RunContextMessageTrustEnum> _$runContextMessageTrustEnumSerializer =
    _$RunContextMessageTrustEnumSerializer();

class _$RunContextMessageRoleEnumSerializer
    implements PrimitiveSerializer<RunContextMessageRoleEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'user': 'user',
    'assistant': 'assistant',
    'other': 'other',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'user': 'user',
    'assistant': 'assistant',
    'other': 'other',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunContextMessageRoleEnum];
  @override
  final String wireName = 'RunContextMessageRoleEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunContextMessageRoleEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunContextMessageRoleEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunContextMessageRoleEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunContextMessageTrustEnumSerializer
    implements PrimitiveSerializer<RunContextMessageTrustEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'trigger': 'trigger',
    'authorized': 'authorized',
    'self': 'self',
    'unverified': 'unverified',
    'system': 'system',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'trigger': 'trigger',
    'authorized': 'authorized',
    'self': 'self',
    'unverified': 'unverified',
    'system': 'system',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunContextMessageTrustEnum];
  @override
  final String wireName = 'RunContextMessageTrustEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunContextMessageTrustEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunContextMessageTrustEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunContextMessageTrustEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunContextMessage extends RunContextMessage {
  @override
  final String messageId;
  @override
  final int threadSeq;
  @override
  final DateTime createdAt;
  @override
  final PrincipalRef author;
  @override
  final RunContextMessageRoleEnum role;
  @override
  final MessageOrigin origin;
  @override
  final MessageKind kind;
  @override
  final String channelId;
  @override
  final String contentMd;
  @override
  final bool? truncated;
  @override
  final BuiltList<RunContextAttachment> attachments;
  @override
  final String? replyToMessageId;
  @override
  final RunContextMessageTrustEnum trust;
  @override
  final bool? alreadyDelivered;

  factory _$RunContextMessage([
    void Function(RunContextMessageBuilder)? updates,
  ]) => (RunContextMessageBuilder()..update(updates))._build();

  _$RunContextMessage._({
    required this.messageId,
    required this.threadSeq,
    required this.createdAt,
    required this.author,
    required this.role,
    required this.origin,
    required this.kind,
    required this.channelId,
    required this.contentMd,
    this.truncated,
    required this.attachments,
    this.replyToMessageId,
    required this.trust,
    this.alreadyDelivered,
  }) : super._();
  @override
  RunContextMessage rebuild(void Function(RunContextMessageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunContextMessageBuilder toBuilder() =>
      RunContextMessageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunContextMessage &&
        messageId == other.messageId &&
        threadSeq == other.threadSeq &&
        createdAt == other.createdAt &&
        author == other.author &&
        role == other.role &&
        origin == other.origin &&
        kind == other.kind &&
        channelId == other.channelId &&
        contentMd == other.contentMd &&
        truncated == other.truncated &&
        attachments == other.attachments &&
        replyToMessageId == other.replyToMessageId &&
        trust == other.trust &&
        alreadyDelivered == other.alreadyDelivered;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, messageId.hashCode);
    _$hash = $jc(_$hash, threadSeq.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, author.hashCode);
    _$hash = $jc(_$hash, role.hashCode);
    _$hash = $jc(_$hash, origin.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, contentMd.hashCode);
    _$hash = $jc(_$hash, truncated.hashCode);
    _$hash = $jc(_$hash, attachments.hashCode);
    _$hash = $jc(_$hash, replyToMessageId.hashCode);
    _$hash = $jc(_$hash, trust.hashCode);
    _$hash = $jc(_$hash, alreadyDelivered.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunContextMessage')
          ..add('messageId', messageId)
          ..add('threadSeq', threadSeq)
          ..add('createdAt', createdAt)
          ..add('author', author)
          ..add('role', role)
          ..add('origin', origin)
          ..add('kind', kind)
          ..add('channelId', channelId)
          ..add('contentMd', contentMd)
          ..add('truncated', truncated)
          ..add('attachments', attachments)
          ..add('replyToMessageId', replyToMessageId)
          ..add('trust', trust)
          ..add('alreadyDelivered', alreadyDelivered))
        .toString();
  }
}

class RunContextMessageBuilder
    implements Builder<RunContextMessage, RunContextMessageBuilder> {
  _$RunContextMessage? _$v;

  String? _messageId;
  String? get messageId => _$this._messageId;
  set messageId(String? messageId) => _$this._messageId = messageId;

  int? _threadSeq;
  int? get threadSeq => _$this._threadSeq;
  set threadSeq(int? threadSeq) => _$this._threadSeq = threadSeq;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  PrincipalRefBuilder? _author;
  PrincipalRefBuilder get author => _$this._author ??= PrincipalRefBuilder();
  set author(PrincipalRefBuilder? author) => _$this._author = author;

  RunContextMessageRoleEnum? _role;
  RunContextMessageRoleEnum? get role => _$this._role;
  set role(RunContextMessageRoleEnum? role) => _$this._role = role;

  MessageOrigin? _origin;
  MessageOrigin? get origin => _$this._origin;
  set origin(MessageOrigin? origin) => _$this._origin = origin;

  MessageKind? _kind;
  MessageKind? get kind => _$this._kind;
  set kind(MessageKind? kind) => _$this._kind = kind;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _contentMd;
  String? get contentMd => _$this._contentMd;
  set contentMd(String? contentMd) => _$this._contentMd = contentMd;

  bool? _truncated;
  bool? get truncated => _$this._truncated;
  set truncated(bool? truncated) => _$this._truncated = truncated;

  ListBuilder<RunContextAttachment>? _attachments;
  ListBuilder<RunContextAttachment> get attachments =>
      _$this._attachments ??= ListBuilder<RunContextAttachment>();
  set attachments(ListBuilder<RunContextAttachment>? attachments) =>
      _$this._attachments = attachments;

  String? _replyToMessageId;
  String? get replyToMessageId => _$this._replyToMessageId;
  set replyToMessageId(String? replyToMessageId) =>
      _$this._replyToMessageId = replyToMessageId;

  RunContextMessageTrustEnum? _trust;
  RunContextMessageTrustEnum? get trust => _$this._trust;
  set trust(RunContextMessageTrustEnum? trust) => _$this._trust = trust;

  bool? _alreadyDelivered;
  bool? get alreadyDelivered => _$this._alreadyDelivered;
  set alreadyDelivered(bool? alreadyDelivered) =>
      _$this._alreadyDelivered = alreadyDelivered;

  RunContextMessageBuilder() {
    RunContextMessage._defaults(this);
  }

  RunContextMessageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _messageId = $v.messageId;
      _threadSeq = $v.threadSeq;
      _createdAt = $v.createdAt;
      _author = $v.author.toBuilder();
      _role = $v.role;
      _origin = $v.origin;
      _kind = $v.kind;
      _channelId = $v.channelId;
      _contentMd = $v.contentMd;
      _truncated = $v.truncated;
      _attachments = $v.attachments.toBuilder();
      _replyToMessageId = $v.replyToMessageId;
      _trust = $v.trust;
      _alreadyDelivered = $v.alreadyDelivered;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunContextMessage other) {
    _$v = other as _$RunContextMessage;
  }

  @override
  void update(void Function(RunContextMessageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunContextMessage build() => _build();

  _$RunContextMessage _build() {
    _$RunContextMessage _$result;
    try {
      _$result =
          _$v ??
          _$RunContextMessage._(
            messageId: BuiltValueNullFieldError.checkNotNull(
              messageId,
              r'RunContextMessage',
              'messageId',
            ),
            threadSeq: BuiltValueNullFieldError.checkNotNull(
              threadSeq,
              r'RunContextMessage',
              'threadSeq',
            ),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'RunContextMessage',
              'createdAt',
            ),
            author: author.build(),
            role: BuiltValueNullFieldError.checkNotNull(
              role,
              r'RunContextMessage',
              'role',
            ),
            origin: BuiltValueNullFieldError.checkNotNull(
              origin,
              r'RunContextMessage',
              'origin',
            ),
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'RunContextMessage',
              'kind',
            ),
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'RunContextMessage',
              'channelId',
            ),
            contentMd: BuiltValueNullFieldError.checkNotNull(
              contentMd,
              r'RunContextMessage',
              'contentMd',
            ),
            truncated: truncated,
            attachments: attachments.build(),
            replyToMessageId: replyToMessageId,
            trust: BuiltValueNullFieldError.checkNotNull(
              trust,
              r'RunContextMessage',
              'trust',
            ),
            alreadyDelivered: alreadyDelivered,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'author';
        author.build();

        _$failedField = 'attachments';
        attachments.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunContextMessage',
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

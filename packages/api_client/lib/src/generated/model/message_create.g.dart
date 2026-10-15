// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'message_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const MessageCreateKindEnum _$messageCreateKindEnum_text =
    const MessageCreateKindEnum._('text');
const MessageCreateKindEnum _$messageCreateKindEnum_voice =
    const MessageCreateKindEnum._('voice');
const MessageCreateKindEnum _$messageCreateKindEnum_card =
    const MessageCreateKindEnum._('card');
const MessageCreateKindEnum _$messageCreateKindEnum_unknownDefaultOpenApi =
    const MessageCreateKindEnum._('unknownDefaultOpenApi');

MessageCreateKindEnum _$messageCreateKindEnumValueOf(String name) {
  switch (name) {
    case 'text':
      return _$messageCreateKindEnum_text;
    case 'voice':
      return _$messageCreateKindEnum_voice;
    case 'card':
      return _$messageCreateKindEnum_card;
    case 'unknownDefaultOpenApi':
      return _$messageCreateKindEnum_unknownDefaultOpenApi;
    default:
      return _$messageCreateKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<MessageCreateKindEnum> _$messageCreateKindEnumValues =
    BuiltSet<MessageCreateKindEnum>(const <MessageCreateKindEnum>[
      _$messageCreateKindEnum_text,
      _$messageCreateKindEnum_voice,
      _$messageCreateKindEnum_card,
      _$messageCreateKindEnum_unknownDefaultOpenApi,
    ]);

Serializer<MessageCreateKindEnum> _$messageCreateKindEnumSerializer =
    _$MessageCreateKindEnumSerializer();

class _$MessageCreateKindEnumSerializer
    implements PrimitiveSerializer<MessageCreateKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'text': 'text',
    'voice': 'voice',
    'card': 'card',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'text': 'text',
    'voice': 'voice',
    'card': 'card',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[MessageCreateKindEnum];
  @override
  final String wireName = 'MessageCreateKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    MessageCreateKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  MessageCreateKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => MessageCreateKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$MessageCreate extends MessageCreate {
  @override
  final String clientMsgId;
  @override
  final MessageCreateKindEnum? kind;
  @override
  final String? contentMd;
  @override
  final String? replyToMessageId;
  @override
  final BuiltList<String>? attachmentFileIds;
  @override
  final BuiltList<String>? mentionIds;
  @override
  final bool? silent;
  @override
  final bool? clearDraft;
  @override
  final String? runId;
  @override
  final BuiltList<ComponentCreate>? components;

  factory _$MessageCreate([void Function(MessageCreateBuilder)? updates]) =>
      (MessageCreateBuilder()..update(updates))._build();

  _$MessageCreate._({
    required this.clientMsgId,
    this.kind,
    this.contentMd,
    this.replyToMessageId,
    this.attachmentFileIds,
    this.mentionIds,
    this.silent,
    this.clearDraft,
    this.runId,
    this.components,
  }) : super._();
  @override
  MessageCreate rebuild(void Function(MessageCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  MessageCreateBuilder toBuilder() => MessageCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is MessageCreate &&
        clientMsgId == other.clientMsgId &&
        kind == other.kind &&
        contentMd == other.contentMd &&
        replyToMessageId == other.replyToMessageId &&
        attachmentFileIds == other.attachmentFileIds &&
        mentionIds == other.mentionIds &&
        silent == other.silent &&
        clearDraft == other.clearDraft &&
        runId == other.runId &&
        components == other.components;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, clientMsgId.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, contentMd.hashCode);
    _$hash = $jc(_$hash, replyToMessageId.hashCode);
    _$hash = $jc(_$hash, attachmentFileIds.hashCode);
    _$hash = $jc(_$hash, mentionIds.hashCode);
    _$hash = $jc(_$hash, silent.hashCode);
    _$hash = $jc(_$hash, clearDraft.hashCode);
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, components.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'MessageCreate')
          ..add('clientMsgId', clientMsgId)
          ..add('kind', kind)
          ..add('contentMd', contentMd)
          ..add('replyToMessageId', replyToMessageId)
          ..add('attachmentFileIds', attachmentFileIds)
          ..add('mentionIds', mentionIds)
          ..add('silent', silent)
          ..add('clearDraft', clearDraft)
          ..add('runId', runId)
          ..add('components', components))
        .toString();
  }
}

class MessageCreateBuilder
    implements Builder<MessageCreate, MessageCreateBuilder> {
  _$MessageCreate? _$v;

  String? _clientMsgId;
  String? get clientMsgId => _$this._clientMsgId;
  set clientMsgId(String? clientMsgId) => _$this._clientMsgId = clientMsgId;

  MessageCreateKindEnum? _kind;
  MessageCreateKindEnum? get kind => _$this._kind;
  set kind(MessageCreateKindEnum? kind) => _$this._kind = kind;

  String? _contentMd;
  String? get contentMd => _$this._contentMd;
  set contentMd(String? contentMd) => _$this._contentMd = contentMd;

  String? _replyToMessageId;
  String? get replyToMessageId => _$this._replyToMessageId;
  set replyToMessageId(String? replyToMessageId) =>
      _$this._replyToMessageId = replyToMessageId;

  ListBuilder<String>? _attachmentFileIds;
  ListBuilder<String> get attachmentFileIds =>
      _$this._attachmentFileIds ??= ListBuilder<String>();
  set attachmentFileIds(ListBuilder<String>? attachmentFileIds) =>
      _$this._attachmentFileIds = attachmentFileIds;

  ListBuilder<String>? _mentionIds;
  ListBuilder<String> get mentionIds =>
      _$this._mentionIds ??= ListBuilder<String>();
  set mentionIds(ListBuilder<String>? mentionIds) =>
      _$this._mentionIds = mentionIds;

  bool? _silent;
  bool? get silent => _$this._silent;
  set silent(bool? silent) => _$this._silent = silent;

  bool? _clearDraft;
  bool? get clearDraft => _$this._clearDraft;
  set clearDraft(bool? clearDraft) => _$this._clearDraft = clearDraft;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  ListBuilder<ComponentCreate>? _components;
  ListBuilder<ComponentCreate> get components =>
      _$this._components ??= ListBuilder<ComponentCreate>();
  set components(ListBuilder<ComponentCreate>? components) =>
      _$this._components = components;

  MessageCreateBuilder() {
    MessageCreate._defaults(this);
  }

  MessageCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _clientMsgId = $v.clientMsgId;
      _kind = $v.kind;
      _contentMd = $v.contentMd;
      _replyToMessageId = $v.replyToMessageId;
      _attachmentFileIds = $v.attachmentFileIds?.toBuilder();
      _mentionIds = $v.mentionIds?.toBuilder();
      _silent = $v.silent;
      _clearDraft = $v.clearDraft;
      _runId = $v.runId;
      _components = $v.components?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(MessageCreate other) {
    _$v = other as _$MessageCreate;
  }

  @override
  void update(void Function(MessageCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  MessageCreate build() => _build();

  _$MessageCreate _build() {
    _$MessageCreate _$result;
    try {
      _$result =
          _$v ??
          _$MessageCreate._(
            clientMsgId: BuiltValueNullFieldError.checkNotNull(
              clientMsgId,
              r'MessageCreate',
              'clientMsgId',
            ),
            kind: kind,
            contentMd: contentMd,
            replyToMessageId: replyToMessageId,
            attachmentFileIds: _attachmentFileIds?.build(),
            mentionIds: _mentionIds?.build(),
            silent: silent,
            clearDraft: clearDraft,
            runId: runId,
            components: _components?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'attachmentFileIds';
        _attachmentFileIds?.build();
        _$failedField = 'mentionIds';
        _mentionIds?.build();

        _$failedField = 'components';
        _components?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'MessageCreate',
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

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'message_deleted_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$MessageDeletedPayload extends MessageDeletedPayload {
  @override
  final String messageId;
  @override
  final String threadId;
  @override
  final int threadSeq;
  @override
  final DateTime deletedAt;
  @override
  final String? deletedBy;

  factory _$MessageDeletedPayload([
    void Function(MessageDeletedPayloadBuilder)? updates,
  ]) => (MessageDeletedPayloadBuilder()..update(updates))._build();

  _$MessageDeletedPayload._({
    required this.messageId,
    required this.threadId,
    required this.threadSeq,
    required this.deletedAt,
    this.deletedBy,
  }) : super._();
  @override
  MessageDeletedPayload rebuild(
    void Function(MessageDeletedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  MessageDeletedPayloadBuilder toBuilder() =>
      MessageDeletedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is MessageDeletedPayload &&
        messageId == other.messageId &&
        threadId == other.threadId &&
        threadSeq == other.threadSeq &&
        deletedAt == other.deletedAt &&
        deletedBy == other.deletedBy;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, messageId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, threadSeq.hashCode);
    _$hash = $jc(_$hash, deletedAt.hashCode);
    _$hash = $jc(_$hash, deletedBy.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'MessageDeletedPayload')
          ..add('messageId', messageId)
          ..add('threadId', threadId)
          ..add('threadSeq', threadSeq)
          ..add('deletedAt', deletedAt)
          ..add('deletedBy', deletedBy))
        .toString();
  }
}

class MessageDeletedPayloadBuilder
    implements Builder<MessageDeletedPayload, MessageDeletedPayloadBuilder> {
  _$MessageDeletedPayload? _$v;

  String? _messageId;
  String? get messageId => _$this._messageId;
  set messageId(String? messageId) => _$this._messageId = messageId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  int? _threadSeq;
  int? get threadSeq => _$this._threadSeq;
  set threadSeq(int? threadSeq) => _$this._threadSeq = threadSeq;

  DateTime? _deletedAt;
  DateTime? get deletedAt => _$this._deletedAt;
  set deletedAt(DateTime? deletedAt) => _$this._deletedAt = deletedAt;

  String? _deletedBy;
  String? get deletedBy => _$this._deletedBy;
  set deletedBy(String? deletedBy) => _$this._deletedBy = deletedBy;

  MessageDeletedPayloadBuilder() {
    MessageDeletedPayload._defaults(this);
  }

  MessageDeletedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _messageId = $v.messageId;
      _threadId = $v.threadId;
      _threadSeq = $v.threadSeq;
      _deletedAt = $v.deletedAt;
      _deletedBy = $v.deletedBy;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(MessageDeletedPayload other) {
    _$v = other as _$MessageDeletedPayload;
  }

  @override
  void update(void Function(MessageDeletedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  MessageDeletedPayload build() => _build();

  _$MessageDeletedPayload _build() {
    final _$result =
        _$v ??
        _$MessageDeletedPayload._(
          messageId: BuiltValueNullFieldError.checkNotNull(
            messageId,
            r'MessageDeletedPayload',
            'messageId',
          ),
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'MessageDeletedPayload',
            'threadId',
          ),
          threadSeq: BuiltValueNullFieldError.checkNotNull(
            threadSeq,
            r'MessageDeletedPayload',
            'threadSeq',
          ),
          deletedAt: BuiltValueNullFieldError.checkNotNull(
            deletedAt,
            r'MessageDeletedPayload',
            'deletedAt',
          ),
          deletedBy: deletedBy,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

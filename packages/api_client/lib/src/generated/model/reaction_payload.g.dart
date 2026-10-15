// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'reaction_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ReactionPayload extends ReactionPayload {
  @override
  final String messageId;
  @override
  final String threadId;
  @override
  final String emoji;
  @override
  final String principalId;
  @override
  final int count;

  factory _$ReactionPayload([void Function(ReactionPayloadBuilder)? updates]) =>
      (ReactionPayloadBuilder()..update(updates))._build();

  _$ReactionPayload._({
    required this.messageId,
    required this.threadId,
    required this.emoji,
    required this.principalId,
    required this.count,
  }) : super._();
  @override
  ReactionPayload rebuild(void Function(ReactionPayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ReactionPayloadBuilder toBuilder() => ReactionPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ReactionPayload &&
        messageId == other.messageId &&
        threadId == other.threadId &&
        emoji == other.emoji &&
        principalId == other.principalId &&
        count == other.count;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, messageId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, emoji.hashCode);
    _$hash = $jc(_$hash, principalId.hashCode);
    _$hash = $jc(_$hash, count.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ReactionPayload')
          ..add('messageId', messageId)
          ..add('threadId', threadId)
          ..add('emoji', emoji)
          ..add('principalId', principalId)
          ..add('count', count))
        .toString();
  }
}

class ReactionPayloadBuilder
    implements Builder<ReactionPayload, ReactionPayloadBuilder> {
  _$ReactionPayload? _$v;

  String? _messageId;
  String? get messageId => _$this._messageId;
  set messageId(String? messageId) => _$this._messageId = messageId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _emoji;
  String? get emoji => _$this._emoji;
  set emoji(String? emoji) => _$this._emoji = emoji;

  String? _principalId;
  String? get principalId => _$this._principalId;
  set principalId(String? principalId) => _$this._principalId = principalId;

  int? _count;
  int? get count => _$this._count;
  set count(int? count) => _$this._count = count;

  ReactionPayloadBuilder() {
    ReactionPayload._defaults(this);
  }

  ReactionPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _messageId = $v.messageId;
      _threadId = $v.threadId;
      _emoji = $v.emoji;
      _principalId = $v.principalId;
      _count = $v.count;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ReactionPayload other) {
    _$v = other as _$ReactionPayload;
  }

  @override
  void update(void Function(ReactionPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ReactionPayload build() => _build();

  _$ReactionPayload _build() {
    final _$result =
        _$v ??
        _$ReactionPayload._(
          messageId: BuiltValueNullFieldError.checkNotNull(
            messageId,
            r'ReactionPayload',
            'messageId',
          ),
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'ReactionPayload',
            'threadId',
          ),
          emoji: BuiltValueNullFieldError.checkNotNull(
            emoji,
            r'ReactionPayload',
            'emoji',
          ),
          principalId: BuiltValueNullFieldError.checkNotNull(
            principalId,
            r'ReactionPayload',
            'principalId',
          ),
          count: BuiltValueNullFieldError.checkNotNull(
            count,
            r'ReactionPayload',
            'count',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'incoming_webhook_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$IncomingWebhookResult extends IncomingWebhookResult {
  @override
  final String messageId;
  @override
  final String threadId;

  factory _$IncomingWebhookResult([
    void Function(IncomingWebhookResultBuilder)? updates,
  ]) => (IncomingWebhookResultBuilder()..update(updates))._build();

  _$IncomingWebhookResult._({required this.messageId, required this.threadId})
    : super._();
  @override
  IncomingWebhookResult rebuild(
    void Function(IncomingWebhookResultBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  IncomingWebhookResultBuilder toBuilder() =>
      IncomingWebhookResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IncomingWebhookResult &&
        messageId == other.messageId &&
        threadId == other.threadId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, messageId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'IncomingWebhookResult')
          ..add('messageId', messageId)
          ..add('threadId', threadId))
        .toString();
  }
}

class IncomingWebhookResultBuilder
    implements Builder<IncomingWebhookResult, IncomingWebhookResultBuilder> {
  _$IncomingWebhookResult? _$v;

  String? _messageId;
  String? get messageId => _$this._messageId;
  set messageId(String? messageId) => _$this._messageId = messageId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  IncomingWebhookResultBuilder() {
    IncomingWebhookResult._defaults(this);
  }

  IncomingWebhookResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _messageId = $v.messageId;
      _threadId = $v.threadId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IncomingWebhookResult other) {
    _$v = other as _$IncomingWebhookResult;
  }

  @override
  void update(void Function(IncomingWebhookResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IncomingWebhookResult build() => _build();

  _$IncomingWebhookResult _build() {
    final _$result =
        _$v ??
        _$IncomingWebhookResult._(
          messageId: BuiltValueNullFieldError.checkNotNull(
            messageId,
            r'IncomingWebhookResult',
            'messageId',
          ),
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'IncomingWebhookResult',
            'threadId',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

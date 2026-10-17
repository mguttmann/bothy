// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'message_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$MessagePayload extends MessagePayload {
  @override
  final Message message;

  factory _$MessagePayload([void Function(MessagePayloadBuilder)? updates]) =>
      (MessagePayloadBuilder()..update(updates))._build();

  _$MessagePayload._({required this.message}) : super._();
  @override
  MessagePayload rebuild(void Function(MessagePayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  MessagePayloadBuilder toBuilder() => MessagePayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is MessagePayload && message == other.message;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, message.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'MessagePayload',
    )..add('message', message)).toString();
  }
}

class MessagePayloadBuilder
    implements Builder<MessagePayload, MessagePayloadBuilder> {
  _$MessagePayload? _$v;

  MessageBuilder? _message;
  MessageBuilder get message => _$this._message ??= MessageBuilder();
  set message(MessageBuilder? message) => _$this._message = message;

  MessagePayloadBuilder() {
    MessagePayload._defaults(this);
  }

  MessagePayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _message = $v.message.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(MessagePayload other) {
    _$v = other as _$MessagePayload;
  }

  @override
  void update(void Function(MessagePayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  MessagePayload build() => _build();

  _$MessagePayload _build() {
    _$MessagePayload _$result;
    try {
      _$result = _$v ?? _$MessagePayload._(message: message.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'message';
        message.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'MessagePayload',
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

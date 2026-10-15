// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ChannelPayload extends ChannelPayload {
  @override
  final Channel channel;

  factory _$ChannelPayload([void Function(ChannelPayloadBuilder)? updates]) =>
      (ChannelPayloadBuilder()..update(updates))._build();

  _$ChannelPayload._({required this.channel}) : super._();
  @override
  ChannelPayload rebuild(void Function(ChannelPayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ChannelPayloadBuilder toBuilder() => ChannelPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelPayload && channel == other.channel;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channel.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ChannelPayload',
    )..add('channel', channel)).toString();
  }
}

class ChannelPayloadBuilder
    implements Builder<ChannelPayload, ChannelPayloadBuilder> {
  _$ChannelPayload? _$v;

  ChannelBuilder? _channel;
  ChannelBuilder get channel => _$this._channel ??= ChannelBuilder();
  set channel(ChannelBuilder? channel) => _$this._channel = channel;

  ChannelPayloadBuilder() {
    ChannelPayload._defaults(this);
  }

  ChannelPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channel = $v.channel.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelPayload other) {
    _$v = other as _$ChannelPayload;
  }

  @override
  void update(void Function(ChannelPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelPayload build() => _build();

  _$ChannelPayload _build() {
    _$ChannelPayload _$result;
    try {
      _$result = _$v ?? _$ChannelPayload._(channel: channel.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'channel';
        channel.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ChannelPayload',
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

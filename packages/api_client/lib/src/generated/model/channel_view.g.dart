// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_view.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ChannelView extends ChannelView {
  @override
  final Channel channel;
  @override
  final Level level;
  @override
  final BuiltList<Capability> capabilities;

  factory _$ChannelView([void Function(ChannelViewBuilder)? updates]) =>
      (ChannelViewBuilder()..update(updates))._build();

  _$ChannelView._({
    required this.channel,
    required this.level,
    required this.capabilities,
  }) : super._();
  @override
  ChannelView rebuild(void Function(ChannelViewBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ChannelViewBuilder toBuilder() => ChannelViewBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelView &&
        channel == other.channel &&
        level == other.level &&
        capabilities == other.capabilities;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channel.hashCode);
    _$hash = $jc(_$hash, level.hashCode);
    _$hash = $jc(_$hash, capabilities.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ChannelView')
          ..add('channel', channel)
          ..add('level', level)
          ..add('capabilities', capabilities))
        .toString();
  }
}

class ChannelViewBuilder implements Builder<ChannelView, ChannelViewBuilder> {
  _$ChannelView? _$v;

  ChannelBuilder? _channel;
  ChannelBuilder get channel => _$this._channel ??= ChannelBuilder();
  set channel(ChannelBuilder? channel) => _$this._channel = channel;

  Level? _level;
  Level? get level => _$this._level;
  set level(Level? level) => _$this._level = level;

  ListBuilder<Capability>? _capabilities;
  ListBuilder<Capability> get capabilities =>
      _$this._capabilities ??= ListBuilder<Capability>();
  set capabilities(ListBuilder<Capability>? capabilities) =>
      _$this._capabilities = capabilities;

  ChannelViewBuilder() {
    ChannelView._defaults(this);
  }

  ChannelViewBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channel = $v.channel.toBuilder();
      _level = $v.level;
      _capabilities = $v.capabilities.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelView other) {
    _$v = other as _$ChannelView;
  }

  @override
  void update(void Function(ChannelViewBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelView build() => _build();

  _$ChannelView _build() {
    _$ChannelView _$result;
    try {
      _$result =
          _$v ??
          _$ChannelView._(
            channel: channel.build(),
            level: BuiltValueNullFieldError.checkNotNull(
              level,
              r'ChannelView',
              'level',
            ),
            capabilities: capabilities.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'channel';
        channel.build();

        _$failedField = 'capabilities';
        capabilities.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ChannelView',
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

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ChannelUpdate extends ChannelUpdate {
  @override
  final String? name;
  @override
  final String? topic;
  @override
  final ForumSort? forumDefaultSort;
  @override
  final NotificationLevel? defaultNotificationLevel;
  @override
  final bool? clearDefaultNotificationLevel;

  factory _$ChannelUpdate([void Function(ChannelUpdateBuilder)? updates]) =>
      (ChannelUpdateBuilder()..update(updates))._build();

  _$ChannelUpdate._({
    this.name,
    this.topic,
    this.forumDefaultSort,
    this.defaultNotificationLevel,
    this.clearDefaultNotificationLevel,
  }) : super._();
  @override
  ChannelUpdate rebuild(void Function(ChannelUpdateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ChannelUpdateBuilder toBuilder() => ChannelUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelUpdate &&
        name == other.name &&
        topic == other.topic &&
        forumDefaultSort == other.forumDefaultSort &&
        defaultNotificationLevel == other.defaultNotificationLevel &&
        clearDefaultNotificationLevel == other.clearDefaultNotificationLevel;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, topic.hashCode);
    _$hash = $jc(_$hash, forumDefaultSort.hashCode);
    _$hash = $jc(_$hash, defaultNotificationLevel.hashCode);
    _$hash = $jc(_$hash, clearDefaultNotificationLevel.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ChannelUpdate')
          ..add('name', name)
          ..add('topic', topic)
          ..add('forumDefaultSort', forumDefaultSort)
          ..add('defaultNotificationLevel', defaultNotificationLevel)
          ..add('clearDefaultNotificationLevel', clearDefaultNotificationLevel))
        .toString();
  }
}

class ChannelUpdateBuilder
    implements Builder<ChannelUpdate, ChannelUpdateBuilder> {
  _$ChannelUpdate? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _topic;
  String? get topic => _$this._topic;
  set topic(String? topic) => _$this._topic = topic;

  ForumSort? _forumDefaultSort;
  ForumSort? get forumDefaultSort => _$this._forumDefaultSort;
  set forumDefaultSort(ForumSort? forumDefaultSort) =>
      _$this._forumDefaultSort = forumDefaultSort;

  NotificationLevel? _defaultNotificationLevel;
  NotificationLevel? get defaultNotificationLevel =>
      _$this._defaultNotificationLevel;
  set defaultNotificationLevel(NotificationLevel? defaultNotificationLevel) =>
      _$this._defaultNotificationLevel = defaultNotificationLevel;

  bool? _clearDefaultNotificationLevel;
  bool? get clearDefaultNotificationLevel =>
      _$this._clearDefaultNotificationLevel;
  set clearDefaultNotificationLevel(bool? clearDefaultNotificationLevel) =>
      _$this._clearDefaultNotificationLevel = clearDefaultNotificationLevel;

  ChannelUpdateBuilder() {
    ChannelUpdate._defaults(this);
  }

  ChannelUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _topic = $v.topic;
      _forumDefaultSort = $v.forumDefaultSort;
      _defaultNotificationLevel = $v.defaultNotificationLevel;
      _clearDefaultNotificationLevel = $v.clearDefaultNotificationLevel;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelUpdate other) {
    _$v = other as _$ChannelUpdate;
  }

  @override
  void update(void Function(ChannelUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelUpdate build() => _build();

  _$ChannelUpdate _build() {
    final _$result =
        _$v ??
        _$ChannelUpdate._(
          name: name,
          topic: topic,
          forumDefaultSort: forumDefaultSort,
          defaultNotificationLevel: defaultNotificationLevel,
          clearDefaultNotificationLevel: clearDefaultNotificationLevel,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

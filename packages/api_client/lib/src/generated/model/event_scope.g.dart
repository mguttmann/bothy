// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'event_scope.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$EventScope extends EventScope {
  @override
  final String? spaceId;
  @override
  final String? channelId;
  @override
  final String? threadId;

  factory _$EventScope([void Function(EventScopeBuilder)? updates]) =>
      (EventScopeBuilder()..update(updates))._build();

  _$EventScope._({this.spaceId, this.channelId, this.threadId}) : super._();
  @override
  EventScope rebuild(void Function(EventScopeBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  EventScopeBuilder toBuilder() => EventScopeBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is EventScope &&
        spaceId == other.spaceId &&
        channelId == other.channelId &&
        threadId == other.threadId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, spaceId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'EventScope')
          ..add('spaceId', spaceId)
          ..add('channelId', channelId)
          ..add('threadId', threadId))
        .toString();
  }
}

class EventScopeBuilder implements Builder<EventScope, EventScopeBuilder> {
  _$EventScope? _$v;

  String? _spaceId;
  String? get spaceId => _$this._spaceId;
  set spaceId(String? spaceId) => _$this._spaceId = spaceId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  EventScopeBuilder() {
    EventScope._defaults(this);
  }

  EventScopeBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _spaceId = $v.spaceId;
      _channelId = $v.channelId;
      _threadId = $v.threadId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(EventScope other) {
    _$v = other as _$EventScope;
  }

  @override
  void update(void Function(EventScopeBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  EventScope build() => _build();

  _$EventScope _build() {
    final _$result =
        _$v ??
        _$EventScope._(
          spaceId: spaceId,
          channelId: channelId,
          threadId: threadId,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

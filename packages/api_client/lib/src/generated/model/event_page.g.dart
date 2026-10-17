// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'event_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$EventPage extends EventPage {
  @override
  final BuiltList<Event> events;
  @override
  final int nextCursor;
  @override
  final int headSeq;
  @override
  final String syncEpoch;

  factory _$EventPage([void Function(EventPageBuilder)? updates]) =>
      (EventPageBuilder()..update(updates))._build();

  _$EventPage._({
    required this.events,
    required this.nextCursor,
    required this.headSeq,
    required this.syncEpoch,
  }) : super._();
  @override
  EventPage rebuild(void Function(EventPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  EventPageBuilder toBuilder() => EventPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is EventPage &&
        events == other.events &&
        nextCursor == other.nextCursor &&
        headSeq == other.headSeq &&
        syncEpoch == other.syncEpoch;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, events.hashCode);
    _$hash = $jc(_$hash, nextCursor.hashCode);
    _$hash = $jc(_$hash, headSeq.hashCode);
    _$hash = $jc(_$hash, syncEpoch.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'EventPage')
          ..add('events', events)
          ..add('nextCursor', nextCursor)
          ..add('headSeq', headSeq)
          ..add('syncEpoch', syncEpoch))
        .toString();
  }
}

class EventPageBuilder implements Builder<EventPage, EventPageBuilder> {
  _$EventPage? _$v;

  ListBuilder<Event>? _events;
  ListBuilder<Event> get events => _$this._events ??= ListBuilder<Event>();
  set events(ListBuilder<Event>? events) => _$this._events = events;

  int? _nextCursor;
  int? get nextCursor => _$this._nextCursor;
  set nextCursor(int? nextCursor) => _$this._nextCursor = nextCursor;

  int? _headSeq;
  int? get headSeq => _$this._headSeq;
  set headSeq(int? headSeq) => _$this._headSeq = headSeq;

  String? _syncEpoch;
  String? get syncEpoch => _$this._syncEpoch;
  set syncEpoch(String? syncEpoch) => _$this._syncEpoch = syncEpoch;

  EventPageBuilder() {
    EventPage._defaults(this);
  }

  EventPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _events = $v.events.toBuilder();
      _nextCursor = $v.nextCursor;
      _headSeq = $v.headSeq;
      _syncEpoch = $v.syncEpoch;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(EventPage other) {
    _$v = other as _$EventPage;
  }

  @override
  void update(void Function(EventPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  EventPage build() => _build();

  _$EventPage _build() {
    _$EventPage _$result;
    try {
      _$result =
          _$v ??
          _$EventPage._(
            events: events.build(),
            nextCursor: BuiltValueNullFieldError.checkNotNull(
              nextCursor,
              r'EventPage',
              'nextCursor',
            ),
            headSeq: BuiltValueNullFieldError.checkNotNull(
              headSeq,
              r'EventPage',
              'headSeq',
            ),
            syncEpoch: BuiltValueNullFieldError.checkNotNull(
              syncEpoch,
              r'EventPage',
              'syncEpoch',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'events';
        events.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'EventPage',
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

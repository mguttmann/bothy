// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'event_actor.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$EventActor extends EventActor {
  @override
  final String? id;
  @override
  final ActorKind kind;

  factory _$EventActor([void Function(EventActorBuilder)? updates]) =>
      (EventActorBuilder()..update(updates))._build();

  _$EventActor._({this.id, required this.kind}) : super._();
  @override
  EventActor rebuild(void Function(EventActorBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  EventActorBuilder toBuilder() => EventActorBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is EventActor && id == other.id && kind == other.kind;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'EventActor')
          ..add('id', id)
          ..add('kind', kind))
        .toString();
  }
}

class EventActorBuilder implements Builder<EventActor, EventActorBuilder> {
  _$EventActor? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  ActorKind? _kind;
  ActorKind? get kind => _$this._kind;
  set kind(ActorKind? kind) => _$this._kind = kind;

  EventActorBuilder() {
    EventActor._defaults(this);
  }

  EventActorBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _kind = $v.kind;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(EventActor other) {
    _$v = other as _$EventActor;
  }

  @override
  void update(void Function(EventActorBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  EventActor build() => _build();

  _$EventActor _build() {
    final _$result =
        _$v ??
        _$EventActor._(
          id: id,
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'EventActor',
            'kind',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'event_resource.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$EventResource extends EventResource {
  @override
  final String kind;
  @override
  final String? id;
  @override
  final int? version;

  factory _$EventResource([void Function(EventResourceBuilder)? updates]) =>
      (EventResourceBuilder()..update(updates))._build();

  _$EventResource._({required this.kind, this.id, this.version}) : super._();
  @override
  EventResource rebuild(void Function(EventResourceBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  EventResourceBuilder toBuilder() => EventResourceBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is EventResource &&
        kind == other.kind &&
        id == other.id &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'EventResource')
          ..add('kind', kind)
          ..add('id', id)
          ..add('version', version))
        .toString();
  }
}

class EventResourceBuilder
    implements Builder<EventResource, EventResourceBuilder> {
  _$EventResource? _$v;

  String? _kind;
  String? get kind => _$this._kind;
  set kind(String? kind) => _$this._kind = kind;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  EventResourceBuilder() {
    EventResource._defaults(this);
  }

  EventResourceBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _id = $v.id;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(EventResource other) {
    _$v = other as _$EventResource;
  }

  @override
  void update(void Function(EventResourceBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  EventResource build() => _build();

  _$EventResource _build() {
    final _$result =
        _$v ??
        _$EventResource._(
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'EventResource',
            'kind',
          ),
          id: id,
          version: version,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

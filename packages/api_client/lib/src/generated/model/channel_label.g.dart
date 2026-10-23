// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_label.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ChannelLabel extends ChannelLabel {
  @override
  final String id;
  @override
  final String spaceId;
  @override
  final String name;

  factory _$ChannelLabel([void Function(ChannelLabelBuilder)? updates]) =>
      (ChannelLabelBuilder()..update(updates))._build();

  _$ChannelLabel._({
    required this.id,
    required this.spaceId,
    required this.name,
  }) : super._();
  @override
  ChannelLabel rebuild(void Function(ChannelLabelBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ChannelLabelBuilder toBuilder() => ChannelLabelBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelLabel &&
        id == other.id &&
        spaceId == other.spaceId &&
        name == other.name;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, spaceId.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ChannelLabel')
          ..add('id', id)
          ..add('spaceId', spaceId)
          ..add('name', name))
        .toString();
  }
}

class ChannelLabelBuilder
    implements Builder<ChannelLabel, ChannelLabelBuilder> {
  _$ChannelLabel? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _spaceId;
  String? get spaceId => _$this._spaceId;
  set spaceId(String? spaceId) => _$this._spaceId = spaceId;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  ChannelLabelBuilder() {
    ChannelLabel._defaults(this);
  }

  ChannelLabelBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _spaceId = $v.spaceId;
      _name = $v.name;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelLabel other) {
    _$v = other as _$ChannelLabel;
  }

  @override
  void update(void Function(ChannelLabelBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelLabel build() => _build();

  _$ChannelLabel _build() {
    final _$result =
        _$v ??
        _$ChannelLabel._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'ChannelLabel', 'id'),
          spaceId: BuiltValueNullFieldError.checkNotNull(
            spaceId,
            r'ChannelLabel',
            'spaceId',
          ),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'ChannelLabel',
            'name',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

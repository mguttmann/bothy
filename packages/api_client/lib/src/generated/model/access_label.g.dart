// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'access_label.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AccessLabel extends AccessLabel {
  @override
  final String id;
  @override
  final String name;

  factory _$AccessLabel([void Function(AccessLabelBuilder)? updates]) =>
      (AccessLabelBuilder()..update(updates))._build();

  _$AccessLabel._({required this.id, required this.name}) : super._();
  @override
  AccessLabel rebuild(void Function(AccessLabelBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AccessLabelBuilder toBuilder() => AccessLabelBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AccessLabel && id == other.id && name == other.name;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AccessLabel')
          ..add('id', id)
          ..add('name', name))
        .toString();
  }
}

class AccessLabelBuilder implements Builder<AccessLabel, AccessLabelBuilder> {
  _$AccessLabel? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  AccessLabelBuilder() {
    AccessLabel._defaults(this);
  }

  AccessLabelBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _name = $v.name;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AccessLabel other) {
    _$v = other as _$AccessLabel;
  }

  @override
  void update(void Function(AccessLabelBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AccessLabel build() => _build();

  _$AccessLabel _build() {
    final _$result =
        _$v ??
        _$AccessLabel._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'AccessLabel', 'id'),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'AccessLabel',
            'name',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

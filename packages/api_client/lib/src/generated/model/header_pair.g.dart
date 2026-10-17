// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'header_pair.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HeaderPair extends HeaderPair {
  @override
  final String name;
  @override
  final String value;

  factory _$HeaderPair([void Function(HeaderPairBuilder)? updates]) =>
      (HeaderPairBuilder()..update(updates))._build();

  _$HeaderPair._({required this.name, required this.value}) : super._();
  @override
  HeaderPair rebuild(void Function(HeaderPairBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HeaderPairBuilder toBuilder() => HeaderPairBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HeaderPair && name == other.name && value == other.value;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, value.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HeaderPair')
          ..add('name', name)
          ..add('value', value))
        .toString();
  }
}

class HeaderPairBuilder implements Builder<HeaderPair, HeaderPairBuilder> {
  _$HeaderPair? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _value;
  String? get value => _$this._value;
  set value(String? value) => _$this._value = value;

  HeaderPairBuilder() {
    HeaderPair._defaults(this);
  }

  HeaderPairBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _value = $v.value;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HeaderPair other) {
    _$v = other as _$HeaderPair;
  }

  @override
  void update(void Function(HeaderPairBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HeaderPair build() => _build();

  _$HeaderPair _build() {
    final _$result =
        _$v ??
        _$HeaderPair._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'HeaderPair',
            'name',
          ),
          value: BuiltValueNullFieldError.checkNotNull(
            value,
            r'HeaderPair',
            'value',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

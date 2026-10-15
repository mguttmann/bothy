// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'template_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TemplateCreate extends TemplateCreate {
  @override
  final String name;
  @override
  final String contentMd;

  factory _$TemplateCreate([void Function(TemplateCreateBuilder)? updates]) =>
      (TemplateCreateBuilder()..update(updates))._build();

  _$TemplateCreate._({required this.name, required this.contentMd}) : super._();
  @override
  TemplateCreate rebuild(void Function(TemplateCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TemplateCreateBuilder toBuilder() => TemplateCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TemplateCreate &&
        name == other.name &&
        contentMd == other.contentMd;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, contentMd.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TemplateCreate')
          ..add('name', name)
          ..add('contentMd', contentMd))
        .toString();
  }
}

class TemplateCreateBuilder
    implements Builder<TemplateCreate, TemplateCreateBuilder> {
  _$TemplateCreate? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _contentMd;
  String? get contentMd => _$this._contentMd;
  set contentMd(String? contentMd) => _$this._contentMd = contentMd;

  TemplateCreateBuilder() {
    TemplateCreate._defaults(this);
  }

  TemplateCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _contentMd = $v.contentMd;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TemplateCreate other) {
    _$v = other as _$TemplateCreate;
  }

  @override
  void update(void Function(TemplateCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TemplateCreate build() => _build();

  _$TemplateCreate _build() {
    final _$result =
        _$v ??
        _$TemplateCreate._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'TemplateCreate',
            'name',
          ),
          contentMd: BuiltValueNullFieldError.checkNotNull(
            contentMd,
            r'TemplateCreate',
            'contentMd',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

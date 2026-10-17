// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'template_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TemplateUpdate extends TemplateUpdate {
  @override
  final String? name;
  @override
  final String? contentMd;

  factory _$TemplateUpdate([void Function(TemplateUpdateBuilder)? updates]) =>
      (TemplateUpdateBuilder()..update(updates))._build();

  _$TemplateUpdate._({this.name, this.contentMd}) : super._();
  @override
  TemplateUpdate rebuild(void Function(TemplateUpdateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TemplateUpdateBuilder toBuilder() => TemplateUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TemplateUpdate &&
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
    return (newBuiltValueToStringHelper(r'TemplateUpdate')
          ..add('name', name)
          ..add('contentMd', contentMd))
        .toString();
  }
}

class TemplateUpdateBuilder
    implements Builder<TemplateUpdate, TemplateUpdateBuilder> {
  _$TemplateUpdate? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _contentMd;
  String? get contentMd => _$this._contentMd;
  set contentMd(String? contentMd) => _$this._contentMd = contentMd;

  TemplateUpdateBuilder() {
    TemplateUpdate._defaults(this);
  }

  TemplateUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _contentMd = $v.contentMd;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TemplateUpdate other) {
    _$v = other as _$TemplateUpdate;
  }

  @override
  void update(void Function(TemplateUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TemplateUpdate build() => _build();

  _$TemplateUpdate _build() {
    final _$result =
        _$v ?? _$TemplateUpdate._(name: name, contentMd: contentMd);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_space_data.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportSpaceData extends ImportSpaceData {
  @override
  final String? name;
  @override
  final String sortKey;
  @override
  final String? existingSpaceId;

  factory _$ImportSpaceData([void Function(ImportSpaceDataBuilder)? updates]) =>
      (ImportSpaceDataBuilder()..update(updates))._build();

  _$ImportSpaceData._({this.name, required this.sortKey, this.existingSpaceId})
    : super._();
  @override
  ImportSpaceData rebuild(void Function(ImportSpaceDataBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportSpaceDataBuilder toBuilder() => ImportSpaceDataBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportSpaceData &&
        name == other.name &&
        sortKey == other.sortKey &&
        existingSpaceId == other.existingSpaceId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, sortKey.hashCode);
    _$hash = $jc(_$hash, existingSpaceId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportSpaceData')
          ..add('name', name)
          ..add('sortKey', sortKey)
          ..add('existingSpaceId', existingSpaceId))
        .toString();
  }
}

class ImportSpaceDataBuilder
    implements Builder<ImportSpaceData, ImportSpaceDataBuilder> {
  _$ImportSpaceData? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _sortKey;
  String? get sortKey => _$this._sortKey;
  set sortKey(String? sortKey) => _$this._sortKey = sortKey;

  String? _existingSpaceId;
  String? get existingSpaceId => _$this._existingSpaceId;
  set existingSpaceId(String? existingSpaceId) =>
      _$this._existingSpaceId = existingSpaceId;

  ImportSpaceDataBuilder() {
    ImportSpaceData._defaults(this);
  }

  ImportSpaceDataBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _sortKey = $v.sortKey;
      _existingSpaceId = $v.existingSpaceId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportSpaceData other) {
    _$v = other as _$ImportSpaceData;
  }

  @override
  void update(void Function(ImportSpaceDataBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportSpaceData build() => _build();

  _$ImportSpaceData _build() {
    final _$result =
        _$v ??
        _$ImportSpaceData._(
          name: name,
          sortKey: BuiltValueNullFieldError.checkNotNull(
            sortKey,
            r'ImportSpaceData',
            'sortKey',
          ),
          existingSpaceId: existingSpaceId,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

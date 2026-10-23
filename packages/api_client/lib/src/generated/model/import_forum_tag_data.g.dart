// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_forum_tag_data.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportForumTagData extends ImportForumTagData {
  @override
  final String channelOriginKey;
  @override
  final String name;
  @override
  final String? emoji;

  factory _$ImportForumTagData([
    void Function(ImportForumTagDataBuilder)? updates,
  ]) => (ImportForumTagDataBuilder()..update(updates))._build();

  _$ImportForumTagData._({
    required this.channelOriginKey,
    required this.name,
    this.emoji,
  }) : super._();
  @override
  ImportForumTagData rebuild(
    void Function(ImportForumTagDataBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ImportForumTagDataBuilder toBuilder() =>
      ImportForumTagDataBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportForumTagData &&
        channelOriginKey == other.channelOriginKey &&
        name == other.name &&
        emoji == other.emoji;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channelOriginKey.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, emoji.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportForumTagData')
          ..add('channelOriginKey', channelOriginKey)
          ..add('name', name)
          ..add('emoji', emoji))
        .toString();
  }
}

class ImportForumTagDataBuilder
    implements Builder<ImportForumTagData, ImportForumTagDataBuilder> {
  _$ImportForumTagData? _$v;

  String? _channelOriginKey;
  String? get channelOriginKey => _$this._channelOriginKey;
  set channelOriginKey(String? channelOriginKey) =>
      _$this._channelOriginKey = channelOriginKey;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _emoji;
  String? get emoji => _$this._emoji;
  set emoji(String? emoji) => _$this._emoji = emoji;

  ImportForumTagDataBuilder() {
    ImportForumTagData._defaults(this);
  }

  ImportForumTagDataBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channelOriginKey = $v.channelOriginKey;
      _name = $v.name;
      _emoji = $v.emoji;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportForumTagData other) {
    _$v = other as _$ImportForumTagData;
  }

  @override
  void update(void Function(ImportForumTagDataBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportForumTagData build() => _build();

  _$ImportForumTagData _build() {
    final _$result =
        _$v ??
        _$ImportForumTagData._(
          channelOriginKey: BuiltValueNullFieldError.checkNotNull(
            channelOriginKey,
            r'ImportForumTagData',
            'channelOriginKey',
          ),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'ImportForumTagData',
            'name',
          ),
          emoji: emoji,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

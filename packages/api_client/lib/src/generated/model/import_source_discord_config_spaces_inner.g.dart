// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_source_discord_config_spaces_inner.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportSourceDiscordConfigSpacesInner
    extends ImportSourceDiscordConfigSpacesInner {
  @override
  final String discordCategory;
  @override
  final JsonObject? target;

  factory _$ImportSourceDiscordConfigSpacesInner([
    void Function(ImportSourceDiscordConfigSpacesInnerBuilder)? updates,
  ]) =>
      (ImportSourceDiscordConfigSpacesInnerBuilder()..update(updates))._build();

  _$ImportSourceDiscordConfigSpacesInner._({
    required this.discordCategory,
    this.target,
  }) : super._();
  @override
  ImportSourceDiscordConfigSpacesInner rebuild(
    void Function(ImportSourceDiscordConfigSpacesInnerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ImportSourceDiscordConfigSpacesInnerBuilder toBuilder() =>
      ImportSourceDiscordConfigSpacesInnerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportSourceDiscordConfigSpacesInner &&
        discordCategory == other.discordCategory &&
        target == other.target;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, discordCategory.hashCode);
    _$hash = $jc(_$hash, target.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportSourceDiscordConfigSpacesInner')
          ..add('discordCategory', discordCategory)
          ..add('target', target))
        .toString();
  }
}

class ImportSourceDiscordConfigSpacesInnerBuilder
    implements
        Builder<
          ImportSourceDiscordConfigSpacesInner,
          ImportSourceDiscordConfigSpacesInnerBuilder
        > {
  _$ImportSourceDiscordConfigSpacesInner? _$v;

  String? _discordCategory;
  String? get discordCategory => _$this._discordCategory;
  set discordCategory(String? discordCategory) =>
      _$this._discordCategory = discordCategory;

  JsonObject? _target;
  JsonObject? get target => _$this._target;
  set target(JsonObject? target) => _$this._target = target;

  ImportSourceDiscordConfigSpacesInnerBuilder() {
    ImportSourceDiscordConfigSpacesInner._defaults(this);
  }

  ImportSourceDiscordConfigSpacesInnerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _discordCategory = $v.discordCategory;
      _target = $v.target;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportSourceDiscordConfigSpacesInner other) {
    _$v = other as _$ImportSourceDiscordConfigSpacesInner;
  }

  @override
  void update(
    void Function(ImportSourceDiscordConfigSpacesInnerBuilder)? updates,
  ) {
    if (updates != null) updates(this);
  }

  @override
  ImportSourceDiscordConfigSpacesInner build() => _build();

  _$ImportSourceDiscordConfigSpacesInner _build() {
    final _$result =
        _$v ??
        _$ImportSourceDiscordConfigSpacesInner._(
          discordCategory: BuiltValueNullFieldError.checkNotNull(
            discordCategory,
            r'ImportSourceDiscordConfigSpacesInner',
            'discordCategory',
          ),
          target: target,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

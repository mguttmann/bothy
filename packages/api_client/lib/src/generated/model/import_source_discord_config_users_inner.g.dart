// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_source_discord_config_users_inner.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportSourceDiscordConfigUsersInner
    extends ImportSourceDiscordConfigUsersInner {
  @override
  final String discordUser;
  @override
  final String target;

  factory _$ImportSourceDiscordConfigUsersInner([
    void Function(ImportSourceDiscordConfigUsersInnerBuilder)? updates,
  ]) =>
      (ImportSourceDiscordConfigUsersInnerBuilder()..update(updates))._build();

  _$ImportSourceDiscordConfigUsersInner._({
    required this.discordUser,
    required this.target,
  }) : super._();
  @override
  ImportSourceDiscordConfigUsersInner rebuild(
    void Function(ImportSourceDiscordConfigUsersInnerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ImportSourceDiscordConfigUsersInnerBuilder toBuilder() =>
      ImportSourceDiscordConfigUsersInnerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportSourceDiscordConfigUsersInner &&
        discordUser == other.discordUser &&
        target == other.target;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, discordUser.hashCode);
    _$hash = $jc(_$hash, target.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportSourceDiscordConfigUsersInner')
          ..add('discordUser', discordUser)
          ..add('target', target))
        .toString();
  }
}

class ImportSourceDiscordConfigUsersInnerBuilder
    implements
        Builder<
          ImportSourceDiscordConfigUsersInner,
          ImportSourceDiscordConfigUsersInnerBuilder
        > {
  _$ImportSourceDiscordConfigUsersInner? _$v;

  String? _discordUser;
  String? get discordUser => _$this._discordUser;
  set discordUser(String? discordUser) => _$this._discordUser = discordUser;

  String? _target;
  String? get target => _$this._target;
  set target(String? target) => _$this._target = target;

  ImportSourceDiscordConfigUsersInnerBuilder() {
    ImportSourceDiscordConfigUsersInner._defaults(this);
  }

  ImportSourceDiscordConfigUsersInnerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _discordUser = $v.discordUser;
      _target = $v.target;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportSourceDiscordConfigUsersInner other) {
    _$v = other as _$ImportSourceDiscordConfigUsersInner;
  }

  @override
  void update(
    void Function(ImportSourceDiscordConfigUsersInnerBuilder)? updates,
  ) {
    if (updates != null) updates(this);
  }

  @override
  ImportSourceDiscordConfigUsersInner build() => _build();

  _$ImportSourceDiscordConfigUsersInner _build() {
    final _$result =
        _$v ??
        _$ImportSourceDiscordConfigUsersInner._(
          discordUser: BuiltValueNullFieldError.checkNotNull(
            discordUser,
            r'ImportSourceDiscordConfigUsersInner',
            'discordUser',
          ),
          target: BuiltValueNullFieldError.checkNotNull(
            target,
            r'ImportSourceDiscordConfigUsersInner',
            'target',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

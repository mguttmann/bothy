// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'view_as_snapshot.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ViewAsSnapshot extends ViewAsSnapshot {
  @override
  final PrincipalRef user;
  @override
  final DateTime expiresAt;
  @override
  final BuiltList<SpaceView> spaces;
  @override
  final BuiltList<ChannelView> channels;

  factory _$ViewAsSnapshot([void Function(ViewAsSnapshotBuilder)? updates]) =>
      (ViewAsSnapshotBuilder()..update(updates))._build();

  _$ViewAsSnapshot._({
    required this.user,
    required this.expiresAt,
    required this.spaces,
    required this.channels,
  }) : super._();
  @override
  ViewAsSnapshot rebuild(void Function(ViewAsSnapshotBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ViewAsSnapshotBuilder toBuilder() => ViewAsSnapshotBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ViewAsSnapshot &&
        user == other.user &&
        expiresAt == other.expiresAt &&
        spaces == other.spaces &&
        channels == other.channels;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, user.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jc(_$hash, spaces.hashCode);
    _$hash = $jc(_$hash, channels.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ViewAsSnapshot')
          ..add('user', user)
          ..add('expiresAt', expiresAt)
          ..add('spaces', spaces)
          ..add('channels', channels))
        .toString();
  }
}

class ViewAsSnapshotBuilder
    implements Builder<ViewAsSnapshot, ViewAsSnapshotBuilder> {
  _$ViewAsSnapshot? _$v;

  PrincipalRefBuilder? _user;
  PrincipalRefBuilder get user => _$this._user ??= PrincipalRefBuilder();
  set user(PrincipalRefBuilder? user) => _$this._user = user;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  ListBuilder<SpaceView>? _spaces;
  ListBuilder<SpaceView> get spaces =>
      _$this._spaces ??= ListBuilder<SpaceView>();
  set spaces(ListBuilder<SpaceView>? spaces) => _$this._spaces = spaces;

  ListBuilder<ChannelView>? _channels;
  ListBuilder<ChannelView> get channels =>
      _$this._channels ??= ListBuilder<ChannelView>();
  set channels(ListBuilder<ChannelView>? channels) =>
      _$this._channels = channels;

  ViewAsSnapshotBuilder() {
    ViewAsSnapshot._defaults(this);
  }

  ViewAsSnapshotBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _user = $v.user.toBuilder();
      _expiresAt = $v.expiresAt;
      _spaces = $v.spaces.toBuilder();
      _channels = $v.channels.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ViewAsSnapshot other) {
    _$v = other as _$ViewAsSnapshot;
  }

  @override
  void update(void Function(ViewAsSnapshotBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ViewAsSnapshot build() => _build();

  _$ViewAsSnapshot _build() {
    _$ViewAsSnapshot _$result;
    try {
      _$result =
          _$v ??
          _$ViewAsSnapshot._(
            user: user.build(),
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'ViewAsSnapshot',
              'expiresAt',
            ),
            spaces: spaces.build(),
            channels: channels.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'user';
        user.build();

        _$failedField = 'spaces';
        spaces.build();
        _$failedField = 'channels';
        channels.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ViewAsSnapshot',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'access_labels.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AccessLabels extends AccessLabels {
  @override
  final BuiltList<PrincipalRef> principals;
  @override
  final BuiltList<AccessLabel> groups;
  @override
  final BuiltList<AccessLabel> spaces;
  @override
  final BuiltList<ChannelLabel> channels;

  factory _$AccessLabels([void Function(AccessLabelsBuilder)? updates]) =>
      (AccessLabelsBuilder()..update(updates))._build();

  _$AccessLabels._({
    required this.principals,
    required this.groups,
    required this.spaces,
    required this.channels,
  }) : super._();
  @override
  AccessLabels rebuild(void Function(AccessLabelsBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AccessLabelsBuilder toBuilder() => AccessLabelsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AccessLabels &&
        principals == other.principals &&
        groups == other.groups &&
        spaces == other.spaces &&
        channels == other.channels;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, principals.hashCode);
    _$hash = $jc(_$hash, groups.hashCode);
    _$hash = $jc(_$hash, spaces.hashCode);
    _$hash = $jc(_$hash, channels.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AccessLabels')
          ..add('principals', principals)
          ..add('groups', groups)
          ..add('spaces', spaces)
          ..add('channels', channels))
        .toString();
  }
}

class AccessLabelsBuilder
    implements Builder<AccessLabels, AccessLabelsBuilder> {
  _$AccessLabels? _$v;

  ListBuilder<PrincipalRef>? _principals;
  ListBuilder<PrincipalRef> get principals =>
      _$this._principals ??= ListBuilder<PrincipalRef>();
  set principals(ListBuilder<PrincipalRef>? principals) =>
      _$this._principals = principals;

  ListBuilder<AccessLabel>? _groups;
  ListBuilder<AccessLabel> get groups =>
      _$this._groups ??= ListBuilder<AccessLabel>();
  set groups(ListBuilder<AccessLabel>? groups) => _$this._groups = groups;

  ListBuilder<AccessLabel>? _spaces;
  ListBuilder<AccessLabel> get spaces =>
      _$this._spaces ??= ListBuilder<AccessLabel>();
  set spaces(ListBuilder<AccessLabel>? spaces) => _$this._spaces = spaces;

  ListBuilder<ChannelLabel>? _channels;
  ListBuilder<ChannelLabel> get channels =>
      _$this._channels ??= ListBuilder<ChannelLabel>();
  set channels(ListBuilder<ChannelLabel>? channels) =>
      _$this._channels = channels;

  AccessLabelsBuilder() {
    AccessLabels._defaults(this);
  }

  AccessLabelsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _principals = $v.principals.toBuilder();
      _groups = $v.groups.toBuilder();
      _spaces = $v.spaces.toBuilder();
      _channels = $v.channels.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AccessLabels other) {
    _$v = other as _$AccessLabels;
  }

  @override
  void update(void Function(AccessLabelsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AccessLabels build() => _build();

  _$AccessLabels _build() {
    _$AccessLabels _$result;
    try {
      _$result =
          _$v ??
          _$AccessLabels._(
            principals: principals.build(),
            groups: groups.build(),
            spaces: spaces.build(),
            channels: channels.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'principals';
        principals.build();
        _$failedField = 'groups';
        groups.build();
        _$failedField = 'spaces';
        spaces.build();
        _$failedField = 'channels';
        channels.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AccessLabels',
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

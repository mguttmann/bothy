// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_delete.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ChannelDelete extends ChannelDelete {
  @override
  final bool? trashProjectFolder;

  factory _$ChannelDelete([void Function(ChannelDeleteBuilder)? updates]) =>
      (ChannelDeleteBuilder()..update(updates))._build();

  _$ChannelDelete._({this.trashProjectFolder}) : super._();
  @override
  ChannelDelete rebuild(void Function(ChannelDeleteBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ChannelDeleteBuilder toBuilder() => ChannelDeleteBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelDelete &&
        trashProjectFolder == other.trashProjectFolder;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, trashProjectFolder.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ChannelDelete',
    )..add('trashProjectFolder', trashProjectFolder)).toString();
  }
}

class ChannelDeleteBuilder
    implements Builder<ChannelDelete, ChannelDeleteBuilder> {
  _$ChannelDelete? _$v;

  bool? _trashProjectFolder;
  bool? get trashProjectFolder => _$this._trashProjectFolder;
  set trashProjectFolder(bool? trashProjectFolder) =>
      _$this._trashProjectFolder = trashProjectFolder;

  ChannelDeleteBuilder() {
    ChannelDelete._defaults(this);
  }

  ChannelDeleteBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _trashProjectFolder = $v.trashProjectFolder;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelDelete other) {
    _$v = other as _$ChannelDelete;
  }

  @override
  void update(void Function(ChannelDeleteBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelDelete build() => _build();

  _$ChannelDelete _build() {
    final _$result =
        _$v ?? _$ChannelDelete._(trashProjectFolder: trashProjectFolder);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

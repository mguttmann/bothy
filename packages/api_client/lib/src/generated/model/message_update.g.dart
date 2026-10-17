// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'message_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$MessageUpdate extends MessageUpdate {
  @override
  final String? contentMd;
  @override
  final BuiltList<String>? mentionIds;

  factory _$MessageUpdate([void Function(MessageUpdateBuilder)? updates]) =>
      (MessageUpdateBuilder()..update(updates))._build();

  _$MessageUpdate._({this.contentMd, this.mentionIds}) : super._();
  @override
  MessageUpdate rebuild(void Function(MessageUpdateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  MessageUpdateBuilder toBuilder() => MessageUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is MessageUpdate &&
        contentMd == other.contentMd &&
        mentionIds == other.mentionIds;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, contentMd.hashCode);
    _$hash = $jc(_$hash, mentionIds.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'MessageUpdate')
          ..add('contentMd', contentMd)
          ..add('mentionIds', mentionIds))
        .toString();
  }
}

class MessageUpdateBuilder
    implements Builder<MessageUpdate, MessageUpdateBuilder> {
  _$MessageUpdate? _$v;

  String? _contentMd;
  String? get contentMd => _$this._contentMd;
  set contentMd(String? contentMd) => _$this._contentMd = contentMd;

  ListBuilder<String>? _mentionIds;
  ListBuilder<String> get mentionIds =>
      _$this._mentionIds ??= ListBuilder<String>();
  set mentionIds(ListBuilder<String>? mentionIds) =>
      _$this._mentionIds = mentionIds;

  MessageUpdateBuilder() {
    MessageUpdate._defaults(this);
  }

  MessageUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _contentMd = $v.contentMd;
      _mentionIds = $v.mentionIds?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(MessageUpdate other) {
    _$v = other as _$MessageUpdate;
  }

  @override
  void update(void Function(MessageUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  MessageUpdate build() => _build();

  _$MessageUpdate _build() {
    _$MessageUpdate _$result;
    try {
      _$result =
          _$v ??
          _$MessageUpdate._(
            contentMd: contentMd,
            mentionIds: _mentionIds?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'mentionIds';
        _mentionIds?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'MessageUpdate',
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

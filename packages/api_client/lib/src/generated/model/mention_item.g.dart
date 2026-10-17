// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'mention_item.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$MentionItem extends MentionItem {
  @override
  final Message message;
  @override
  final DateTime createdAt;

  factory _$MentionItem([void Function(MentionItemBuilder)? updates]) =>
      (MentionItemBuilder()..update(updates))._build();

  _$MentionItem._({required this.message, required this.createdAt}) : super._();
  @override
  MentionItem rebuild(void Function(MentionItemBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  MentionItemBuilder toBuilder() => MentionItemBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is MentionItem &&
        message == other.message &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, message.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'MentionItem')
          ..add('message', message)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class MentionItemBuilder implements Builder<MentionItem, MentionItemBuilder> {
  _$MentionItem? _$v;

  MessageBuilder? _message;
  MessageBuilder get message => _$this._message ??= MessageBuilder();
  set message(MessageBuilder? message) => _$this._message = message;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  MentionItemBuilder() {
    MentionItem._defaults(this);
  }

  MentionItemBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _message = $v.message.toBuilder();
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(MentionItem other) {
    _$v = other as _$MentionItem;
  }

  @override
  void update(void Function(MentionItemBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  MentionItem build() => _build();

  _$MentionItem _build() {
    _$MentionItem _$result;
    try {
      _$result =
          _$v ??
          _$MentionItem._(
            message: message.build(),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'MentionItem',
              'createdAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'message';
        message.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'MentionItem',
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

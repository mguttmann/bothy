// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ThreadCreate extends ThreadCreate {
  @override
  final String? title;
  @override
  final String? parentMessageId;
  @override
  final BuiltList<String>? tagIds;
  @override
  final MessageCreate? firstMessage;

  factory _$ThreadCreate([void Function(ThreadCreateBuilder)? updates]) =>
      (ThreadCreateBuilder()..update(updates))._build();

  _$ThreadCreate._({
    this.title,
    this.parentMessageId,
    this.tagIds,
    this.firstMessage,
  }) : super._();
  @override
  ThreadCreate rebuild(void Function(ThreadCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ThreadCreateBuilder toBuilder() => ThreadCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ThreadCreate &&
        title == other.title &&
        parentMessageId == other.parentMessageId &&
        tagIds == other.tagIds &&
        firstMessage == other.firstMessage;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, title.hashCode);
    _$hash = $jc(_$hash, parentMessageId.hashCode);
    _$hash = $jc(_$hash, tagIds.hashCode);
    _$hash = $jc(_$hash, firstMessage.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ThreadCreate')
          ..add('title', title)
          ..add('parentMessageId', parentMessageId)
          ..add('tagIds', tagIds)
          ..add('firstMessage', firstMessage))
        .toString();
  }
}

class ThreadCreateBuilder
    implements Builder<ThreadCreate, ThreadCreateBuilder> {
  _$ThreadCreate? _$v;

  String? _title;
  String? get title => _$this._title;
  set title(String? title) => _$this._title = title;

  String? _parentMessageId;
  String? get parentMessageId => _$this._parentMessageId;
  set parentMessageId(String? parentMessageId) =>
      _$this._parentMessageId = parentMessageId;

  ListBuilder<String>? _tagIds;
  ListBuilder<String> get tagIds => _$this._tagIds ??= ListBuilder<String>();
  set tagIds(ListBuilder<String>? tagIds) => _$this._tagIds = tagIds;

  MessageCreateBuilder? _firstMessage;
  MessageCreateBuilder get firstMessage =>
      _$this._firstMessage ??= MessageCreateBuilder();
  set firstMessage(MessageCreateBuilder? firstMessage) =>
      _$this._firstMessage = firstMessage;

  ThreadCreateBuilder() {
    ThreadCreate._defaults(this);
  }

  ThreadCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _title = $v.title;
      _parentMessageId = $v.parentMessageId;
      _tagIds = $v.tagIds?.toBuilder();
      _firstMessage = $v.firstMessage?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ThreadCreate other) {
    _$v = other as _$ThreadCreate;
  }

  @override
  void update(void Function(ThreadCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ThreadCreate build() => _build();

  _$ThreadCreate _build() {
    _$ThreadCreate _$result;
    try {
      _$result =
          _$v ??
          _$ThreadCreate._(
            title: title,
            parentMessageId: parentMessageId,
            tagIds: _tagIds?.build(),
            firstMessage: _firstMessage?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'tagIds';
        _tagIds?.build();
        _$failedField = 'firstMessage';
        _firstMessage?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ThreadCreate',
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

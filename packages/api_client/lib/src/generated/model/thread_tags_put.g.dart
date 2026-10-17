// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_tags_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ThreadTagsPut extends ThreadTagsPut {
  @override
  final BuiltList<String> tagIds;

  factory _$ThreadTagsPut([void Function(ThreadTagsPutBuilder)? updates]) =>
      (ThreadTagsPutBuilder()..update(updates))._build();

  _$ThreadTagsPut._({required this.tagIds}) : super._();
  @override
  ThreadTagsPut rebuild(void Function(ThreadTagsPutBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ThreadTagsPutBuilder toBuilder() => ThreadTagsPutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ThreadTagsPut && tagIds == other.tagIds;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, tagIds.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ThreadTagsPut',
    )..add('tagIds', tagIds)).toString();
  }
}

class ThreadTagsPutBuilder
    implements Builder<ThreadTagsPut, ThreadTagsPutBuilder> {
  _$ThreadTagsPut? _$v;

  ListBuilder<String>? _tagIds;
  ListBuilder<String> get tagIds => _$this._tagIds ??= ListBuilder<String>();
  set tagIds(ListBuilder<String>? tagIds) => _$this._tagIds = tagIds;

  ThreadTagsPutBuilder() {
    ThreadTagsPut._defaults(this);
  }

  ThreadTagsPutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _tagIds = $v.tagIds.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ThreadTagsPut other) {
    _$v = other as _$ThreadTagsPut;
  }

  @override
  void update(void Function(ThreadTagsPutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ThreadTagsPut build() => _build();

  _$ThreadTagsPut _build() {
    _$ThreadTagsPut _$result;
    try {
      _$result = _$v ?? _$ThreadTagsPut._(tagIds: tagIds.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'tagIds';
        tagIds.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ThreadTagsPut',
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

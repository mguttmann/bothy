// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_title_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ThreadTitlePut extends ThreadTitlePut {
  @override
  final String title;

  factory _$ThreadTitlePut([void Function(ThreadTitlePutBuilder)? updates]) =>
      (ThreadTitlePutBuilder()..update(updates))._build();

  _$ThreadTitlePut._({required this.title}) : super._();
  @override
  ThreadTitlePut rebuild(void Function(ThreadTitlePutBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ThreadTitlePutBuilder toBuilder() => ThreadTitlePutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ThreadTitlePut && title == other.title;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, title.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ThreadTitlePut',
    )..add('title', title)).toString();
  }
}

class ThreadTitlePutBuilder
    implements Builder<ThreadTitlePut, ThreadTitlePutBuilder> {
  _$ThreadTitlePut? _$v;

  String? _title;
  String? get title => _$this._title;
  set title(String? title) => _$this._title = title;

  ThreadTitlePutBuilder() {
    ThreadTitlePut._defaults(this);
  }

  ThreadTitlePutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _title = $v.title;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ThreadTitlePut other) {
    _$v = other as _$ThreadTitlePut;
  }

  @override
  void update(void Function(ThreadTitlePutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ThreadTitlePut build() => _build();

  _$ThreadTitlePut _build() {
    final _$result =
        _$v ??
        _$ThreadTitlePut._(
          title: BuiltValueNullFieldError.checkNotNull(
            title,
            r'ThreadTitlePut',
            'title',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

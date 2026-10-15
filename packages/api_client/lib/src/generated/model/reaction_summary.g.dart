// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'reaction_summary.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ReactionSummary extends ReactionSummary {
  @override
  final String emoji;
  @override
  final int count;

  factory _$ReactionSummary([void Function(ReactionSummaryBuilder)? updates]) =>
      (ReactionSummaryBuilder()..update(updates))._build();

  _$ReactionSummary._({required this.emoji, required this.count}) : super._();
  @override
  ReactionSummary rebuild(void Function(ReactionSummaryBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ReactionSummaryBuilder toBuilder() => ReactionSummaryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ReactionSummary &&
        emoji == other.emoji &&
        count == other.count;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, emoji.hashCode);
    _$hash = $jc(_$hash, count.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ReactionSummary')
          ..add('emoji', emoji)
          ..add('count', count))
        .toString();
  }
}

class ReactionSummaryBuilder
    implements Builder<ReactionSummary, ReactionSummaryBuilder> {
  _$ReactionSummary? _$v;

  String? _emoji;
  String? get emoji => _$this._emoji;
  set emoji(String? emoji) => _$this._emoji = emoji;

  int? _count;
  int? get count => _$this._count;
  set count(int? count) => _$this._count = count;

  ReactionSummaryBuilder() {
    ReactionSummary._defaults(this);
  }

  ReactionSummaryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _emoji = $v.emoji;
      _count = $v.count;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ReactionSummary other) {
    _$v = other as _$ReactionSummary;
  }

  @override
  void update(void Function(ReactionSummaryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ReactionSummary build() => _build();

  _$ReactionSummary _build() {
    final _$result =
        _$v ??
        _$ReactionSummary._(
          emoji: BuiltValueNullFieldError.checkNotNull(
            emoji,
            r'ReactionSummary',
            'emoji',
          ),
          count: BuiltValueNullFieldError.checkNotNull(
            count,
            r'ReactionSummary',
            'count',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

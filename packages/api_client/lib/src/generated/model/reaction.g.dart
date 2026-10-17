// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'reaction.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Reaction extends Reaction {
  @override
  final String emoji;
  @override
  final PrincipalRef principal;
  @override
  final DateTime createdAt;

  factory _$Reaction([void Function(ReactionBuilder)? updates]) =>
      (ReactionBuilder()..update(updates))._build();

  _$Reaction._({
    required this.emoji,
    required this.principal,
    required this.createdAt,
  }) : super._();
  @override
  Reaction rebuild(void Function(ReactionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ReactionBuilder toBuilder() => ReactionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Reaction &&
        emoji == other.emoji &&
        principal == other.principal &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, emoji.hashCode);
    _$hash = $jc(_$hash, principal.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Reaction')
          ..add('emoji', emoji)
          ..add('principal', principal)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class ReactionBuilder implements Builder<Reaction, ReactionBuilder> {
  _$Reaction? _$v;

  String? _emoji;
  String? get emoji => _$this._emoji;
  set emoji(String? emoji) => _$this._emoji = emoji;

  PrincipalRefBuilder? _principal;
  PrincipalRefBuilder get principal =>
      _$this._principal ??= PrincipalRefBuilder();
  set principal(PrincipalRefBuilder? principal) =>
      _$this._principal = principal;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  ReactionBuilder() {
    Reaction._defaults(this);
  }

  ReactionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _emoji = $v.emoji;
      _principal = $v.principal.toBuilder();
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Reaction other) {
    _$v = other as _$Reaction;
  }

  @override
  void update(void Function(ReactionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Reaction build() => _build();

  _$Reaction _build() {
    _$Reaction _$result;
    try {
      _$result =
          _$v ??
          _$Reaction._(
            emoji: BuiltValueNullFieldError.checkNotNull(
              emoji,
              r'Reaction',
              'emoji',
            ),
            principal: principal.build(),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'Reaction',
              'createdAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'principal';
        principal.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Reaction',
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

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_preference_put.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$NotificationPreferencePut extends NotificationPreferencePut {
  @override
  final NotificationLevel level;
  @override
  final DateTime? mutedUntil;

  factory _$NotificationPreferencePut([
    void Function(NotificationPreferencePutBuilder)? updates,
  ]) => (NotificationPreferencePutBuilder()..update(updates))._build();

  _$NotificationPreferencePut._({required this.level, this.mutedUntil})
    : super._();
  @override
  NotificationPreferencePut rebuild(
    void Function(NotificationPreferencePutBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  NotificationPreferencePutBuilder toBuilder() =>
      NotificationPreferencePutBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is NotificationPreferencePut &&
        level == other.level &&
        mutedUntil == other.mutedUntil;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, level.hashCode);
    _$hash = $jc(_$hash, mutedUntil.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'NotificationPreferencePut')
          ..add('level', level)
          ..add('mutedUntil', mutedUntil))
        .toString();
  }
}

class NotificationPreferencePutBuilder
    implements
        Builder<NotificationPreferencePut, NotificationPreferencePutBuilder> {
  _$NotificationPreferencePut? _$v;

  NotificationLevel? _level;
  NotificationLevel? get level => _$this._level;
  set level(NotificationLevel? level) => _$this._level = level;

  DateTime? _mutedUntil;
  DateTime? get mutedUntil => _$this._mutedUntil;
  set mutedUntil(DateTime? mutedUntil) => _$this._mutedUntil = mutedUntil;

  NotificationPreferencePutBuilder() {
    NotificationPreferencePut._defaults(this);
  }

  NotificationPreferencePutBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _level = $v.level;
      _mutedUntil = $v.mutedUntil;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(NotificationPreferencePut other) {
    _$v = other as _$NotificationPreferencePut;
  }

  @override
  void update(void Function(NotificationPreferencePutBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  NotificationPreferencePut build() => _build();

  _$NotificationPreferencePut _build() {
    final _$result =
        _$v ??
        _$NotificationPreferencePut._(
          level: BuiltValueNullFieldError.checkNotNull(
            level,
            r'NotificationPreferencePut',
            'level',
          ),
          mutedUntil: mutedUntil,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

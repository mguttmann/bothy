// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_focus.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayFocus extends GatewayFocus {
  @override
  final BuiltList<String> threadIds;
  @override
  final bool? idle;
  @override
  final BuiltList<String>? backgroundThreadIds;

  factory _$GatewayFocus([void Function(GatewayFocusBuilder)? updates]) =>
      (GatewayFocusBuilder()..update(updates))._build();

  _$GatewayFocus._({
    required this.threadIds,
    this.idle,
    this.backgroundThreadIds,
  }) : super._();
  @override
  GatewayFocus rebuild(void Function(GatewayFocusBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GatewayFocusBuilder toBuilder() => GatewayFocusBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayFocus &&
        threadIds == other.threadIds &&
        idle == other.idle &&
        backgroundThreadIds == other.backgroundThreadIds;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadIds.hashCode);
    _$hash = $jc(_$hash, idle.hashCode);
    _$hash = $jc(_$hash, backgroundThreadIds.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayFocus')
          ..add('threadIds', threadIds)
          ..add('idle', idle)
          ..add('backgroundThreadIds', backgroundThreadIds))
        .toString();
  }
}

class GatewayFocusBuilder
    implements Builder<GatewayFocus, GatewayFocusBuilder> {
  _$GatewayFocus? _$v;

  ListBuilder<String>? _threadIds;
  ListBuilder<String> get threadIds =>
      _$this._threadIds ??= ListBuilder<String>();
  set threadIds(ListBuilder<String>? threadIds) =>
      _$this._threadIds = threadIds;

  bool? _idle;
  bool? get idle => _$this._idle;
  set idle(bool? idle) => _$this._idle = idle;

  ListBuilder<String>? _backgroundThreadIds;
  ListBuilder<String> get backgroundThreadIds =>
      _$this._backgroundThreadIds ??= ListBuilder<String>();
  set backgroundThreadIds(ListBuilder<String>? backgroundThreadIds) =>
      _$this._backgroundThreadIds = backgroundThreadIds;

  GatewayFocusBuilder() {
    GatewayFocus._defaults(this);
  }

  GatewayFocusBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadIds = $v.threadIds.toBuilder();
      _idle = $v.idle;
      _backgroundThreadIds = $v.backgroundThreadIds?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayFocus other) {
    _$v = other as _$GatewayFocus;
  }

  @override
  void update(void Function(GatewayFocusBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayFocus build() => _build();

  _$GatewayFocus _build() {
    _$GatewayFocus _$result;
    try {
      _$result =
          _$v ??
          _$GatewayFocus._(
            threadIds: threadIds.build(),
            idle: idle,
            backgroundThreadIds: _backgroundThreadIds?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'threadIds';
        threadIds.build();

        _$failedField = 'backgroundThreadIds';
        _backgroundThreadIds?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GatewayFocus',
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

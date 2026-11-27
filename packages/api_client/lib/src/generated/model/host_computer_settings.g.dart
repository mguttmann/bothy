// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_computer_settings.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostComputerSettings extends HostComputerSettings {
  @override
  final int imageMaxEdge;
  @override
  final int jpegQuality;
  @override
  final int settleMs;
  @override
  final bool autoScreenshot;
  @override
  final int waitMaxMs;
  @override
  final int keyRepeatMax;
  @override
  final int scrollAmountMax;
  @override
  final int auditAppsMax;
  @override
  final int statusIntervalMs;

  factory _$HostComputerSettings([
    void Function(HostComputerSettingsBuilder)? updates,
  ]) => (HostComputerSettingsBuilder()..update(updates))._build();

  _$HostComputerSettings._({
    required this.imageMaxEdge,
    required this.jpegQuality,
    required this.settleMs,
    required this.autoScreenshot,
    required this.waitMaxMs,
    required this.keyRepeatMax,
    required this.scrollAmountMax,
    required this.auditAppsMax,
    required this.statusIntervalMs,
  }) : super._();
  @override
  HostComputerSettings rebuild(
    void Function(HostComputerSettingsBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  HostComputerSettingsBuilder toBuilder() =>
      HostComputerSettingsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostComputerSettings &&
        imageMaxEdge == other.imageMaxEdge &&
        jpegQuality == other.jpegQuality &&
        settleMs == other.settleMs &&
        autoScreenshot == other.autoScreenshot &&
        waitMaxMs == other.waitMaxMs &&
        keyRepeatMax == other.keyRepeatMax &&
        scrollAmountMax == other.scrollAmountMax &&
        auditAppsMax == other.auditAppsMax &&
        statusIntervalMs == other.statusIntervalMs;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, imageMaxEdge.hashCode);
    _$hash = $jc(_$hash, jpegQuality.hashCode);
    _$hash = $jc(_$hash, settleMs.hashCode);
    _$hash = $jc(_$hash, autoScreenshot.hashCode);
    _$hash = $jc(_$hash, waitMaxMs.hashCode);
    _$hash = $jc(_$hash, keyRepeatMax.hashCode);
    _$hash = $jc(_$hash, scrollAmountMax.hashCode);
    _$hash = $jc(_$hash, auditAppsMax.hashCode);
    _$hash = $jc(_$hash, statusIntervalMs.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostComputerSettings')
          ..add('imageMaxEdge', imageMaxEdge)
          ..add('jpegQuality', jpegQuality)
          ..add('settleMs', settleMs)
          ..add('autoScreenshot', autoScreenshot)
          ..add('waitMaxMs', waitMaxMs)
          ..add('keyRepeatMax', keyRepeatMax)
          ..add('scrollAmountMax', scrollAmountMax)
          ..add('auditAppsMax', auditAppsMax)
          ..add('statusIntervalMs', statusIntervalMs))
        .toString();
  }
}

class HostComputerSettingsBuilder
    implements Builder<HostComputerSettings, HostComputerSettingsBuilder> {
  _$HostComputerSettings? _$v;

  int? _imageMaxEdge;
  int? get imageMaxEdge => _$this._imageMaxEdge;
  set imageMaxEdge(int? imageMaxEdge) => _$this._imageMaxEdge = imageMaxEdge;

  int? _jpegQuality;
  int? get jpegQuality => _$this._jpegQuality;
  set jpegQuality(int? jpegQuality) => _$this._jpegQuality = jpegQuality;

  int? _settleMs;
  int? get settleMs => _$this._settleMs;
  set settleMs(int? settleMs) => _$this._settleMs = settleMs;

  bool? _autoScreenshot;
  bool? get autoScreenshot => _$this._autoScreenshot;
  set autoScreenshot(bool? autoScreenshot) =>
      _$this._autoScreenshot = autoScreenshot;

  int? _waitMaxMs;
  int? get waitMaxMs => _$this._waitMaxMs;
  set waitMaxMs(int? waitMaxMs) => _$this._waitMaxMs = waitMaxMs;

  int? _keyRepeatMax;
  int? get keyRepeatMax => _$this._keyRepeatMax;
  set keyRepeatMax(int? keyRepeatMax) => _$this._keyRepeatMax = keyRepeatMax;

  int? _scrollAmountMax;
  int? get scrollAmountMax => _$this._scrollAmountMax;
  set scrollAmountMax(int? scrollAmountMax) =>
      _$this._scrollAmountMax = scrollAmountMax;

  int? _auditAppsMax;
  int? get auditAppsMax => _$this._auditAppsMax;
  set auditAppsMax(int? auditAppsMax) => _$this._auditAppsMax = auditAppsMax;

  int? _statusIntervalMs;
  int? get statusIntervalMs => _$this._statusIntervalMs;
  set statusIntervalMs(int? statusIntervalMs) =>
      _$this._statusIntervalMs = statusIntervalMs;

  HostComputerSettingsBuilder() {
    HostComputerSettings._defaults(this);
  }

  HostComputerSettingsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _imageMaxEdge = $v.imageMaxEdge;
      _jpegQuality = $v.jpegQuality;
      _settleMs = $v.settleMs;
      _autoScreenshot = $v.autoScreenshot;
      _waitMaxMs = $v.waitMaxMs;
      _keyRepeatMax = $v.keyRepeatMax;
      _scrollAmountMax = $v.scrollAmountMax;
      _auditAppsMax = $v.auditAppsMax;
      _statusIntervalMs = $v.statusIntervalMs;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostComputerSettings other) {
    _$v = other as _$HostComputerSettings;
  }

  @override
  void update(void Function(HostComputerSettingsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostComputerSettings build() => _build();

  _$HostComputerSettings _build() {
    final _$result =
        _$v ??
        _$HostComputerSettings._(
          imageMaxEdge: BuiltValueNullFieldError.checkNotNull(
            imageMaxEdge,
            r'HostComputerSettings',
            'imageMaxEdge',
          ),
          jpegQuality: BuiltValueNullFieldError.checkNotNull(
            jpegQuality,
            r'HostComputerSettings',
            'jpegQuality',
          ),
          settleMs: BuiltValueNullFieldError.checkNotNull(
            settleMs,
            r'HostComputerSettings',
            'settleMs',
          ),
          autoScreenshot: BuiltValueNullFieldError.checkNotNull(
            autoScreenshot,
            r'HostComputerSettings',
            'autoScreenshot',
          ),
          waitMaxMs: BuiltValueNullFieldError.checkNotNull(
            waitMaxMs,
            r'HostComputerSettings',
            'waitMaxMs',
          ),
          keyRepeatMax: BuiltValueNullFieldError.checkNotNull(
            keyRepeatMax,
            r'HostComputerSettings',
            'keyRepeatMax',
          ),
          scrollAmountMax: BuiltValueNullFieldError.checkNotNull(
            scrollAmountMax,
            r'HostComputerSettings',
            'scrollAmountMax',
          ),
          auditAppsMax: BuiltValueNullFieldError.checkNotNull(
            auditAppsMax,
            r'HostComputerSettings',
            'auditAppsMax',
          ),
          statusIntervalMs: BuiltValueNullFieldError.checkNotNull(
            statusIntervalMs,
            r'HostComputerSettings',
            'statusIntervalMs',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint

// SPDX-License-Identifier: Apache-2.0

/// The hints of the gateway inside the phone app (docs/spec/13-push-notifications.md 4.6 item 1:
/// a device with a gateway connection gets `notification.created` instead of a push, and "the
/// app shows its own notification or an in-app banner"; docs/spec/15-client-architecture.md 10;
/// docs/decisions.d/AUD-client-app-01.md).
///
/// The phone has a connection only while it is in front, so the hint is shown where the person
/// looks: [InAppBanners] is the [NotificationPresenter] of a phone build and keeps the newest
/// hint, [InAppBannerHost] lays it over the app until the person opens or dismisses it or its
/// time is over.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import 'notification_presenter.dart';
import 'toast_content.dart';

/// One hint as the phone app shows it over its content.
///
/// [toString] names no content, so a log line never carries it (13 10).
@immutable
final class InAppBanner {
  /// Creates the banner of [content] in the slot [tag]; [silent] shows it without the haptic
  /// tap (quiet hours, 13 3.4); [payload] is the app path a tap opens.
  const InAppBanner({
    required this.content,
    required this.tag,
    required this.silent,
    required this.payload,
  });

  /// Title and text.
  final ToastContent content;

  /// The slot: one per thread, one per kind without a thread (13 4.5).
  final String tag;

  /// Whether the hint came inside quiet hours.
  final bool silent;

  /// The app path a tap opens, for example `/t/<thread>?m=<message>`.
  final String payload;

  @override
  String toString() => 'InAppBanner(<content>)';
}

/// The [NotificationPresenter] of a phone build: the newest hint is the one banner the app
/// shows.
///
/// The screen of a phone has room for one banner, so a later hint replaces the one on screen
/// whatever its slot; the counters of the tabs and rows keep counting the rest. Nothing is
/// stored: the banner lives as long as the app shows it.
///
/// Example:
/// ```dart
/// final banners = InAppBanners();
/// await banners.show(content, tag: 'thread:$id', silent: false, payload: '/t/$id');
/// banners.clicks.listen(router.go);
/// ```
final class InAppBanners implements NotificationPresenter {
  final ValueNotifier<InAppBanner?> _current = ValueNotifier(null);
  final StreamController<String> _clicks = StreamController<String>.broadcast();
  bool _disposed = false;

  /// The banner to show, or null.
  ValueListenable<InAppBanner?> get current => _current;

  @override
  Future<void> show(
    ToastContent content, {
    required String tag,
    required bool silent,
    required String payload,
  }) async {
    if (_disposed) return;
    _current.value = InAppBanner(
      content: content,
      tag: tag,
      silent: silent,
      payload: payload,
    );
  }

  @override
  Future<void> cancelThread(String tag) async {
    if (!_disposed && _current.value?.tag == tag) _current.value = null;
  }

  /// The person tapped [banner]: it leaves the screen and its target opens ([clicks]).
  void open(InAppBanner banner) {
    if (_disposed) return;
    dismiss(banner);
    _clicks.add(banner.payload);
  }

  /// Takes [banner] off the screen; a newer banner that replaced it stays.
  void dismiss(InAppBanner banner) {
    if (!_disposed && identical(_current.value, banner)) _current.value = null;
  }

  /// Takes whatever shows off the screen: the account ended.
  void clear() {
    if (!_disposed) _current.value = null;
  }

  @override
  Stream<String> get clicks => _clicks.stream;

  /// A banner never starts the app.
  @override
  Future<String?> launchPayload() async => null;

  @override
  Future<void> dispose() async {
    if (_disposed) return;
    _disposed = true;
    _current.dispose();
    await _clicks.close();
  }
}

/// Lays the banner of [banners] over [child], at the top inside the safe area.
///
/// A banner shows only while [visible]: the app passes false while it is signed out or the app
/// lock is closed, so no title or text of a hint is on screen then, and a hint that arrived
/// meanwhile shows once the content is back. A banner stays for [stay] and then leaves by
/// itself; with a screen reader or a switch control active (`MediaQuery.accessibleNavigation`)
/// it stays until the person opens or dismisses it or the next hint replaces it (WCAG 2.2.1).
/// It appears without an animation. A hint that is not silent gives one haptic tap when its
/// banner first shows.
///
/// Example:
/// ```dart
/// InAppBannerHost(banners: banners, visible: signedIn, child: router)
/// ```
class InAppBannerHost extends StatefulWidget {
  /// Creates the host.
  const InAppBannerHost({
    super.key,
    required this.banners,
    required this.child,
    this.visible = true,
  });

  /// How long a banner stays on screen.
  static const Duration stay = Duration(seconds: 6);

  /// The banners to show.
  final InAppBanners banners;

  /// Whether a banner may show now.
  final bool visible;

  /// The app.
  final Widget child;

  @override
  State<InAppBannerHost> createState() => _InAppBannerHostState();
}

class _InAppBannerHostState extends State<InAppBannerHost> {
  /// The banner on screen.
  InAppBanner? _shown;

  /// The banner that already gave its haptic tap.
  InAppBanner? _felt;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    widget.banners.current.addListener(_changed);
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _sync();
  }

  @override
  void didUpdateWidget(InAppBannerHost old) {
    super.didUpdateWidget(old);
    if (!identical(old.banners, widget.banners)) {
      old.banners.current.removeListener(_changed);
      widget.banners.current.addListener(_changed);
    }
    _sync();
  }

  @override
  void dispose() {
    widget.banners.current.removeListener(_changed);
    _timer?.cancel();
    super.dispose();
  }

  void _changed() {
    if (!mounted) return;
    // A provider may take the banner away while the widgets of a frame build (the engine of
    // an account that ended); the host then follows right after that frame.
    if (SchedulerBinding.instance.schedulerPhase ==
        SchedulerPhase.persistentCallbacks) {
      SchedulerBinding.instance.addPostFrameCallback((_) {
        if (mounted) setState(_sync);
      });
      return;
    }
    setState(_sync);
  }

  /// Takes the banner that may show now on screen and starts what belongs to it: its time
  /// and, once per banner, the haptic tap.
  void _sync() {
    final banner = widget.visible ? widget.banners.current.value : null;
    if (identical(banner, _shown)) return;
    _shown = banner;
    _timer?.cancel();
    _timer = null;
    if (banner == null) return;
    if (!banner.silent && !identical(banner, _felt)) {
      unawaited(
        HapticFeedback.mediumImpact().then<void>(
          (_) {},
          onError: (Object _) {},
        ),
      );
    }
    _felt = banner;
    if (!MediaQuery.accessibleNavigationOf(context)) {
      _timer = Timer(
        InAppBannerHost.stay,
        () => widget.banners.dismiss(banner),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final banner = _shown;
    return Stack(
      fit: StackFit.expand,
      children: [
        widget.child,
        if (banner != null)
          SafeArea(
            bottom: false,
            child: Align(
              alignment: Alignment.topCenter,
              child: Padding(
                padding: const EdgeInsets.all(DsSpace.s8),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(
                    maxWidth: DsLayout.overlayWidth,
                  ),
                  child: InAppBannerView(
                    key: const ValueKey('notification.banner'),
                    content: banner.content,
                    onOpen: () => widget.banners.open(banner),
                    onDismiss: () => widget.banners.dismiss(banner),
                  ),
                ),
              ),
            ),
          ),
      ],
    );
  }
}

/// The look of one banner: the title and the text of [content] as one button that opens the
/// target ([onOpen]) and a close button ([onDismiss]).
///
/// The button is a polite live region, so a screen reader announces the hint when the banner
/// appears (01 8 "Live content": a hint is a state change the person asked to be told about).
class InAppBannerView extends StatelessWidget {
  /// Creates the banner.
  const InAppBannerView({
    super.key,
    required this.content,
    required this.onOpen,
    required this.onDismiss,
  });

  /// Title and text.
  final ToastContent content;

  /// Opens the target of the hint.
  final VoidCallback onOpen;

  /// Takes the banner off the screen.
  final VoidCallback onDismiss;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Material(
      type: MaterialType.transparency,
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: colors.surfaceOverlay,
          border: Border.all(
            color: colors.borderDefault,
            width: DsBorderWidth.hairline,
          ),
          borderRadius: BorderRadius.circular(DsRadius.lg),
          boxShadow: context.dsElevation.menu,
        ),
        child: Row(
          children: [
            Expanded(
              // One node for a screen reader: a button that reads title and text and
              // announces them when it appears.
              child: MergeSemantics(
                child: Semantics(
                  liveRegion: true,
                  child: DsTappable(
                    key: const ValueKey('notification.banner.open'),
                    onTap: onOpen,
                    radius: DsRadius.lg,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        horizontal: DsSpace.s12,
                        vertical: DsSpace.s8,
                      ),
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        mainAxisAlignment: MainAxisAlignment.center,
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            content.title,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: dsTextStyle(DsType.label)
                                .copyWith(color: colors.textPrimary),
                          ),
                          Text(
                            content.body,
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                            style: dsTextStyle(DsType.bodySm)
                                .copyWith(color: colors.textSecondary),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
            ),
            DsTappable(
              key: const ValueKey('notification.banner.dismiss'),
              onTap: onDismiss,
              label: l.notificationBannerDismiss,
              radius: DsRadius.lg,
              excludeChildSemantics: true,
              child: Center(
                widthFactor: 1,
                heightFactor: 1,
                child: DsIcon(
                  DsIcons.actionDismiss,
                  size: DsIconSizeRole.sm,
                  color: colors.textSecondary,
                  decorative: true,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

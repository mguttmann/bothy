// SPDX-License-Identifier: Apache-2.0

/// The virtualized list of the history (brief M4-20 Design note 1): newest at the bottom,
/// paging up when the person scrolls near the top, the position anchored when older pages
/// arrive, the jump to a message and the report whether the newest message is in view (the
/// read state, Design note 2).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';

import 'history_model.dart';

/// The rows of [items] in a reversed list: index 0 is the newest row at the bottom, so rows
/// added above (an older page) never move what is on screen.
class MessageList extends StatefulWidget {
  /// Creates the list.
  const MessageList({
    super.key,
    required this.items,
    required this.itemBuilder,
    required this.onNearTop,
    required this.onNewestVisible,
    this.jumpTo,
  });

  /// The rows, oldest first.
  final List<HistoryItem> items;

  /// Builds a row.
  final Widget Function(BuildContext context, HistoryItem item) itemBuilder;

  /// Called when the person scrolled near the oldest row.
  final VoidCallback onNearTop;

  /// Called with whether the newest row is in view, on every change.
  final ValueChanged<bool> onNewestVisible;

  /// The key of a row to bring into view once it is in [items] (`message.<id>`).
  final String? jumpTo;

  @override
  State<MessageList> createState() => _MessageListState();
}

class _MessageListState extends State<MessageList> {
  final ScrollController _scroll = ScrollController();
  final Map<String, GlobalKey> _keys = {};
  String? _jumped;
  bool? _newest;

  @override
  void initState() {
    super.initState();
    SchedulerBinding.instance.addPostFrameCallback((_) => _afterFrame());
  }

  @override
  void didUpdateWidget(MessageList old) {
    super.didUpdateWidget(old);
    SchedulerBinding.instance.addPostFrameCallback((_) => _afterFrame());
  }

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  void _afterFrame() {
    if (!mounted) return;
    _report();
    final target = widget.jumpTo;
    if (target != null &&
        target != _jumped &&
        widget.items.any((i) => i.key == target)) {
      _jumped = target;
      unawaited(_bringIntoView(target));
    }
  }

  /// Scrolls up page by page until the row [key] is built, then centers it.
  Future<void> _bringIntoView(String key) async {
    for (var step = 0; step < widget.items.length && mounted; step++) {
      final row = _keys[key]?.currentContext;
      if (row != null && row.mounted) {
        await Scrollable.ensureVisible(row, alignment: 0.5);
        return;
      }
      if (!_scroll.hasClients) return;
      final position = _scroll.position;
      if (position.pixels >= position.maxScrollExtent) return;
      _scroll.jumpTo(
        (position.pixels + position.viewportDimension).clamp(
          0,
          position.maxScrollExtent,
        ),
      );
      await SchedulerBinding.instance.endOfFrame;
    }
  }

  void _report() {
    if (!_scroll.hasClients) {
      _setNewest(true);
      return;
    }
    final p = _scroll.position;
    _setNewest(p.pixels <= p.minScrollExtent + DsSpace.s24);
    if (p.extentAfter < p.viewportDimension / 2) widget.onNearTop();
  }

  void _setNewest(bool visible) {
    if (_newest == visible) return;
    _newest = visible;
    widget.onNewestVisible(visible);
  }

  @override
  Widget build(BuildContext context) {
    final items = widget.items;
    return NotificationListener<ScrollNotification>(
      onNotification: (n) {
        if (n.depth == 0) _report();
        return false;
      },
      child: ListView.builder(
        key: const ValueKey('history.list'),
        controller: _scroll,
        reverse: true,
        padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
        itemCount: items.length,
        findChildIndexCallback: (key) {
          if (key is! ValueKey<String>) return null;
          final at = items.indexWhere((i) => 'row.${i.key}' == key.value);
          return at < 0 ? null : items.length - 1 - at;
        },
        itemBuilder: (context, index) {
          final item = items[items.length - 1 - index];
          final key = _keys.putIfAbsent(item.key, GlobalKey.new);
          return KeyedSubtree(
            key: ValueKey('row.${item.key}'),
            child: KeyedSubtree(
              key: key,
              child: widget.itemBuilder(context, item),
            ),
          );
        },
      ),
    );
  }
}

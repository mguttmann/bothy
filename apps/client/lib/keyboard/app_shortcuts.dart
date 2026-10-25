// SPDX-License-Identifier: Apache-2.0

/// The shortcut layer of the desktop shell (docs/spec/01-product.md 7; brief M4-24 Design
/// note 2): [AppShortcuts] maps the app bindings of `bindings.dart` to their intents above
/// the columns, and the views offer the targets of those intents with [KeyboardActions].
///
/// Scopes: a focused input keeps every key it handles itself (Enter, Shift+Enter, arrow up,
/// Esc with suggestions open), because the key reaches the focused node first; the app layer
/// binds no plain key except Esc and F6. A handler that is not enabled where the focus is
/// does nothing and does not consume its key. Components register no intent at all, so no
/// binding can trigger an approval (R-PR-04). [KeyboardScope.passThrough] marks a region
/// (the terminal of M4-36) where only shortcuts with the platform modifier reach the app
/// (R-PR-10).
library;

import 'dart:async';

import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';

import 'bindings.dart';
import 'column_focus.dart';
import 'intents.dart';
import 'shortcut_list.dart';

/// What a view offers for one [ShortcutAction].
///
/// [enabled] false makes the key pass on as if nothing were bound; [focusScoped] limits the
/// handler to a focus inside the view that registered it (Esc marks read only in the history,
/// 01 7).
///
/// Example: `KeyboardHandler(stop, enabled: canStop)`.
@immutable
final class KeyboardHandler {
  /// Creates the handler.
  const KeyboardHandler(
    this.onInvoke, {
    this.enabled = true,
    this.focusScoped = false,
  });

  /// Runs the action.
  final VoidCallback onInvoke;

  /// Whether the action has a target here.
  final bool enabled;

  /// Whether the handler applies only while the focus is inside its view.
  final bool focusScoped;
}

/// A registered provider of handlers: a [KeyboardActions] or the shell itself.
abstract interface class _HandlerSource {
  Map<ShortcutAction, KeyboardHandler> get handlers;

  int get priority;

  int get serial;

  bool holdsFocus(FocusNode? focus);
}

/// The handlers of the shell and its views, and the choice between them.
///
/// For an action the enabled handlers compete: a handler of the view that holds the focus
/// wins, then the higher priority (an open thread over the main history beside it), then the
/// one registered last.
final class KeyboardRegistry {
  /// The registries of the shells that are built, the newest last.
  static final List<KeyboardRegistry> _shown = [];

  /// The registry of the newest shell that is built, or null (the menu bar of macOS sits above
  /// the router and runs its items here; during a route change the new shell is newest).
  static KeyboardRegistry? get current => _shown.isEmpty ? null : _shown.last;

  final List<_HandlerSource> _sources = [];
  int _serial = 0;

  int _nextSerial() => ++_serial;

  void _add(_HandlerSource source) => _sources.add(source);

  void _remove(_HandlerSource source) => _sources.remove(source);

  /// The handler that runs [action] with the focus on [focus], or null when nothing is
  /// enabled for it.
  KeyboardHandler? resolve(ShortcutAction action, FocusNode? focus) {
    _HandlerSource? best;
    var bestFocused = false;
    KeyboardHandler? chosen;
    for (final source in _sources) {
      final handler = source.handlers[action];
      if (handler == null || !handler.enabled) continue;
      final focused = source.holdsFocus(focus);
      if (handler.focusScoped && !focused) continue;
      final better =
          best == null ||
          (focused && !bestFocused) ||
          (focused == bestFocused &&
              (source.priority > best.priority ||
                  (source.priority == best.priority &&
                      source.serial > best.serial)));
      if (better) {
        best = source;
        bestFocused = focused;
        chosen = handler;
      }
    }
    return chosen;
  }

  /// Whether [action] has an enabled handler for the current focus.
  bool isEnabled(ShortcutAction action) =>
      resolve(action, FocusManager.instance.primaryFocus) != null;

  /// Runs [action] for the current focus; returns whether a handler ran.
  bool invoke(ShortcutAction action) {
    final handler = resolve(action, FocusManager.instance.primaryFocus);
    if (handler == null) return false;
    handler.onInvoke();
    return true;
  }
}

/// The keyboard layer of the shell for its descendants: the handler registry, the column
/// order and the platform of the bindings.
class KeyboardScope extends InheritedWidget {
  /// Creates the scope.
  const KeyboardScope({
    super.key,
    required this.registry,
    required this.columns,
    required this.platform,
    required super.child,
  });

  /// The handlers.
  final KeyboardRegistry registry;

  /// The columns for F6.
  final ColumnFocusRegistry columns;

  /// The keyboard family of the bindings.
  final ShortcutPlatform platform;

  /// The nearest scope, or null outside the shell (a view in a test host).
  static KeyboardScope? maybeOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<KeyboardScope>();

  /// A region in which only shortcuts with the platform modifier reach the app: Cmd on macOS,
  /// Ctrl+Shift on Windows (01 7 "In the terminal", R-PR-10). Every other key that [child]
  /// leaves unhandled goes to the platform, never to an app shortcut, so Ctrl+C, Ctrl+K,
  /// Alt+F, Esc and F6 stay with the terminal of M4-36.
  ///
  /// Example: `KeyboardScope.passThrough(child: terminalView)`.
  static Widget passThrough({Key? key, required Widget child}) =>
      _PassThrough(key: key, child: child);

  @override
  bool updateShouldNotify(KeyboardScope old) =>
      registry != old.registry ||
      columns != old.columns ||
      platform != old.platform;
}

class _PassThrough extends StatelessWidget {
  const _PassThrough({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final platform =
        KeyboardScope.maybeOf(context)?.platform ?? ShortcutPlatform.current;
    return Focus(
      canRequestFocus: false,
      skipTraversal: true,
      onKeyEvent: (node, event) {
        final keys = HardwareKeyboard.instance;
        final modifier = platform == ShortcutPlatform.mac
            ? keys.isMetaPressed
            : keys.isControlPressed && keys.isShiftPressed;
        return modifier
            ? KeyEventResult.ignored
            : KeyEventResult.skipRemainingHandlers;
      },
      child: child,
    );
  }
}

/// Offers [handlers] of a view to the shortcut layer while the view is built.
///
/// [priority] decides between two views that offer the same action and neither holds the
/// focus (the thread beside the main history in D2 wins with a higher value).
///
/// Example:
/// ```dart
/// KeyboardActions(
///   handlers: {ShortcutAction.toggleDone: KeyboardHandler(done, enabled: canSetDone)},
///   child: column,
/// )
/// ```
class KeyboardActions extends StatefulWidget {
  /// Creates the region.
  const KeyboardActions({
    super.key,
    required this.handlers,
    this.priority = 0,
    required this.child,
  });

  /// The handlers by action.
  final Map<ShortcutAction, KeyboardHandler> handlers;

  /// The priority against other views.
  final int priority;

  /// The view.
  final Widget child;

  @override
  State<KeyboardActions> createState() => _KeyboardActionsState();
}

class _KeyboardActionsState extends State<KeyboardActions>
    implements _HandlerSource {
  final FocusNode _node = FocusNode(
    debugLabel: 'keyboardActions',
    canRequestFocus: false,
    skipTraversal: true,
  );
  KeyboardRegistry? _registry;
  int _serial = 0;

  @override
  Map<ShortcutAction, KeyboardHandler> get handlers => widget.handlers;

  @override
  int get priority => widget.priority;

  @override
  int get serial => _serial;

  @override
  bool holdsFocus(FocusNode? focus) =>
      focus != null &&
      (identical(focus, _node) || focus.ancestors.contains(_node));

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final registry = KeyboardScope.maybeOf(context)?.registry;
    if (!identical(registry, _registry)) {
      _registry?._remove(this);
      _registry = registry;
      _serial = registry?._nextSerial() ?? 0;
      registry?._add(this);
    }
  }

  @override
  void dispose() {
    _registry?._remove(this);
    _node.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) =>
      Focus(focusNode: _node, child: widget.child);
}

/// The shell's handlers, registered below every view.
final class _ShellSource implements _HandlerSource {
  _ShellSource(this.handlers);

  @override
  Map<ShortcutAction, KeyboardHandler> handlers;

  @override
  int get priority => -1;

  @override
  int get serial => 0;

  @override
  bool holdsFocus(FocusNode? focus) => false;
}

/// The intents of the app bindings run through [registry].
class _RegistryAction extends Action<AppIntent> {
  _RegistryAction(this.registry);

  final KeyboardRegistry registry;

  @override
  bool isEnabled(AppIntent intent) => registry.isEnabled(intent.action);

  @override
  Object? invoke(AppIntent intent) {
    registry.invoke(intent.action);
    return null;
  }
}

/// The shortcut layer above the columns of the shell.
///
/// Binds every app binding of `bindings.dart` for [platform] (the running platform when
/// null), offers [handlers] for the actions of the shell (quick switcher, search, views,
/// settings, channels, the session column), F6 and Shift+F6 over the columns, and the list of
/// shortcuts (Cmd+/ or Ctrl+/). A focus node at the top takes the focus when the route shows
/// and nothing else has it, so the keys reach the layer from the first frame.
class AppShortcuts extends StatefulWidget {
  /// Creates the layer.
  const AppShortcuts({
    super.key,
    this.handlers = const {},
    this.platform,
    required this.child,
  });

  /// The shell's handlers.
  final Map<ShortcutAction, KeyboardHandler> handlers;

  /// The keyboard family, or null for the running platform.
  final ShortcutPlatform? platform;

  /// The shell.
  final Widget child;

  @override
  State<AppShortcuts> createState() => _AppShortcutsState();
}

class _AppShortcutsState extends State<AppShortcuts> {
  final KeyboardRegistry _registry = KeyboardRegistry();
  final ColumnFocusRegistry _columns = ColumnFocusRegistry();
  late final _ShellSource _shell = _ShellSource(_shellHandlers());
  late final Map<Type, Action<Intent>> _actions = {
    for (final action in ShortcutAction.values)
      intentOf(action).runtimeType: _RegistryAction(_registry),
  };
  final FocusNode _root = FocusNode(debugLabel: 'appShortcuts');

  ShortcutPlatform get _platform => widget.platform ?? ShortcutPlatform.current;

  Map<ShortcutAction, KeyboardHandler> _shellHandlers() => {
    ShortcutAction.nextColumn: KeyboardHandler(
      () => _columns.move(forward: true),
    ),
    ShortcutAction.previousColumn: KeyboardHandler(
      () => _columns.move(forward: false),
    ),
    ShortcutAction.shortcutList: KeyboardHandler(
      () => unawaited(showShortcutList(context, _platform)),
    ),
    ...widget.handlers,
  };

  @override
  void initState() {
    super.initState();
    _registry._add(_shell);
    KeyboardRegistry._shown.add(_registry);
  }

  @override
  void didUpdateWidget(AppShortcuts old) {
    super.didUpdateWidget(old);
    _shell.handlers = _shellHandlers();
  }

  @override
  void dispose() {
    KeyboardRegistry._shown.remove(_registry);
    _root.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => KeyboardScope(
    registry: _registry,
    columns: _columns,
    platform: _platform,
    child: Shortcuts(
      shortcuts: appActivators(_platform),
      child: Actions(
        actions: _actions,
        child: Focus(
          focusNode: _root,
          autofocus: true,
          skipTraversal: true,
          child: widget.child,
        ),
      ),
    ),
  );
}

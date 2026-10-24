// SPDX-License-Identifier: Apache-2.0

/// The option suggestions of a command from its agent (09 10.5; 04 15.7; components.md 9;
/// brief M4-21 Design note 4; docs/decisions.d/M4-21.md D-M4-21-4): a request waits
/// `ui.autocomplete_debounce` without typing, a newer request cancels the older one, and the
/// list shows at most what the server returned.
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as wire;
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter/foundation.dart';

/// One question to the agent: the option [option] of [command] in [channel] (and [thread])
/// with the typed [value] and the values of the other options.
typedef AutocompleteRequest = ({
  ChannelId channel,
  CommandId command,
  ThreadId? thread,
  String option,
  String value,
  List<({String name, String value})> options,
});

/// Fetches the suggestions of [request]; [cancel] is cancelled when a newer request starts.
typedef AutocompleteFetch = Future<List<CommandChoice>> Function(
  AutocompleteRequest request,
  CancelToken cancel,
);

/// The debounced, cancelling client of `autocompleteCommand`.
///
/// [query] restarts the wait of [debounce]; when it passes without a newer [query], the
/// request goes out and every older one still in flight is cancelled, so a late answer of an
/// older request never replaces a newer one. [choices] holds the newest answer; a failed or
/// cancelled request leaves it empty. [clear] stops everything.
///
/// Example:
/// ```dart
/// final client = AutocompleteClient(debounce: config.autocompleteDebounce,
///     fetch: AutocompleteClient.api(api));
/// client.query(request);
/// ```
final class AutocompleteClient extends ChangeNotifier {
  /// Creates the client.
  AutocompleteClient({required this.debounce, required this.fetch});

  /// The fetch of `autocompleteCommand` through [api].
  static AutocompleteFetch api(wire.ApiClient api) => (request, cancel) async {
    final response = await api.getCommandsApi().autocompleteCommand(
      channelId: request.channel.value,
      commandId: request.command.value,
      autocompleteQuery: wire.AutocompleteQuery(
        (b) => b
          ..option = request.option
          ..value = request.value
          ..threadId = request.thread?.value
          ..options.addAll([
            for (final o in request.options)
              wire.CommandOptionValue(
                (v) => v
                  ..name = o.name
                  ..value = o.value,
              ),
          ]),
      ),
      cancelToken: cancel,
    );
    return [
      for (final c in response.data?.choices ?? const <wire.CommandChoice>[])
        CommandChoice(name: c.name, value: c.value),
    ];
  };

  /// `ui.autocomplete_debounce`.
  final Duration debounce;

  /// The request to the server.
  final AutocompleteFetch fetch;

  /// The suggestions of the newest answered request.
  List<CommandChoice> get choices => _choices;
  List<CommandChoice> _choices = const [];

  Timer? _wait;
  CancelToken? _running;
  String? _last;
  int _generation = 0;
  bool _disposed = false;

  /// Asks for the suggestions of [request] after [debounce]; an equal request to the last
  /// one does nothing.
  void query(AutocompleteRequest request) {
    // Compared by value: the record holds a list, whose `==` is identity, and every build of
    // the composer makes a new one; an identity check would restart the wait on every frame
    // and ask the agent again after every answer.
    final key = _keyOf(request);
    if (_disposed || _last == key) return;
    _last = key;
    _wait?.cancel();
    _wait = Timer(debounce, () => unawaited(_send(request)));
  }

  /// Stops the wait and the request in flight and empties [choices].
  void clear() {
    _wait?.cancel();
    _wait = null;
    _running?.cancel();
    _running = null;
    _last = null;
    _generation++;
    if (_choices.isNotEmpty) {
      _choices = const [];
      if (!_disposed) notifyListeners();
    }
  }

  Future<void> _send(AutocompleteRequest request) async {
    _running?.cancel();
    final cancel = _running = CancelToken();
    final mine = ++_generation;
    List<CommandChoice> result;
    try {
      result = await fetch(request, cancel);
    } on Object {
      result = const [];
    }
    if (_disposed || mine != _generation || cancel.isCancelled) return;
    _running = null;
    _choices = List.unmodifiable(result);
    notifyListeners();
  }

  static String _keyOf(AutocompleteRequest r) => [
    r.channel.value,
    r.command.value,
    r.thread?.value ?? '',
    r.option,
    r.value,
    for (final o in r.options) '${o.name}=${o.value}',
  ].join('\u0000');

  @override
  void dispose() {
    _disposed = true;
    _wait?.cancel();
    _running?.cancel();
    super.dispose();
  }
}

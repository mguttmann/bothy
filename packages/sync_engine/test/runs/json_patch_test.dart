// SPDX-License-Identifier: Apache-2.0

// applyRunPatch: RFC 6902 add, replace and remove with RFC 6901 pointers, copy on write.

import 'dart:convert';

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

Map<String, Object?> _op(String op, String path, [Object? value = _none]) => {
  'op': op,
  'path': path,
  if (!identical(value, _none)) 'value': value,
};

const Object _none = Object();

Matcher _patchError(String fragment) => throwsA(
  isA<RunPatchError>().having((e) => e.message, 'message', contains(fragment)),
);

void main() {
  group('documented paths', () {
    test('replace and remove /status_line', () {
      final base = doc(1);
      final replaced = applyRunPatch(base, [
        _op('replace', '/status_line', 'Done'),
      ]);
      expect(replaced['status_line'], 'Done');
      final removed = applyRunPatch(base, [_op('remove', '/status_line')]);
      expect(removed.containsKey('status_line'), isFalse);
    });

    test('add /status_line when absent', () {
      final base = doc(4);
      final added = applyRunPatch(base, [_op('add', '/status_line', 'Again')]);
      expect(added['status_line'], 'Again');
    });

    test('replace /metrics/context_pct', () {
      final out = applyRunPatch(doc(1), [
        _op('replace', '/metrics/context_pct', 50.5),
      ]);
      expect((out['metrics']! as Map)['context_pct'], 50.5);
    });

    test('add /steps/<id> and replace /steps/<id>/status', () {
      final out = applyRunPatch(doc(1), [
        _op('add', '/steps/$stepA2', {'id': stepA2, 'status': 'running'}),
        _op('replace', '/steps/$stepA2/status', 'completed'),
      ]);
      final steps = out['steps']! as Map;
      expect((steps[stepA2]! as Map)['status'], 'completed');
      expect(steps.containsKey(stepA1), isTrue);
    });

    test('replace /followups/<id>/state', () {
      final out = applyRunPatch(doc(1), [
        _op('replace', '/followups/$followupB1/state', 'delivered'),
      ]);
      expect(
        ((out['followups']! as Map)[followupB1]! as Map)['state'],
        'delivered',
      );
    });

    test('add, replace and remove /plan', () {
      final plan = {
        'items': [
          {'id': 'step-1', 'text': 'Reproduce', 'status': 'pending'},
        ],
      };
      final added = applyRunPatch(doc(1), [_op('add', '/plan', plan)]);
      expect(added['plan'], plan);
      final replaced = applyRunPatch(added, [
        _op('replace', '/plan', {'items': <Object?>[]}),
      ]);
      expect(replaced['plan'], {'items': <Object?>[]});
      final removed = applyRunPatch(replaced, [_op('remove', '/plan')]);
      expect(removed.containsKey('plan'), isFalse);
    });
  });

  group('pointers', () {
    test('~1 decodes to / and ~0 to ~, in that order', () {
      final out = applyRunPatch(
        {'a/b': 1, '~1': 2, 'm~n': 3},
        [
          _op('replace', '/a~1b', 10),
          _op('replace', '/~01', 20),
          _op('replace', '/m~0n', 30),
        ],
      );
      expect(out, {'a/b': 10, '~1': 20, 'm~n': 30});
    });

    test('an invalid escape is an error', () {
      expect(
        () => applyRunPatch({'a': 1}, [_op('replace', '/~2', 1)]),
        _patchError('escape'),
      );
    });

    test('a path without a leading slash is an error', () {
      expect(
        () => applyRunPatch({'a': 1}, [_op('replace', 'a', 1)]),
        _patchError('pointer'),
      );
    });

    test('the root pointer is an error', () {
      expect(
        () => applyRunPatch({'a': 1}, [_op('replace', '', 1)]),
        _patchError('root'),
      );
    });

    test('a path through a non-container is an error', () {
      expect(
        () => applyRunPatch(doc(1), [_op('add', '/status_line/x', 1)]),
        _patchError('container'),
      );
    });

    test('a missing intermediate member is an error', () {
      expect(
        () => applyRunPatch(doc(1), [_op('add', '/plan/items', <Object?>[])]),
        _patchError('missing'),
      );
    });
  });

  group('arrays (RFC 6902 4.1 to 4.3)', () {
    Map<String, Object?> plan() => {
      'plan': {
        'items': [
          {'id': 'a'},
          {'id': 'b'},
        ],
      },
    };

    List<Object?> items(Map<String, Object?> d) =>
        (d['plan']! as Map)['items']! as List<Object?>;

    test('add with - appends, add with an index inserts', () {
      final appended = applyRunPatch(plan(), [
        _op('add', '/plan/items/-', {'id': 'c'}),
      ]);
      expect(items(appended).map((e) => (e! as Map)['id']), ['a', 'b', 'c']);
      final inserted = applyRunPatch(plan(), [
        _op('add', '/plan/items/0', {'id': 'z'}),
      ]);
      expect(items(inserted).map((e) => (e! as Map)['id']), ['z', 'a', 'b']);
    });

    test('replace and remove by index, and into an element', () {
      final out = applyRunPatch(plan(), [
        _op('replace', '/plan/items/1', {'id': 'B'}),
        _op('replace', '/plan/items/0/id', 'A'),
        _op('remove', '/plan/items/1'),
      ]);
      expect(items(out), [
        {'id': 'A'},
      ]);
    });

    test('bad indexes are errors', () {
      for (final path in [
        '/plan/items/2',
        '/plan/items/01',
        '/plan/items/x',
        '/plan/items/-1',
        '/plan/items/99999999999999999999999',
        '/plan/items/5/id',
      ]) {
        expect(
          () => applyRunPatch(plan(), [_op('replace', path, 1)]),
          throwsA(isA<RunPatchError>()),
          reason: path,
        );
      }
      expect(
        () => applyRunPatch(plan(), [_op('add', '/plan/items/3', 1)]),
        throwsA(isA<RunPatchError>()),
      );
      expect(
        () => applyRunPatch(plan(), [_op('remove', '/plan/items/-')]),
        throwsA(isA<RunPatchError>()),
      );
    });
  });

  group('rejected operations', () {
    for (final op in ['move', 'copy', 'test', 'merge']) {
      test('$op throws RunPatchError', () {
        final ops = [
          {'op': op, 'path': '/status_line', 'from': '/model', 'value': 'x'},
        ];
        expect(() => applyRunPatch(doc(1), ops), _patchError('operation'));
      });
    }

    test('replace and remove of a missing member throw', () {
      expect(
        () => applyRunPatch(doc(4), [_op('replace', '/status_line', 'x')]),
        _patchError('missing'),
      );
      expect(
        () => applyRunPatch(doc(4), [_op('remove', '/status_line')]),
        _patchError('missing'),
      );
    });

    test('add and replace without a value throw', () {
      expect(
        () => applyRunPatch(doc(1), [_op('add', '/model')]),
        _patchError('value'),
      );
      expect(
        () => applyRunPatch(doc(1), [_op('replace', '/model')]),
        _patchError('value'),
      );
    });

    test('a non-text op or path throws', () {
      expect(
        () => applyRunPatch(doc(1), [
          {'op': 1, 'path': '/model', 'value': 'x'},
        ]),
        throwsA(isA<RunPatchError>()),
      );
      expect(
        () => applyRunPatch(doc(1), [
          {'op': 'add', 'path': 1, 'value': 'x'},
        ]),
        throwsA(isA<RunPatchError>()),
      );
    });

    test('the error names the index of the failing operation', () {
      final error = _catch(
        () => applyRunPatch(doc(1), [
          _op('replace', '/model', 'm'),
          _op('copy', '/model'),
        ]),
      );
      expect(error.index, 1);
      expect(error.toString(), contains('1'));
    });

    test(
      'a store treats a rejected patch as a gap and never applies a part',
      () async {
        final (store, fetcher, _) = await focusedStore();
        store.onRunUpdated({
          'run_id': runId.value,
          'base_version': 1,
          'version': 2,
          'ops': [
            _op('replace', '/status_line', 'half'),
            {'op': 'move', 'from': '/model', 'path': '/status_line'},
          ],
        });
        expect(fetcher.calls, 2);
        expect(store.documentOf(runId)!.raw, doc(1));
        await fetcher.respond(doc(2));
        expect(store.documentOf(runId)!.raw, doc(2));
      },
    );
  });

  group('copy on write', () {
    test('the input map is never changed', () {
      final input = doc(1);
      final before = jsonEncode(input);
      final ops = patch(1)['ops']! as List<Object?>;
      applyRunPatch(input, ops.cast<Map<String, Object?>>());
      expect(jsonEncode(input), before);
    });

    test('untouched branches are shared, touched ones are new', () {
      final input = doc(1);
      final out = applyRunPatch(input, [
        _op('replace', '/metrics/context_pct', 1),
      ]);
      expect(identical(out, input), isFalse);
      expect(identical(out['metrics'], input['metrics']), isFalse);
      expect(identical(out['steps'], input['steps']), isTrue);
      expect(identical(out['cost'], input['cost']), isTrue);
    });

    test('a value added by a patch is not changed by a later op on it', () {
      final value = <String, Object?>{'id': stepA2, 'status': 'running'};
      final out = applyRunPatch(doc(1), [
        _op('add', '/steps/$stepA2', value),
        _op('replace', '/steps/$stepA2/status', 'completed'),
      ]);
      expect(value['status'], 'running');
      expect(((out['steps']! as Map)[stepA2]! as Map)['status'], 'completed');
    });

    test('an empty list of ops returns a new equal map', () {
      final input = doc(1);
      final out = applyRunPatch(input, const []);
      expect(identical(out, input), isFalse);
      expect(out, input);
    });
  });
}

RunPatchError _catch(void Function() body) {
  try {
    body();
  } on RunPatchError catch (e) {
    return e;
  }
  fail('no RunPatchError');
}

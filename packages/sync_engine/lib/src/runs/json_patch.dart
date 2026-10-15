// SPDX-License-Identifier: Apache-2.0

/// The patch applier of the live box: RFC 6902 restricted to `add`, `replace` and `remove`
/// (04 11.2, 09 5.3), with RFC 6901 JSON Pointers into the run document.
library;

/// A patch that cannot be applied: an operation other than `add`, `replace` or `remove`, a
/// missing target for `replace` or `remove`, a missing `value`, a malformed pointer or a path
/// through a value that is no object or array.
///
/// The store treats it like a gap and refetches the whole document; a patch is never applied
/// in part (docs/DECISIONS.md D-M4-16-2).
final class RunPatchError implements Exception {
  /// Creates the error for the operation at [index] of the list.
  const RunPatchError(this.index, this.message);

  /// Position of the failing operation in the list of ops.
  final int index;

  /// What is wrong; names the pointer or the operation, never a value of the document.
  final String message;

  @override
  String toString() => 'RunPatchError: operation $index: $message';
}

/// Applies [ops] to [doc] in order and returns the patched document.
///
/// [doc] is never changed. The result is always a new root map; below it only the branches an
/// operation touches are copied (copy on write), every other branch is shared with [doc], so
/// a patch on `/steps/<id>/status` copies the root, `steps` and that step, not the document.
/// Values that an operation inserts are not copied either; a later operation that writes into
/// such a value copies it first, so the caller's maps stay unchanged too.
///
/// Semantics (RFC 6902): `add` creates or replaces an object member and inserts into an array
/// at an index from 0 to its length, or appends with `-` (4.1); `replace` requires the target
/// to exist (4.3); `remove` requires it to exist (4.2). Pointer tokens decode `~1` to `/`
/// and then `~0` to `~` (RFC 6901 4).
///
/// Throws [RunPatchError] for any other `op`, a missing target of `replace` or `remove`, a
/// missing parent, an `add` or `replace` without `value`, a pointer that is empty, has no
/// leading `/` or holds an invalid escape, and a path through a non-container. When it
/// throws, nothing of the patch is visible to the caller.
///
/// Example:
/// ```dart
/// final next = applyRunPatch(doc, [
///   {'op': 'replace', 'path': '/steps/0192f4a0-7b1c-7000-8a00-0000000000a1/status',
///    'value': 'completed'},
/// ]);
/// ```
Map<String, Object?> applyRunPatch(
  Map<String, Object?> doc,
  List<Map<String, Object?>> ops,
) {
  final patch = _Patch(doc);
  for (var i = 0; i < ops.length; i++) {
    patch.apply(i, ops[i]);
  }
  return patch.root;
}

/// One application of a patch: the containers created during it may be written in place,
/// everything else is copied before the first write.
final class _Patch {
  _Patch(Map<String, Object?> doc) : root = Map<String, Object?>.of(doc) {
    _owned.add(root);
  }

  Map<String, Object?> root;

  // Identity set: two equal but distinct maps must not count as one.
  final Set<Object> _owned = Set<Object>.identity();

  void apply(int index, Map<String, Object?> op) {
    final kind = op['op'];
    final path = op['path'];
    if (kind is! String) throw RunPatchError(index, 'operation is not a text');
    if (path is! String) throw RunPatchError(index, 'pointer is not a text');
    if (kind != 'add' && kind != 'replace' && kind != 'remove') {
      throw RunPatchError(index, 'operation "$kind" is not allowed');
    }
    if (kind != 'remove' && !op.containsKey('value')) {
      throw RunPatchError(index, '$kind without value at "$path"');
    }
    final tokens = _tokens(index, path);
    final parent = _parentFor(index, path, tokens);
    final last = tokens.last;
    if (parent is Map<String, Object?>) {
      _onObject(index, kind, path, parent, last, op['value']);
    } else {
      _onArray(index, kind, path, parent as List<Object?>, last, op['value']);
    }
  }

  void _onObject(
    int index,
    String kind,
    String path,
    Map<String, Object?> parent,
    String key,
    Object? value,
  ) {
    switch (kind) {
      case 'add':
        parent[key] = value;
      case 'replace':
        if (!parent.containsKey(key)) {
          throw RunPatchError(index, 'replace of a missing member "$path"');
        }
        parent[key] = value;
      default:
        if (!parent.containsKey(key)) {
          throw RunPatchError(index, 'remove of a missing member "$path"');
        }
        parent.remove(key);
    }
  }

  void _onArray(
    int index,
    String kind,
    String path,
    List<Object?> parent,
    String token,
    Object? value,
  ) {
    if (kind == 'add' && token == '-') {
      parent.add(value);
      return;
    }
    final at = _arrayIndex(index, path, token);
    final limit = kind == 'add' ? parent.length : parent.length - 1;
    if (at > limit) {
      throw RunPatchError(index, '$kind of a missing element "$path"');
    }
    switch (kind) {
      case 'add':
        parent.insert(at, value);
      case 'replace':
        parent[at] = value;
      default:
        parent.removeAt(at);
    }
  }

  /// Walks to the container that holds the last token, copying every container on the way
  /// that this patch does not own yet, and returns it (a map or a list owned by this patch).
  Object _parentFor(int index, String path, List<String> tokens) {
    Object current = root;
    for (var i = 0; i < tokens.length - 1; i++) {
      final token = tokens[i];
      final Object? child;
      if (current is Map<String, Object?>) {
        if (!current.containsKey(token)) {
          throw RunPatchError(index, 'missing parent of "$path"');
        }
        child = current[token];
        final owned = _own(index, path, child);
        current[token] = owned;
        current = owned;
      } else {
        final list = current as List<Object?>;
        final at = _arrayIndex(index, path, token);
        if (at >= list.length) {
          throw RunPatchError(index, 'missing parent of "$path"');
        }
        child = list[at];
        final owned = _own(index, path, child);
        list[at] = owned;
        current = owned;
      }
    }
    return current;
  }

  /// Returns [value] when this patch owns it, otherwise a shallow copy that it then owns.
  Object _own(int index, String path, Object? value) {
    if (value != null && _owned.contains(value)) return value;
    final Object copy;
    if (value is Map<String, Object?>) {
      copy = Map<String, Object?>.of(value);
    } else if (value is List<Object?>) {
      copy = List<Object?>.of(value);
    } else {
      throw RunPatchError(
        index,
        'path "$path" goes through a value that is no container',
      );
    }
    _owned.add(copy);
    return copy;
  }
}

/// Splits [path] into decoded reference tokens (RFC 6901 3 and 4).
List<String> _tokens(int index, String path) {
  if (path.isEmpty) {
    // The root pointer would replace the whole document; no documented path does that, and a
    // whole document arrives as a snapshot instead (04 11.3 item 3).
    throw RunPatchError(index, 'the root pointer is not a run document path');
  }
  if (!path.startsWith('/')) {
    throw RunPatchError(index, 'pointer "$path" does not start with "/"');
  }
  return [
    for (final raw in path.substring(1).split('/')) _decode(index, path, raw),
  ];
}

final RegExp _badEscape = RegExp('~(?![01])');

String _decode(int index, String path, String raw) {
  if (_badEscape.hasMatch(raw)) {
    throw RunPatchError(index, 'pointer "$path" holds an invalid escape');
  }
  // ~1 first, then ~0: "~01" must become "~1", not "/" (RFC 6901 4).
  return raw.replaceAll('~1', '/').replaceAll('~0', '~');
}

final RegExp _arrayIndexPattern = RegExp(r'^(0|[1-9][0-9]*)$');

int _arrayIndex(int index, String path, String token) {
  // tryParse: an index beyond the int range is a bad pointer, not a crash.
  final value = _arrayIndexPattern.hasMatch(token) ? int.tryParse(token) : null;
  if (value == null) {
    throw RunPatchError(
      index,
      'pointer "$path" needs an array index, got "$token"',
    );
  }
  return value;
}

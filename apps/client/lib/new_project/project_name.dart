// SPDX-License-Identifier: Apache-2.0

/// The name suggestion and the field checks of the new project dialog (01 6.9 item 2;
/// docs/decisions.d/M4-43.md D-M4-43-3 and D-M4-43-10).
///
/// The server and the runner check the same rules again (03 `project_bindings`, 09 18.12); the
/// dialog checks them first only so the person sees a field error before anything is created.
library;

final RegExp _name = RegExp(r'^[A-Za-z0-9][A-Za-z0-9._-]*$');
final RegExp _separators = RegExp(r'[^a-z0-9]+');
final RegExp _controls = RegExp(r'[\x00-\x1f\x7f]');
const Map<String, String> _folds = {'ä': 'ae', 'ö': 'oe', 'ü': 'ue', 'ß': 'ss'};

/// The number of words of an idea the suggestion takes.
const int _ideaWords = 4;

/// Suggests a project name from the last path segment of [url] without `.git`, or else from
/// the first words of [idea]: lower case, German umlauts folded, every other run of characters
/// outside `a-z0-9` one hyphen, at most [maxChars] (`ui.new_project_name_max_chars`), cut at a
/// word boundary and without a trailing separator. Returns '' when nothing usable remains; the person types a name then.
///
/// Example: `suggestProjectName(url: 'https://git.example.com/team/website.git', maxChars: 40)`
/// is `website`.
String suggestProjectName({String? url, String? idea, required int maxChars}) {
  String source;
  if (url != null && url.trim().isNotEmpty) {
    final uri = Uri.tryParse(url.trim());
    final segments = (uri?.hasAuthority ?? false)
        ? uri!.pathSegments.where((s) => s.isNotEmpty).toList()
        : url.trim().split('/').where((s) => s.isNotEmpty).toList();
    source = segments.isEmpty ? '' : segments.last;
    if (source.toLowerCase().endsWith('.git')) {
      source = source.substring(0, source.length - 4);
    }
  } else {
    final words = _slug(idea ?? '').split('-').where((w) => w.isNotEmpty);
    source = words.take(_ideaWords).join('-');
  }
  var slug = _slug(source);
  if (slug.length > maxChars) {
    // Cut at the last word boundary, so no part of a word is left at the end.
    final cut = slug.substring(0, maxChars);
    final boundary = slug[maxChars] == '-' ? maxChars : cut.lastIndexOf('-');
    slug = boundary > 0 ? cut.substring(0, boundary) : cut;
  }
  return slug.replaceAll(RegExp(r'[-._]+$'), '');
}

String _slug(String text) {
  final buffer = StringBuffer();
  for (final rune in text.toLowerCase().runes) {
    final c = String.fromCharCode(rune);
    buffer.write(_folds[c] ?? c);
  }
  return buffer
      .toString()
      .replaceAll(_separators, '-')
      .replaceAll(RegExp(r'^-+|-+$'), '');
}

/// Whether [value] is a project name: one segment of letters, digits, `.`, `_` and `-` that
/// starts with a letter or digit, at most [maxChars] characters (the forum and the folder of
/// `provision` `init` or `clone`).
bool isProjectName(String value, {required int maxChars}) =>
    value.length <= maxChars && _name.hasMatch(value);

/// Whether [value] is a URL the runner may clone: `https` with a host, no user information,
/// query or fragment, no white space (09 18.12 item 2).
bool isCloneUrl(String value) {
  if (value.isEmpty ||
      value.startsWith('-') ||
      value.contains(RegExp(r'\s')) ||
      value.contains('?') ||
      value.contains('#') ||
      _controls.hasMatch(value)) {
    return false;
  }
  final uri = Uri.tryParse(value);
  return uri != null &&
      uri.scheme == 'https' &&
      uri.host.isNotEmpty &&
      uri.userInfo.isEmpty;
}

/// Whether [value] names a folder under the runner's projects folder for an existing project
/// (`provision` `none`): not empty, no white space at either end, no leading `/`, `\`, `~` or
/// `-`, no backslash, no drive letter, no `..` segment, no control character (the rule of
/// 03 `project_bindings` `workspace_ref`).
bool isFolderRef(String value) {
  if (value.isEmpty ||
      value.trim() != value ||
      _controls.hasMatch(value) ||
      value.contains(r'\') ||
      RegExp(r'^[/~-]').hasMatch(value) ||
      RegExp(r'^[A-Za-z]:').hasMatch(value)) {
    return false;
  }
  return !value.split('/').contains('..');
}

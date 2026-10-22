// SPDX-License-Identifier: Apache-2.0

/// Display rules of the host key fingerprint (24 4.3, screen D11 of 01).
///
/// `key_fingerprint` is 52 Crockford base32 characters, sent as 13 groups of four joined with
/// `-` (24 4.3). The apps show it in groups of four separated by spaces: in full in the key
/// change dialog and as the first groups followed by ` ..` in the terminal header (01 D11,
/// "Fp 7KQ2 9XTA .."). These rules only format and compare; they do not check the alphabet or
/// the length, because the display must still show what the server sent.
library;

/// The number of groups of the short form in the terminal header (01 D11: "Fp 7KQ2 9XTA ..").
const int fingerprintShortGroups = 2;

/// The characters of a group (24 4.3: "displayed in groups of four characters").
const int _groupLength = 4;

/// The separators a fingerprint may carry: `-` of the wire form (24 4.3) and the space of the
/// display form.
final RegExp _separators = RegExp('[- ]');

/// [fingerprint] without separators, in upper case: the form the groups and the comparison
/// work on, so the wire form, the display form and typed input all agree.
String _normalized(String fingerprint) =>
    fingerprint.replaceAll(_separators, '').toUpperCase();

/// The groups of four characters of [fingerprint] after removing separators (hyphen, space)
/// and upper-casing.
///
/// Returns an empty list for an empty input; if the length is not divisible by four, the last
/// group is shorter. Never throws.
///
/// ```dart
/// fingerprintGroups('f0cc-6qs6-rf'); // ['F0CC', '6QS6', 'RF']
/// ```
List<String> fingerprintGroups(String fingerprint) {
  final s = _normalized(fingerprint);
  return [
    for (var i = 0; i < s.length; i += _groupLength)
      s.substring(i, i + _groupLength > s.length ? s.length : i + _groupLength),
  ];
}

/// The display form of [fingerprint]: its groups ([fingerprintGroups]) joined by one space, as
/// in the key change dialog (24 4.3, 01 D11).
///
/// Returns an empty string for an empty input. Never throws.
///
/// ```dart
/// fingerprintDisplay('F0CC-6QS6-RF5D'); // 'F0CC 6QS6 RF5D'
/// ```
String fingerprintDisplay(String fingerprint) =>
    fingerprintGroups(fingerprint).join(' ');

/// The short form of the terminal header (01 D11): the first [groups] groups of [fingerprint]
/// joined by one space, followed by ` ..` when groups were cut off.
///
/// When the fingerprint has no more than [groups] groups, the whole display form is returned
/// without ` ..`, because nothing is hidden. Throws an [ArgumentError] if [groups] is less
/// than one, since a short form without a group would not identify the key.
///
/// ```dart
/// fingerprintShort('F0CC-6QS6-RF5D-3BSH'); // 'F0CC 6QS6 ..'
/// fingerprintShort('F0CC-6QS6'); // 'F0CC 6QS6'
/// ```
String fingerprintShort(
  String fingerprint, {
  int groups = fingerprintShortGroups,
}) {
  if (groups < 1) {
    throw ArgumentError.value(groups, 'groups', 'must be at least 1');
  }
  final all = fingerprintGroups(fingerprint);
  if (all.length <= groups) {
    return all.join(' ');
  }
  return '${all.take(groups).join(' ')} ..';
}

/// Whether [a] and [b] are the same key, ignoring separators (hyphen, space) and case.
///
/// The key change dialog compares the stored with the offered fingerprint, which may come in
/// the wire form (`-`) or the display form (space) (24 4.3). Never throws.
///
/// ```dart
/// fingerprintsEqual('F0CC-6QS6', 'f0cc 6qs6'); // true
/// ```
bool fingerprintsEqual(String a, String b) => _normalized(a) == _normalized(b);

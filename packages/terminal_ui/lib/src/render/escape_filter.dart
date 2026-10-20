// SPDX-License-Identifier: Apache-2.0

/// The escape sequence filter in front of the renderer (docs/spec/24-terminal.md 11.2,
/// docs/spec/15-client-architektur.md 23.4, threat SEC-TERM-6, test TM-T14).
///
/// The locks live here and not in the renderer so that they do not depend on its version
/// (15 23.4). The filter is pure Dart: no Flutter, no xterm.
library;

import 'dart:convert';
import 'dart:typed_data';

/// One piece of filtered output, in the order of the host output.
///
/// Returned by [EscapeFilter.process]; the view hands [FilteredText] to the renderer and keeps
/// [LinkStart] and [LinkEnd] as the record of an inert hyperlink (24 11.2: a link opens only
/// after a click with the target shown and a confirmation).
sealed class FilteredChunk {
  /// Base constructor of the closed set of chunks.
  const FilteredChunk();
}

/// Bytes to hand to the renderer unchanged.
///
/// UTF-8 is untouched: a multibyte character may even be cut at the end of [bytes] when the
/// frame cut it, so the renderer must decode with state or per joined chunk.
final class FilteredText extends FilteredChunk {
  /// Wraps [bytes]; the filter never hands out an empty one.
  const FilteredText(this.bytes);

  /// The allowed bytes, text and sequences, in host order.
  final Uint8List bytes;
}

/// An OSC 8 sequence opened a hyperlink to [target].
///
/// The visible link text follows as [FilteredText] until the matching [LinkEnd].
final class LinkStart extends FilteredChunk {
  /// Records the opening of a link to [target].
  const LinkStart(this.target);

  /// The target URI: non-empty, printable ASCII only (no space, no control, no non-ASCII
  /// character), at most [EscapeFilter.maxLinkTargetBytes] bytes long. Not checked for a scheme: the view shows it
  /// and asks before opening (24 11.2).
  final String target;
}

/// An OSC 8 sequence with an empty target closed the open hyperlink.
final class LinkEnd extends FilteredChunk {
  /// Records the end of the open link.
  const LinkEnd();
}

/// Filters host output so that nothing reaches the renderer that leaves the device, sends data
/// back or changes the local view on its own (24 11.2, SEC-TERM-6).
///
/// The filter works on bytes, before UTF-8 decoding: ESC and every byte of a control sequence
/// are ASCII and never part of a UTF-8 multibyte character, so the byte view and the decoded
/// view of the renderer see the same sequences.
///
/// Allow list (ruling of M4-35, Design note 2):
///
/// 1. Bytes outside escape sequences pass unchanged: printable text, UTF-8 multibyte, the C0
///    controls (BEL, BS, HT, LF, VT, FF, CR, SO, SI; the others too, xterm ignores them), DEL,
///    and CAN and SUB when no sequence is open. 8-bit C1 bytes are not interpreted: the
///    renderer works on decoded text and xterm does not parse C1.
/// 2. ESC, intermediates 0x20-0x2F and one final byte 0x30-0x7E pass (ESC 7, ESC 8, ESC D,
///    ESC M, ESC ( B, ESC =, ESC > ...), EXCEPT the string introducers ESC P (DCS, among them
///    DECRQSS and XTGETTCAP), ESC X (SOS), ESC ^ (PM) and ESC _ (APC): these are dropped
///    completely with their content up to ST (ESC \); BEL does not end them.
/// 3. CSI (ESC [, parameters 0x30-0x3F, intermediates 0x20-0x2F, final 0x40-0x7E) passes,
///    EXCEPT these queries and reports, dropped completely:
///    - final `t` (window operations and reports: CSI 21 t title, CSI 20 t icon title,
///      CSI 18 t size, CSI 14 t, and CSI 8;r;c t which would resize the local view);
///    - final `n` unless the parameters are exactly `6` without intermediates (CPR allowed;
///      CSI 5 n, CSI ?6 n and the others dropped);
///    - final `c` unless the parameters are ``, `0`, `>` or `>0` without intermediates (DA1 and
///      DA2 allowed; tertiary CSI = c dropped);
///    - any intermediate `$` (DECRQM CSI ? Ps $ p and the rectangle and report family);
///    - final `x` (DECREQTPARM);
///    - final `q` without intermediates and with a `>` prefix (XTVERSION); CSI Ps SP q
///      (cursor style) passes.
///    The answers that shells need are therefore exactly CPR (CSI 6 n) and DA1/DA2.
///    Intermediates count for `n` and `c` because xterm ignores them: CSI 5 SP n would be a
///    status report for it.
/// 3a. Renderer safety (found by the red team of M4-35): a CSI is also dropped when one of its
///    numeric parameters, read as xterm 4.0.0 reads them, exceeds [maxParameterValue] (REP
///    CSI 999999999 b would freeze the view), and an SGR whose 38 or 48 lacks its colour
///    sub-parameters (CSI 38 m, CSI 48;2 m) is dropped, because xterm reads them without a
///    bounds check and throws; so is SGR 38;5 or 48;5 with an index above 255 (its painter
///    reads past the palette), REP CSI Ps b with Ps above [maxRepeatCount] and ECH CSI 0 X
///    (xterm erases from column -1 and throws). These are the renderer faults known for
///    xterm 4.0.0, not a complete guard: the view must still catch exceptions of the renderer
///    and process large output in slices.
/// 4. OSC (ESC ], ended by BEL or ST) is dropped completely: OSC 52 clipboard set and query,
///    OSC 0/1/2 titles, OSC 4/10/11/12 colours with `?` queries, any other. EXCEPT OSC 8,
///    which is rewritten: ESC ] 8 ; params ; URI ST with a non-empty URI emits [LinkStart]
///    (params such as `id=` are ignored), with an empty URI [LinkEnd] (only while a link is
///    open). The link text between them passes as [FilteredText], so the link is inert text
///    plus a record. A [LinkStart] while a link is open first emits [LinkEnd]. An OSC 8 whose
///    URI is longer than [maxLinkTargetBytes] or holds anything but printable ASCII (a space,
///    a control, a line separator, a bidi control or any non-ASCII character could hide or
///    disguise the shown target) is dropped like any other OSC and emits nothing; an open
///    link then stays open. OSC 8 without the second `;` emits nothing.
/// 5. CAN (0x18) or SUB (0x1A) inside a sequence aborts it; the sequence and the CAN or SUB
///    byte are dropped. An ESC inside a sequence that is not ST aborts the sequence and starts
///    a new one at that ESC. Further rules of the VT500 parser that keep the byte view equal
///    to the renderer's view: a C0 control inside an ESC or CSI sequence is passed at once
///    (executed before the sequence) and the sequence goes on; DEL inside one is dropped; a
///    byte from 0x80 aborts it and is then handled as text; a parameter byte after an
///    intermediate makes the CSI invalid, it is dropped up to its final byte.
/// 6. Memory is bounded: an ESC or CSI sequence longer than [maxControlSequenceBytes] is
///    dropped up to its final byte; dropped string sequences (DCS, SOS, PM, APC, OSC other
///    than 8 and the params of OSC 8) are consumed without buffering; only the URI of OSC 8
///    is buffered, up to [maxLinkTargetBytes].
/// 7. Every sequence may be split at any byte across [process] calls; the result is the
///    same as in one call.
/// 8. The filter never throws, logs nothing and keeps no content beyond the current
///    sequence (15 23.4: terminal content never reaches logs).
///
/// Example:
/// ```dart
/// final filter = EscapeFilter();
/// for (final chunk in filter.process(frame)) {
///   switch (chunk) {
///     case FilteredText(:final bytes): render(bytes);
///     case LinkStart(:final target): openRecord(target);
///     case LinkEnd(): closeRecord();
///   }
/// }
/// ```
final class EscapeFilter {
  /// Creates a filter in the ground state with no open link.
  EscapeFilter();

  /// The longest ESC or CSI sequence that passes, in bytes including ESC and the final byte
  /// (rule 6); real sequences stay far below it, SGR with 24-bit colours is about 20.
  static const int maxControlSequenceBytes = 256;

  /// The longest OSC 8 target URI that is kept, in bytes (rule 4 and 6); a longer one is
  /// dropped and emits nothing.
  static const int maxLinkTargetBytes = 2048;

  /// The largest numeric CSI parameter that passes (rule 3a); screens, colours and modes stay
  /// below it, and xterm loops once per count for some sequences.
  static const int maxParameterValue = 9999;

  /// The largest REP (CSI Ps b) count that passes (rule 3a): one line of a wide screen. It
  /// bounds the work per sequence; it does not stop a flood of REP sequences.
  static const int maxRepeatCount = 512;

  static const int _bel = 0x07;
  static const int _can = 0x18;
  static const int _sub = 0x1a;
  static const int _esc = 0x1b;
  static const int _del = 0x7f;
  static const int _backslash = 0x5c;
  static const int _semicolon = 0x3b;
  static const int _digitEight = 0x38;
  static const int _dollar = 0x24;

  _State _state = _State.ground;
  _OscPart _oscPart = _OscPart.command;

  /// Set when an ESC arrived inside an OSC or a dropped string sequence: ST if a `\` follows.
  bool _stringEscape = false;

  /// The ESC or CSI sequence so far, including ESC.
  final List<int> _sequence = [];

  /// The number of parameter bytes of the CSI in [_sequence] after ESC [.
  int _csiParameterLength = 0;

  /// Whether the CSI in [_sequence] has reached its intermediates.
  bool _csiInIntermediates = false;

  /// The OSC 8 target so far.
  final BytesBuilder _target = BytesBuilder(copy: false);

  /// Whether the OSC 8 target holds a control character.
  bool _targetInvalid = false;

  bool _linkOpen = false;

  final BytesBuilder _text = BytesBuilder();
  final List<FilteredChunk> _chunks = [];

  /// Filters the next [bytes] of the host output.
  ///
  /// Keeps state across calls, so a sequence split across frames works (rule 7); a partial
  /// sequence at the end is held back until a later call completes or aborts it. Values
  /// outside 0-255 are taken modulo 256, as a [Uint8List] would store them.
  ///
  /// Returns the chunks in host order, adjacent text merged into one [FilteredText]; empty if
  /// nothing passed. Never throws.
  List<FilteredChunk> process(List<int> bytes) {
    for (final value in bytes) {
      _consume(value & 0xff);
    }
    _flushText();
    final result = List<FilteredChunk>.of(_chunks);
    _chunks.clear();
    return result;
  }

  /// Drops any partial sequence and the link state, without emitting anything.
  ///
  /// Used when the screen is replaced by a replay: the replay starts in the ground state with
  /// no open link.
  void reset() {
    _toGround();
    _linkOpen = false;
    _text.clear();
    _chunks.clear();
  }

  void _consume(int byte) {
    switch (_state) {
      case _State.ground:
        if (byte == _esc) {
          _startEscape();
        } else {
          _text.addByte(byte);
        }
      case _State.escape:
        _escapeByte(byte);
      case _State.escapeIntermediate:
      case _State.escapeIgnore:
        _escapeIntermediateByte(byte);
      case _State.csi:
      case _State.csiIgnore:
        _csiByte(byte);
      case _State.osc:
        _oscByte(byte);
      case _State.stringDrop:
        _stringDropByte(byte);
    }
  }

  void _startEscape() {
    _sequence
      ..clear()
      ..add(_esc);
    _state = _State.escape;
  }

  /// The first byte after ESC chooses the kind of sequence (rules 2, 3 and 4).
  void _escapeByte(int byte) {
    if (_isSequenceControl(byte)) return;
    switch (byte) {
      case 0x5b: // [
        _sequence.add(byte);
        _csiParameterLength = 0;
        _csiInIntermediates = false;
        _state = _State.csi;
      case 0x5d: // ]
        _sequence.clear();
        _oscPart = _OscPart.command;
        _stringEscape = false;
        _state = _State.osc;
      case 0x50 || 0x58 || 0x5e || 0x5f: // P X ^ _
        _sequence.clear();
        _stringEscape = false;
        _state = _State.stringDrop;
      default:
        _state = _State.escapeIntermediate;
        _escapeIntermediateByte(byte);
    }
  }

  void _escapeIntermediateByte(int byte) {
    if (_isSequenceControl(byte)) return;
    if (byte >= 0x20 && byte <= 0x2f) {
      _append(byte, ignore: _State.escapeIgnore);
    } else if (byte >= 0x30 && byte <= 0x7e) {
      if (_state == _State.escapeIntermediate) {
        _sequence.add(byte);
        _text.add(_sequence);
      }
      _toGround();
    }
  }

  void _csiByte(int byte) {
    if (_isSequenceControl(byte)) return;
    if (byte >= 0x30 && byte <= 0x3f) {
      if (_csiInIntermediates) {
        // ECMA-48 5.4: parameters come before intermediates; the VT500 parser ignores the
        // rest of such a CSI.
        _state = _State.csiIgnore;
        return;
      }
      _csiParameterLength++;
      _append(byte, ignore: _State.csiIgnore);
    } else if (byte >= 0x20 && byte <= 0x2f) {
      _csiInIntermediates = true;
      _append(byte, ignore: _State.csiIgnore);
    } else if (byte >= 0x40 && byte <= 0x7e) {
      if (_state == _State.csi && _csiAllowed(byte)) {
        _sequence.add(byte);
        _text.add(_sequence);
      }
      _toGround();
    }
  }

  /// Rule 3: whether the complete CSI in [_sequence] with [finalByte] may pass.
  bool _csiAllowed(int finalByte) {
    final parameterBytes = _sequence.sublist(2, 2 + _csiParameterLength);
    final parameters = String.fromCharCodes(parameterBytes);
    final intermediates = _sequence.sublist(2 + _csiParameterLength);
    final values = _rendererValues(parameterBytes);
    if (values == null) return false;
    if (!_rendererSafe(finalByte, values)) return false;
    if (intermediates.contains(_dollar)) return false;
    switch (finalByte) {
      case 0x74: // t
      case 0x78: // x
        return false;
      case 0x6e: // n
        return intermediates.isEmpty && parameters == '6';
      case 0x63: // c
        return intermediates.isEmpty &&
            const {'', '0', '>', '>0'}.contains(parameters);
      case 0x71: // q
        return intermediates.isNotEmpty || !parameters.startsWith('>');
      default:
        return true;
    }
  }

  /// The numeric parameters as xterm 4.0.0 reads them (its `_consumeCsi`): one prefix byte
  /// 0x3A-0x3F is skipped, other non-digits are ignored, an empty field counts as 0 once a
  /// digit was seen. Null when a value exceeds [maxParameterValue] (rule 3a).
  static List<int>? _rendererValues(List<int> parameterBytes) {
    final values = <int>[];
    var start = 0;
    if (parameterBytes.isNotEmpty &&
        parameterBytes.first >= 0x3a &&
        parameterBytes.first <= 0x3f) {
      start = 1;
    }
    var value = 0;
    var seenDigit = false;
    for (var i = start; i < parameterBytes.length; i++) {
      final byte = parameterBytes[i];
      if (byte == _semicolon) {
        if (seenDigit) values.add(value);
        value = 0;
      } else if (byte >= 0x30 && byte <= 0x39) {
        seenDigit = true;
        value = value * 10 + byte - 0x30;
        if (value > maxParameterValue) return null;
      }
    }
    if (seenDigit) values.add(value);
    return values;
  }

  /// Rule 3a: whether xterm 4.0.0 handles the CSI with [finalByte] and [values] without
  /// throwing or looping long. REP (`b`) repeats once per count; ECH (`X`) with 0 erases from
  /// column -1 at the line start (`_csiHandleEraseCharacters` has no 0-to-1 mapping); SGR
  /// (`m`) goes on in [_sgrSafe].
  static bool _rendererSafe(int finalByte, List<int> values) {
    final first = values.isEmpty ? null : values.first;
    switch (finalByte) {
      case 0x62: // b
        return first == null || first <= maxRepeatCount;
      case 0x58: // X
        return first != 0;
      case 0x6d: // m
        return _sgrSafe(values);
      default:
        return true;
    }
  }

  /// Whether every SGR 38 and 48 in [values] has the sub-parameters that xterm 4.0.0 reads
  /// without a bounds check (its `_csiHandleSgr`, same indexing): 5 needs one more, 2 three;
  /// a 256-colour index above 255 would make its painter read past the palette.
  static bool _sgrSafe(List<int> values) {
    for (var i = 0; i < values.length; i++) {
      if (values[i] != 38 && values[i] != 48) continue;
      if (i + 1 >= values.length) return false;
      switch (values[i + 1]) {
        case 2:
          if (i + 4 >= values.length) return false;
          i += 4;
        case 5:
          if (i + 2 >= values.length || values[i + 2] > 255) return false;
          i += 2;
      }
    }
    return true;
  }

  void _oscByte(int byte) {
    if (_stringEscape) {
      _stringEscape = false;
      if (byte == _backslash) {
        _finishOsc();
      } else {
        _abortToEscape(byte);
      }
      return;
    }
    switch (byte) {
      case _bel:
        _finishOsc();
        return;
      case _esc:
        _stringEscape = true;
        return;
      case _can || _sub:
        _toGround();
        return;
    }
    switch (_oscPart) {
      case _OscPart.command:
        _oscPart = byte == _digitEight ? _OscPart.sawEight : _OscPart.drop;
      case _OscPart.sawEight:
        _oscPart = byte == _semicolon ? _OscPart.params : _OscPart.drop;
      case _OscPart.params:
        if (byte == _semicolon) {
          _target.clear();
          _targetInvalid = false;
          _oscPart = _OscPart.target;
        }
      case _OscPart.target:
        if (_target.length >= maxLinkTargetBytes) {
          _target.clear();
          _oscPart = _OscPart.drop;
        } else {
          if (byte < 0x20 || byte == _del) _targetInvalid = true;
          _target.addByte(byte);
        }
      case _OscPart.drop:
        break;
    }
  }

  /// Rule 4: a complete OSC; only a valid OSC 8 leaves a trace.
  void _finishOsc() {
    if (_oscPart == _OscPart.target) {
      final raw = _target.takeBytes();
      final target = _targetInvalid ? null : _decodeTarget(raw);
      if (target != null) {
        if (_linkOpen) _emit(const LinkEnd());
        if (target.isEmpty) {
          _linkOpen = false;
        } else {
          _emit(LinkStart(target));
          _linkOpen = true;
        }
      }
    }
    _toGround();
  }

  /// Decodes an OSC 8 target; null unless every character is printable ASCII (0x21 to 0x7E).
  ///
  /// A URI is ASCII (RFC 3986; an IRI travels percent-encoded), and the target is shown before
  /// it opens (24 11.2): spaces, line and paragraph separators, bidi controls or look-alike
  /// letters could push the real host out of view or make it read differently.
  static String? _decodeTarget(Uint8List raw) {
    for (final b in raw) {
      if (b < 0x21 || b > 0x7e) return null;
    }
    return ascii.decode(raw);
  }

  void _stringDropByte(int byte) {
    if (_stringEscape) {
      _stringEscape = false;
      if (byte == _backslash) {
        _toGround();
      } else {
        _abortToEscape(byte);
      }
    } else if (byte == _esc) {
      _stringEscape = true;
    } else if (byte == _can || byte == _sub) {
      _toGround();
    }
  }

  /// Handles the bytes that every ESC or CSI state treats alike (rule 5); returns true when
  /// [byte] is done.
  bool _isSequenceControl(int byte) {
    if (byte == _esc) {
      _startEscape();
    } else if (byte == _can || byte == _sub) {
      _toGround();
    } else if (byte < 0x20) {
      _text.addByte(byte);
    } else if (byte == _del) {
      // Ignored inside a sequence, as by the VT500 parser.
    } else if (byte >= 0x80) {
      _toGround();
      _text.addByte(byte);
    } else {
      return false;
    }
    return true;
  }

  /// Adds a parameter or intermediate byte, or switches to [ignore] when the final byte
  /// would no longer fit under [maxControlSequenceBytes].
  void _append(int byte, {required _State ignore}) {
    if (_state == ignore) return;
    if (_sequence.length >= maxControlSequenceBytes - 1) {
      _sequence.clear();
      _state = ignore;
    } else {
      _sequence.add(byte);
    }
  }

  /// An ESC inside a string sequence was not ST: the sequence is dropped and a new escape
  /// sequence starts at that ESC with [byte] as its first byte (rule 5).
  void _abortToEscape(int byte) {
    _toGround();
    _startEscape();
    _escapeByte(byte);
  }

  void _toGround() {
    _state = _State.ground;
    _sequence.clear();
    _target.clear();
    _targetInvalid = false;
    _stringEscape = false;
    _oscPart = _OscPart.command;
  }

  void _emit(FilteredChunk chunk) {
    _flushText();
    _chunks.add(chunk);
  }

  void _flushText() {
    if (_text.isNotEmpty) _chunks.add(FilteredText(_text.takeBytes()));
  }
}

/// The parser states (rule 5 follows the VT500 parser of vt100.net, reduced to what the allow
/// list needs).
enum _State {
  ground,
  escape,
  escapeIntermediate,
  escapeIgnore,
  csi,
  csiIgnore,
  osc,
  stringDrop,
}

/// Where an OSC stands: its command number, the params of OSC 8, its target, or dropped.
enum _OscPart { command, sawEight, params, target, drop }

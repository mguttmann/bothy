// SPDX-License-Identifier: Apache-2.0

/// What the document viewer shows of a text attachment (ticket FEAT-markdown-viewer): which
/// files it opens ([documentKindOf]), how much of a file it reads, and the prepared forms of
/// the text (Markdown, pretty-printed JSON, a small CSV as a table).
library;

import 'dart:convert';
import 'dart:io';

import 'package:domain/domain.dart';

/// The kinds of files the document viewer opens.
enum DocumentKind {
  /// Markdown, rendered with the renderer of messages.
  markdown,

  /// JSON, pretty-printed.
  json,

  /// Comma separated values, a table when small.
  csv,

  /// YAML, as text with syntax colors.
  yaml,

  /// Plain text and logs.
  text,
}

/// The kind of [file] for the viewer, or null when the viewer does not open it: by the
/// extension of its name first (`.md`, `.markdown`, `.txt`, `.log`, `.json`, `.yaml`, `.yml`,
/// `.csv`), else by its MIME type (`text/markdown`, `application/json`, `text/csv`, YAML types,
/// `text/plain`).
DocumentKind? documentKindOf(FileMeta file) {
  final name = file.originalName.toLowerCase().trim();
  final dot = name.lastIndexOf('.');
  final ext = dot < 0 ? '' : name.substring(dot + 1);
  final byName = switch (ext) {
    'md' || 'markdown' || 'mdown' || 'mkd' || 'mdx' => DocumentKind.markdown,
    'json' || 'jsonl' || 'ndjson' => DocumentKind.json,
    'csv' => DocumentKind.csv,
    'yaml' || 'yml' => DocumentKind.yaml,
    'txt' || 'text' || 'log' => DocumentKind.text,
    _ => null,
  };
  if (byName != null) return byName;
  final mime = file.mimeType.toLowerCase().split(';').first.trim();
  return switch (mime) {
    'text/markdown' || 'text/x-markdown' => DocumentKind.markdown,
    'application/json' || 'text/json' => DocumentKind.json,
    'text/csv' => DocumentKind.csv,
    'application/yaml' ||
    'application/x-yaml' ||
    'text/yaml' ||
    'text/x-yaml' => DocumentKind.yaml,
    'text/plain' => DocumentKind.text,
    _ => null,
  };
}

/// The most bytes of a file the viewer reads; a longer file shows its beginning with a notice
/// (the whole file stays available through Save as and Open externally).
const int documentMaxLoadBytes = 8 * 1024 * 1024;

/// The largest text the viewer renders (Markdown, pretty JSON, a CSV table) when it opens; a
/// longer one opens as raw text with a notice and renders only on request.
const int documentRenderLimitBytes = 1024 * 1024;

/// The most rows (with the header) and columns of a CSV that shows as a table.
const int csvTableMaxRows = 500;

/// The most columns of a CSV that shows as a table.
const int csvTableMaxColumns = 40;

/// The text of a document and its prepared forms.
final class DocumentContent {
  /// Prepares [text] of [kind]; [totalBytes] is the size of the whole file, [truncated] whether
  /// [text] is only its beginning.
  factory DocumentContent.prepare(
    String text,
    DocumentKind kind, {
    required int totalBytes,
    bool truncated = false,
  }) {
    final small = !truncated && totalBytes <= documentRenderLimitBytes;
    String? pretty;
    String? table;
    if (kind == DocumentKind.json && small) pretty = prettyJson(text);
    if (kind == DocumentKind.csv && !truncated) table = csvAsMarkdown(text);
    return DocumentContent._(
      text: text,
      kind: kind,
      totalBytes: totalBytes,
      truncated: truncated,
      pretty: pretty,
      csvTable: table,
    );
  }

  DocumentContent._({
    required this.text,
    required this.kind,
    required this.totalBytes,
    required this.truncated,
    required this.pretty,
    required this.csvTable,
  });

  /// The text as the file has it (decoded as UTF-8, malformed bytes replaced).
  final String text;

  /// The kind.
  final DocumentKind kind;

  /// The size of the whole file in bytes.
  final int totalBytes;

  /// Whether [text] is only the first [documentMaxLoadBytes] of the file.
  final bool truncated;

  /// The pretty-printed JSON, or null (not JSON, not valid, or too large).
  final String? pretty;

  /// A small CSV as a Markdown table, or null.
  final String? csvTable;

  /// Whether the text is above [documentRenderLimitBytes]: it opens raw.
  bool get large => truncated || totalBytes > documentRenderLimitBytes;

  /// Whether a rendered form exists besides the raw text.
  bool get renderable => switch (kind) {
    DocumentKind.markdown => true,
    DocumentKind.json => pretty != null,
    DocumentKind.csv => csvTable != null,
    DocumentKind.yaml || DocumentKind.text => false,
  };
}

/// Reads at most [documentMaxLoadBytes] of [file] and prepares them as [kind].
Future<DocumentContent> readDocument(File file, DocumentKind kind) async {
  final total = await file.length();
  final handle = await file.open();
  try {
    final bytes = await handle.read(documentMaxLoadBytes);
    var text = utf8.decode(bytes, allowMalformed: true);
    final truncated = total > bytes.length;
    // A cut in the middle of a character leaves one replacement character at the end.
    if (truncated && text.endsWith('�')) {
      text = text.substring(0, text.length - 1);
    }
    if (text.startsWith('﻿')) text = text.substring(1);
    return DocumentContent.prepare(
      text,
      kind,
      totalBytes: total,
      truncated: truncated,
    );
  } finally {
    await handle.close();
  }
}

/// [text] as JSON with two spaces of indentation, or null when it is no valid JSON. A JSON
/// Lines text (one value per line) prints each value.
String? prettyJson(String text) {
  const encoder = JsonEncoder.withIndent('  ');
  try {
    return encoder.convert(jsonDecode(text));
  } on FormatException {
    final lines = text.split('\n').where((l) => l.trim().isNotEmpty).toList();
    if (lines.length < 2) return null;
    try {
      return [for (final l in lines) encoder.convert(jsonDecode(l))].join('\n');
    } on FormatException {
      return null;
    }
  }
}

/// The rows of the CSV [text] (RFC 4180: quoted fields with `""` for a quote, line breaks in
/// quotes), or null when a quote is never closed.
List<List<String>>? parseCsv(String text) {
  final rows = <List<String>>[];
  var row = <String>[];
  final field = StringBuffer();
  var quoted = false;
  var i = 0;
  void endField() {
    row.add(field.toString());
    field.clear();
  }

  void endRow() {
    endField();
    rows.add(row);
    row = <String>[];
  }

  while (i < text.length) {
    final c = text[i];
    if (quoted) {
      if (c == '"') {
        if (i + 1 < text.length && text[i + 1] == '"') {
          field.write('"');
          i += 2;
          continue;
        }
        quoted = false;
      } else {
        field.write(c);
      }
      i++;
      continue;
    }
    switch (c) {
      case '"' when field.isEmpty:
        quoted = true;
      case ',':
        endField();
      case '\r':
        break;
      case '\n':
        endRow();
      default:
        field.write(c);
    }
    i++;
  }
  if (quoted) return null;
  if (field.isNotEmpty || row.isNotEmpty) endRow();
  return rows;
}

/// A small CSV [text] as a Markdown table (the first row as its header), or null when it is
/// not a valid CSV, has fewer than two columns, or is larger than [csvTableMaxRows] rows or
/// [csvTableMaxColumns] columns. Every cell is escaped, so it renders as its text.
String? csvAsMarkdown(String text) {
  final rows = parseCsv(text);
  if (rows == null || rows.isEmpty || rows.length > csvTableMaxRows) {
    return null;
  }
  final width = rows.fold<int>(0, (w, r) => r.length > w ? r.length : w);
  if (width < 2 || width > csvTableMaxColumns) return null;
  String cell(String v) {
    final flat = v.replaceAll(RegExp(r'\s*\n\s*'), ' ').trim();
    final escaped = flat.replaceAllMapped(
      RegExp(r'[\\`*_{}\[\]()#+\-.!|<>~&]'),
      (m) => '\\${m[0]}',
    );
    // An empty cell needs a space, or the row loses its column.
    return escaped.isEmpty ? ' ' : escaped;
  }

  String line(List<String> r) =>
      '| ${[for (var c = 0; c < width; c++) cell(c < r.length ? r[c] : '')].join(' | ')} |';
  return [
    line(rows.first),
    '|${List.filled(width, '---').join('|')}|',
    for (final r in rows.skip(1)) line(r),
  ].join('\n');
}

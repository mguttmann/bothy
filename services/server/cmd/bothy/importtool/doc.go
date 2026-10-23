// SPDX-License-Identifier: Apache-2.0

// Package importtool is the command `bothy import discord` (docs/spec/22-import-migration.md
// 4 to 11, 14, 16 and 19; ADR 0043): it reads one Discord server through the read-only client of
// the subpackage discord and writes into Bothy only through the public import interface of the
// server (subpackage bothyapi).
//
// Without --apply a run is a dry run: it sends only GET requests to Discord and Bothy (plus
// refreshToken when the access token expired), downloads no attachment, writes no file except
// the rotated session file of `bothy import login`, and prints the check report of 22 11 with
// "would take over". With --apply it imports into the running source of the guild that the
// owner created and started in the web console, after the preflight of 22 4.4: structure
// batches (spaces, channels, forum tags), then per channel the main thread and its threads in
// Discord order, each batch preceded by the user items of its authors, attachments through
// createImportFile and its upload, the cursor advanced only after a successful batch, under a
// run lease that keeps two writing runs apart (22 7.3, 7.4).
//
// The parts:
//
//   - config.go: the tool keys of 22 19 over platform/config (--config and BOTHY_IMPORT__...).
//   - mapping.go: the mapping file import_source.discord.v1 (22 5.2), strict, with its
//     normalized JSON form and the comparison with the stored mapping.
//   - convert.go: the conversion of a Discord message to the Markdown dialect (22 6.1 to 6.3).
//   - preflight.go, plan.go, attachments.go, apply.go, report.go: the steps of a run.
//   - initmap.go, status.go, run.go: the commands and the exit codes of 22 10.3.
//
// No output (standard output, standard error, report, log) carries message content, a file name
// of an attachment, a name of a person, a token or a signed URL (22 10.2 rule 4).
package importtool

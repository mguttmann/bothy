# SPDX-License-Identifier: Apache-2.0
#
# R8 rules of the release build (docs/decisions.d/M5-03.md D-M5-03-8). The Flutter Gradle plugin
# adds the rules of the engine and the plugins bring their own consumer rules; the runner's own
# classes are reached from the manifest and the Pigeon channel, which R8 follows without rules.

---
status: accepted
date: 2026-11-25
decision-makers: Owner, maintainer
---

# Visual C++ runtime in the Windows package from the toolchain redist

## Context and Problem Statement

A Flutter Windows app loads the Visual C++ runtime (`msvcp140.dll`, `vcruntime140.dll`,
`vcruntime140_1.dll`). The MSIX of the Windows app carries these files next to the executable
(app-local deployment), so it installs without a framework package (`tools/distribution/windows.py`,
`stage`). Until now `stage` copied a file the build lacked from `System32` of the build machine:
version and origin then depended on the runner image, and the files are proprietary Microsoft
code, which docs/quality/licenses.md allows only with an ADR (issue #869).

Microsoft names the folder `[VisualStudioFolder]\VC\redist` as Distributable Code of Visual Studio
2022 (Enterprise, Professional, Community): "you may copy and distribute with your program any of
the files within the following folder and its subfolders except as noted below. You may not modify
these files", subject to the license terms of the edition; the exceptions are the `debug_nonredist`
folders (https://learn.microsoft.com/en-us/visualstudio/releases/2022/redistribution, retrieved
2026-11-25) [verified]. The same page limits the distribution of runtime files obtained from the
download pages to licensed Visual Studio users [verified]. Files copied out of `System32` are named
nowhere in that list.

## Decision Drivers

* Only files whose redistribution the vendor grants go into a package of the project.
* The version of the runtime comes from the build toolchain, not from the state of a runner image.
* No new download in the build and no change of the install experience of the MSIX.

## Considered Options

* Copy the runtime from the redist folder of the Visual Studio that builds the app
* Keep copying from `System32`
* Declare the framework package `Microsoft.VCLibs.140.00.UWPDesktop` as a dependency of the MSIX
  instead of app-local files

## Decision Outcome

Chosen option: "Copy the runtime from the redist folder of the Visual Studio that builds the app",
because it is the only option whose files the vendor lists as Distributable Code and whose version
the build toolchain fixes, without changing how the MSIX installs.

`stage` takes a file of `VC_RUNTIME` the build output lacks from
`VC\Redist\MSVC\<version>\x64\Microsoft.VC*.CRT` of the newest Visual Studio under
`%ProgramFiles%\Microsoft Visual Studio`, where `<version>` is the toolchain's default redist
version (`VC\Auxiliary\Build\Microsoft.VCRedistVersion.default.txt`), copies it unmodified, and
prints that folder and version in the build log. Without such a folder the step fails; it never
falls back to `System32`. The files stay unmodified, never come from a `debug_nonredist` folder,
and are listed under "Bundled native code" in docs/quality/licenses.md.

### Consequences

* Good, because the package holds only files the vendor grants for redistribution, in the version
  of the toolchain that built the app, and the build log names that version.
* Bad, because the grant holds under the license terms of the Visual Studio edition that builds
  the package (the GitHub-hosted Windows runner of the release workflow); the conditions of their
  section "Distributable Code" are not quoted here [unverified] and are checked again when the
  build moves to another edition or another builder.
* Bad, because a build machine without Visual Studio (Build Tools only) has no such folder under
  `%ProgramFiles%` and fails the step; the GitHub-hosted runners have Visual Studio.

### Confirmation

`tools/distribution/tests/test_windows.py`: `test_vc_redist_takes_the_toolchain_default_of_the_newest_install`
(the default version of the newest Visual Studio wins, another version folder is never taken, no
redist fails) and `test_stage_needs_one_executable_and_the_runtime` (a missing runtime file fails
the step); the Windows trial job of CI (`windows.py trial`) runs `stage` on the runner.

## Pros and Cons of the Options

### Copy the runtime from the toolchain redist

* Good, because the vendor lists the folder as Distributable Code.
* Good, because the version follows the toolchain, not the operating system image.
* Bad, because the step depends on the layout of a Visual Studio installation.

### Keep copying from `System32`

* Good, because every Windows machine has the files.
* Bad, because the vendor's list of Distributable Code does not name these copies, and their
  version is whatever the image installed.

### Framework package dependency

* Good, because no runtime file is in the package.
* Bad, because an installation from the App Installer file must also fetch and install the
  framework package, and the version is then the one of the Store package, not of the toolchain.

## More Information

Issue #869; ruling log docs/decisions.d/AUD-PKG-R2-owner-ops.md; licenses:
[docs/quality/licenses.md](../quality/licenses.md).

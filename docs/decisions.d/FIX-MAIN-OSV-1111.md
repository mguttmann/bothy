# FIX-MAIN-OSV-1111 Advisory gate red on main after new node-forge advisories

`make osv` failed on main in run 43028486729 (push of 575359bdc, 2026-11-11) and before that in
the daily advisory run 42965017488 on the commit before it. No change of the repository caused
it: four advisories for node-forge were published on 2026-11-10.

| No | Date | Ruling | Reason |
|---|---|---|---|
| FMO/1 | 2026-11-11 | `tools/node/package.json` overrides node-forge to 1.4.3 and `tools/node/package-lock.json` takes that release; no other package of the lock file moves and no finding is accepted. SEC-OSV/1 stays in force with this version. | GHSA-6889-vf6x-f3g5, GHSA-6cv4-p8m7-4r25, GHSA-j8rw-jp99-435v and GHSA-w4xr-hqv8-ch44 affect node-forge before 1.4.3 (https://osv.dev/GHSA-6889-vf6x-f3g5 and the three others, retrieved 2026-11-11), and the lock file held 1.4.2. A fixed release exists, so the findings are fixed and not accepted (docs/quality/gates.md, "Advisories"). The scanner calls node-forge only to read a PKCS#12 truststore (`extractTruststoreCerts` in its `src/request.js`); a probe ran that code through `getHttpAgents` of the pinned `@sonar/scan` with node-forge 1.4.3 on a truststore made by OpenSSL and got the certificate back unchanged. |
| FMO/2 | 2026-11-11 | `@sonar/scan` stays on its pinned release. The override goes when the scanner moves to a release that asks for a fixed node-forge itself. | `@sonar/scan` 5.1.0 asks for node-forge 1.4.3 exactly (https://registry.npmjs.org/@sonar/scan/5.1.0, retrieved 2026-11-11). A new scanner release is a tool change of its own that the `sonar` job has to prove, and the advisories need only the node-forge release; the scanner update comes with the weekly dependency updates of the group "dev tools (Node)" (`renovate.json`). |

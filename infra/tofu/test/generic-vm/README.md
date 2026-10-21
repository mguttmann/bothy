# Test root of generic-vm

`main.tf` calls the module [generic-vm](../../modules/generic-vm/README.md) with values from
`-var` arguments and returns its outputs. Two tests use it (brief M7-15):

- `tools/ci/tests/test_tofu_generic_vm.py` (part of `make test`) plans it for every `edge_mode`
  and reads the rendered user data and the plan.
- `make tofu-vm-test` runs [tools/ci/tofu_vm_test.py](../../../../tools/ci/tofu_vm_test.py) on the
  build host. It is not part of `make check` and has no CI job: the product trial it starts
  refuses to run under GitHub Actions (D-W-127).

## What `make tofu-vm-test` does

1. The product trial of `make release-trial COMPONENT=bothy`: the pinned registry on
   `127.0.0.1`, the server images for the host platform, bundle and `install.sh`, signed with
   the throwaway key of the trial like the job `product-signing` of `release.yml` and verified,
   served by a static HTTP server on `127.0.0.1`. Two broken copies are served next to it: the
   bundle with one byte appended (`bad-sum`), and that bundle with a `SHA256SUMS` that matches it
   under the old signature (`bad-sig`).
2. `tofu apply` of this root with the download address and public key of the trial.
3. Ubuntu 24.04 (`release-20261017`, pinned by SHA-256 in `tofu_vm_test.py`, cached verified in
   `.cache/tofu-vm/`) in QEMU with user networking: SSH forwarded on `127.0.0.1` with a random
   port, the NoCloud seed from the same static server (SMBIOS `ds=nocloud;s=...`), KVM on
   Linux, HVF on macOS, TCG otherwise. A test-only cloud-config part in front of the module's
   user data forwards `127.0.0.1:<registry port>` and `127.0.0.1:<server port>` inside the VM to
   the host (`10.0.2.2`) with nftables DNAT, so the registry and download address of the bundle
   work unchanged. Docker Engine accepts a registry on a loopback address over plain HTTP, so
   the VM needs no `insecure-registries` entry.
4. `cloud-init status --wait`, then a check that the enrollment code is not in the cloud-init log.
5. The smoke probe of M1-73 (`bothy-smoke run`: enrollment, live delivery over WebSocket,
   catch-up, step-up, revocation) through an SSH forward of the published port, with a code from
   `bothy setup issue-enrollment-code`. Its `precheck` (before the bootstrap) and `scan` (of the
   collector export of the test overlay) do not apply to an installed stack.
6. A snapshot of the host (hashes of every file the setup writes, `.env`, the listing of
   `secrets/` without contents, the Docker packages, the firewall table, the containers with
   their IDs and start times), the same setup over SSH (`provision_over_ssh = true`, key in a
   throwaway `ssh-agent`), the snapshot again (must be equal) and a plan without changes.
7. The setup with `bad-sum` and `bad-sig` in another stack directory: both stop in the step
   `verify` with exit code 1, the directory is never created, and the snapshot stays equal.

Everything runs in a `mktemp -d` directory, which is removed at the end with the VM disk; QEMU,
the SSH forward and the agent are stopped by PID; the trial removes its registry and builder.
`--keep` (run the script directly) keeps the directory for a look at the logs.

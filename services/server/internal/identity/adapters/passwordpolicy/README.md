# Adapter `passwordpolicy`

The local list of common passwords and the NFC normalisation of passwords for the local accounts
of `identity` (docs/spec/08-identitaet.md section 2.5, brief M1-39). It implements
`app.CommonPasswords` and `app.PasswordNormalizer`.

## The list

`common_passwords.txt` holds one line per entry of the source: the first 16 hex characters of
`SHA-256(lowercase(NFC(password)))`, sorted and unique. The repository holds no word list, only
these truncated hashes. `Contains` normalises the candidate to NFC, lowercases it, hashes it the
same way and searches the sorted list, so a listed password is found in any case.

| Item | Value |
|---|---|
| Source | `Passwords/Common-Credentials/10k-most-common.txt` of SecLists, https://github.com/danielmiessler/SecLists |
| Commit | `3a5f62f0e18a1b8a36b25399aff0077f3706e5af` (2026-10-13) |
| Licence | MIT (the `LICENSE` file of the repository at that commit), Copyright (c) 2018 Daniel Miessler |
| SHA-256 of the source file | `68782d6a4a19a4768d5f15dd66bd534e7a33055cc755411e33f16d18c50fdcce` |
| Entries | 10001 lines in the source, 10001 unique hashes |

Generation (Python 3, in an empty temporary directory; only the generated file is committed):

```sh
C=3a5f62f0e18a1b8a36b25399aff0077f3706e5af
curl -sSfo src.txt "https://raw.githubusercontent.com/danielmiessler/SecLists/$C/Passwords/Common-Credentials/10k-most-common.txt"
shasum -a 256 src.txt   # must print the checksum above
python3 -c '
import hashlib, sys, unicodedata
lines = open("src.txt", encoding="utf-8").read().splitlines()
out = sorted({hashlib.sha256(unicodedata.normalize("NFC", l).lower().encode("utf-8")).hexdigest()[:16] for l in lines if l})
sys.stdout.write("\n".join(out) + "\n")
' > common_passwords.txt
```

`TestCommonPasswordsFile` checks that the file is sorted, unique and well formed and that entries
taken from the source during generation are found.

## Normalisation

`NFC` uses `golang.org/x/text/unicode/norm`. The use cases normalise every password before the
length rule, the list check, hashing and verification, so the same password typed on two
keyboards is the same password.

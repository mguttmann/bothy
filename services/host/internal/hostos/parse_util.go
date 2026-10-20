// SPDX-License-Identifier: Apache-2.0

package hostos

import "io/fs"

func fsMode(perm uint16) fs.FileMode { return fs.FileMode(perm & 7) }

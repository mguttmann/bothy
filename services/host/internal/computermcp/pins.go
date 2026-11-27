// SPDX-License-Identifier: Apache-2.0

package computermcp

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// pinFile is the content of the pin file (27 10.4): per host ID the Noise key, its
// key_generation, and whether the next open request of the host says pin_changed (D-CU-05-5).
type pinFile struct {
	V     int            `json:"v"`
	Hosts map[string]pin `json:"hosts"`
}

// pin is the pinned key of one host.
type pin struct {
	Key        string `json:"noise_public_key"`
	Generation int64  `json:"key_generation"`
	Changed    bool   `json:"pin_changed"`
}

// pinVerdict is the outcome of a pin check.
type pinVerdict int

const (
	// pinStored: the first open of the host stored the key.
	pinStored pinVerdict = iota + 1
	// pinMatch: the key equals the pin.
	pinMatch
	// pinReplaced: another key with a higher generation replaced the pin.
	pinReplaced
	// pinMismatch: another key with the same or a lower generation; the session is refused.
	pinMismatch
)

// errPinFile is a pin file that cannot be read or parsed; the session is refused rather than
// pinned anew, so a damaged file never turns into trust on first use again.
var errPinFile = errors.New("computermcp: pin file unreadable")

// pinStore keeps the pin file at path.
type pinStore struct {
	path   string
	random io.Reader
}

// load reads the pin file; a missing file is an empty store.
func (p *pinStore) load() (pinFile, error) {
	b, err := os.ReadFile(p.path)
	if errors.Is(err, fs.ErrNotExist) {
		return pinFile{V: 1, Hosts: map[string]pin{}}, nil
	}
	var f pinFile
	if err != nil || strictDecode(b, &f) != nil || f.V != 1 || f.Hosts == nil {
		return pinFile{}, errPinFile
	}
	return f, nil
}

// save writes the pin file atomically with mode 0600, creating its folder with mode 0700.
func (p *pinStore) save(f pinFile) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o700); err != nil {
		return fmt.Errorf("computermcp: pin folder: %w", err)
	}
	id, err := newUUID(p.random)
	if err != nil {
		return err
	}
	return writeAtomic(p.path, p.path+"."+id+tmpSuffix, b)
}

// pending reports whether the next open request of host says pin_changed.
func (p *pinStore) pending(host string) (bool, error) {
	f, err := p.load()
	if err != nil {
		return false, err
	}
	return f.Hosts[host].Changed, nil
}

// check compares the key of an open answer with the pin of host and updates the file (27 10.4,
// D-CU-01-18): the first key is stored; another key with a higher generation replaces the pin
// and marks pin_changed for the next open request; another key with the same or a lower
// generation is a mismatch and changes nothing. sent says whether this open request carried
// pin_changed; a successful open clears the mark it carried.
func (p *pinStore) check(host string, key [32]byte, gen int64, sent bool) (pinVerdict, error) {
	f, err := p.load()
	if err != nil {
		return 0, err
	}
	enc := base64.RawURLEncoding.EncodeToString(key[:])
	old, ok := f.Hosts[host]
	var v pinVerdict
	switch {
	case !ok:
		v = pinStored
		f.Hosts[host] = pin{Key: enc, Generation: gen}
	case old.Key == enc:
		v = pinMatch
		changed := old.Changed && !sent
		if gen == old.Generation && changed == old.Changed {
			return v, nil
		}
		f.Hosts[host] = pin{Key: enc, Generation: max(gen, old.Generation), Changed: changed}
	case gen > old.Generation:
		v = pinReplaced
		f.Hosts[host] = pin{Key: enc, Generation: gen, Changed: true}
	default:
		return pinMismatch, nil
	}
	return v, p.save(f)
}

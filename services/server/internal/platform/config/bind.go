// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"reflect"
	"slices"
	"time"
)

// The field types Bind fills, each with the kinds it accepts. Only these exact types fit; a named
// type such as `type Role string` does not, so that a field never silently changes meaning.
var bindTypes = map[reflect.Type][]Kind{
	reflect.TypeFor[string]():            stringKinds,
	reflect.TypeFor[bool]():              {KindBool},
	reflect.TypeFor[int64]():             {KindInt},
	reflect.TypeFor[int]():               {KindInt},
	reflect.TypeFor[float64]():           {KindFloat},
	reflect.TypeFor[time.Duration]():     {KindDuration},
	reflect.TypeFor[[]string]():          {KindStringList},
	reflect.TypeFor[[]netip.Prefix]():    {KindCIDRList},
	reflect.TypeFor[map[string]string](): {KindStringMap},
}

// Bind fills the struct dst points to from v. Every exported field carries a tag
// `config:"<key>"` and has the Go type of the key's kind: string (string, enum, listen address,
// cron, semver, secret_ref, url), bool, int64 or int (int), float64, time.Duration, []string
// (list(string)), []netip.Prefix (list(cidr)) or map[string]string (map(string)). Unexported
// fields are left alone.
//
// A consumer declares its Config struct in its own package (02 "Konfigurationsladen" item 5);
// internal/wire loads the values once and binds each struct. Bind returns an error, and changes
// nothing, when dst is not a non-nil pointer to a struct, when an exported field has no tag,
// names an unknown key or has a type that does not fit the kind, or when an int value does not
// fit into int. The error names the field and the key, never a value. Slices and maps are copies.
//
// Example:
//
//	type Config struct {
//		ListenAddr string        `config:"server.listen_addr"`
//		Retention  time.Duration `config:"sync.event_retention"`
//	}
//	var c Config
//	if err := config.Bind(values, &c); err != nil {
//		return err
//	}
func Bind(v *Values, dst any) error {
	if v == nil {
		return errors.New("config: Bind needs loaded values, got nil")
	}
	rv := reflect.ValueOf(dst)
	if !rv.IsValid() || rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("config: Bind needs a non-nil pointer to a struct, got %T", dst)
	}
	target := rv.Elem()
	st := target.Type()
	var errs []error
	var sets []func()
	for i := range st.NumField() {
		f := st.Field(i)
		if !f.IsExported() {
			continue
		}
		fv := target.Field(i)
		x, err := bindValue(v, st.Name()+"."+f.Name, f, fv)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		sets = append(sets, func() { fv.Set(x) })
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	for _, set := range sets {
		set()
	}
	return nil
}

// bindValue returns the value Bind sets into the struct field f with the value fv, or the error
// for it; field is the name the error gives the field.
func bindValue(v *Values, field string, f reflect.StructField, fv reflect.Value) (reflect.Value, error) {
	key := f.Tag.Get("config")
	if key == "" {
		return reflect.Value{}, fmt.Errorf("config: %s has no config tag", field)
	}
	val, ok := v.vals[key]
	if !ok {
		return reflect.Value{}, fmt.Errorf("config: %s: unknown key %q", field, key)
	}
	kind := v.schema.entries[key].Kind
	if !slices.Contains(bindTypes[f.Type], kind) {
		return reflect.Value{}, fmt.Errorf("config: %s has type %s, which does not fit key %q of kind %s", field, f.Type, key, kind)
	}
	x := reflect.ValueOf(val.v)
	switch {
	case f.Type.Kind() == reflect.Int:
		n := val.v.(int64)
		if fv.OverflowInt(n) {
			return reflect.Value{}, fmt.Errorf("config: %s: the value of key %q does not fit into int", field, key)
		}
		x = reflect.ValueOf(int(n))
	case f.Type.Kind() == reflect.Slice:
		x = reflect.AppendSlice(reflect.MakeSlice(f.Type, 0, x.Len()), x)
	case f.Type.Kind() == reflect.Map:
		x = reflect.ValueOf(maps.Clone(val.v.(map[string]string)))
	}
	return x, nil
}

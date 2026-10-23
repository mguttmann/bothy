// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/designkeys"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The keys of instance_settings (docs/operations/configuration.md section 6).
const (
	// KeyAccentColor is branding.accent_color, an accent key of kernel/designkeys.
	KeyAccentColor = "branding.accent_color"
	// KeyLogoFileID is branding.logo_file_id, the UUIDv7 of the logo file or NoLogo; only the logo
	// operations of M3-10 write it.
	KeyLogoFileID = "branding.logo_file_id"
	// KeyProductName is branding.product_name, the displayed product name.
	KeyProductName = "branding.product_name"
	// KeyLocaleDefault is locale.default, the language of new people.
	KeyLocaleDefault = "locale.default"
)

// The defaults of the registry.
const (
	// DefaultProductName is the default of branding.product_name and the one place of the working
	// name in this module (ADR 0044 counts branding.* among the name-bound places; 02 "6.
	// Code layout").
	DefaultProductName = "Bothy"
	// DefaultAccentColor is the default of branding.accent_color.
	DefaultAccentColor = "spruce"
	// DefaultLocale is the default of locale.default (01 section 9).
	DefaultLocale = "de"
	// NoLogo is the stored value of branding.logo_file_id after the logo was removed: the row is
	// never deleted, so the branding version keeps rising (brief M3-10 design note 9).
	NoLogo = ""
)

// Route names the operation that writes a key: putInstanceSetting for RouteSettings,
// updateBranding (and the logo operations of M3-10) for RouteBranding (docs/DECISIONS.md M1-46/1).
type Route string

// The routes of a key.
const (
	// RouteSettings keys change through putInstanceSetting.
	RouteSettings Route = "settings"
	// RouteBranding keys change only through the branding operations.
	RouteBranding Route = "branding"
)

// Field error codes of the settings and branding rules (06 section 5, FieldError.code).
const (
	// CodeRequired marks a missing or blank value.
	CodeRequired = "required"
	// CodeInvalidText marks a display text with a control, bidi control or invalid character
	// (SEC-IN-4).
	CodeInvalidText = "invalid_text"
	// CodeTooLong marks a text above its limit.
	CodeTooLong = "too_long"
	// CodeUnknownColorKey marks a colour that is no accent key of the design system.
	CodeUnknownColorKey = "unknown_color_key"
	// CodeInvalidType marks a JSON value of the wrong type.
	CodeInvalidType = "invalid_type"
	// CodeInvalidEnum marks a value outside its list.
	CodeInvalidEnum = "invalid_enum"
	// CodeInvalidFormat marks a value of the wrong form, for example no UUIDv7.
	CodeInvalidFormat = "invalid_format"
	// CodeUseBrandingEndpoint marks a branding key sent to putInstanceSetting.
	CodeUseBrandingEndpoint = "use_branding_endpoint"
)

// Pointers of the field errors.
const (
	pointerValue       = "/value"
	pointerProductName = "/product_name"
	pointerAccentColor = "/accent_color"
)

// TypeString is the JSON type of every key of this version.
const TypeString = "string"

// Definition is one registered setting key.
type Definition struct {
	// Key is the setting key.
	Key string
	// Type is the JSON type of the value; every key of this version is a string.
	Type string
	// Default is the JSON value of a key without a row; nil means the key has no default and no
	// value until it is written (branding.logo_file_id).
	Default json.RawMessage
	// Route is the operation that writes the key.
	Route Route
}

// Registry is the code registry of the instance settings: one Definition per key of
// configuration.md section 6, and the rules that check a value (03 instance_settings: "checked against
// the schema of the key"). It is immutable and safe for concurrent use.
type Registry struct {
	defs           []Definition
	productNameMax int
	locales        []string
}

// jsonString returns s as a JSON string.
func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

// NewRegistry returns the registry with the product name limit branding.product_name_max_chars
// and the supported locales (instance.SupportedLocales). Errors: a limit below 1, no locales or
// locales without DefaultLocale.
func NewRegistry(productNameMax int, locales []string) (*Registry, error) {
	if productNameMax < 1 {
		return nil, errors.New("instance: branding.product_name_max_chars must be at least 1")
	}
	if !slices.Contains(locales, DefaultLocale) {
		return nil, errors.New("instance: the supported locales must contain the default locale")
	}
	return &Registry{
		defs: []Definition{
			{Key: KeyAccentColor, Type: TypeString, Default: jsonString(DefaultAccentColor), Route: RouteBranding},
			{Key: KeyLogoFileID, Type: TypeString, Route: RouteBranding},
			{Key: KeyProductName, Type: TypeString, Default: jsonString(DefaultProductName), Route: RouteBranding},
			{Key: KeyLocaleDefault, Type: TypeString, Default: jsonString(DefaultLocale), Route: RouteSettings},
		},
		productNameMax: productNameMax,
		locales:        slices.Clone(locales),
	}, nil
}

// Keys returns every registered key, sorted.
func (r *Registry) Keys() []string {
	out := make([]string, len(r.defs))
	for i, d := range r.defs {
		out[i] = d.Key
	}
	return out
}

// Lookup returns the definition of key and whether key is registered.
func (r *Registry) Lookup(key string) (Definition, bool) {
	i := slices.IndexFunc(r.defs, func(d Definition) bool { return d.Key == key })
	if i < 0 {
		return Definition{}, false
	}
	d := r.defs[i]
	d.Default = slices.Clone(d.Default)
	return d, true
}

// ProductNameMax returns branding.product_name_max_chars.
func (r *Registry) ProductNameMax() int { return r.productNameMax }

// Validate checks value against the schema of key and returns its canonical JSON form (a product
// name trimmed). A failed rule is validation_failed with one field error at /value, the member of
// the body of putInstanceSetting; an unknown key is internal (the caller looks the key up first).
//
// Example:
//
//	v, err := reg.Validate("locale.default", json.RawMessage(`"en"`)) // `"en"`, nil
func (r *Registry) Validate(key string, value json.RawMessage) (json.RawMessage, error) {
	if _, ok := r.Lookup(key); !ok {
		return nil, errs.New(errs.CodeInternalError, "instance: validate of an unregistered setting key")
	}
	s, err := stringValue(value)
	if err != nil {
		return nil, err
	}
	switch key {
	case KeyProductName:
		name, err := ValidateProductName(s, r.productNameMax)
		if err != nil {
			return nil, repoint(err)
		}
		return jsonString(name), nil
	case KeyAccentColor:
		if err := ValidateAccentColor(s); err != nil {
			return nil, repoint(err)
		}
	case KeyLogoFileID:
		if _, err := LogoFileID(s); err != nil {
			return nil, field(pointerValue, CodeInvalidFormat, "The value is not a UUIDv7 or the empty string.")
		}
	case KeyLocaleDefault:
		if !slices.Contains(r.locales, s) {
			return nil, field(pointerValue, CodeInvalidEnum, "The value is not one of "+strings.Join(r.locales, ", ")+".")
		}
	}
	return jsonString(s), nil
}

// stringValue decodes a JSON string; any other JSON value (null included) is invalid_type at
// /value.
func stringValue(value json.RawMessage) (string, error) {
	var s string
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 || trimmed[0] != '"' || json.Unmarshal(trimmed, &s) != nil {
		return "", field(pointerValue, CodeInvalidType, "The value must be a JSON string.")
	}
	return s, nil
}

// repoint moves the field errors of a branding rule to /value, the member putInstanceSetting
// validates.
func repoint(err error) error {
	var e *errs.Error
	if errors.As(err, &e) {
		for i := range e.Fields {
			e.Fields[i].Pointer = pointerValue
		}
	}
	return err
}

// field returns validation_failed with one field error.
func field(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// ValidateProductName checks a product name (configuration.md section 6, SEC-IN-4) and returns it
// trimmed of white space: a blank name is required, a control, bidi control or invalid character
// is invalid_text (kernel/textcheck), more than maxChars Unicode characters is too_long; the
// pointer is /product_name. The detail never repeats the value.
func ValidateProductName(s string, maxChars int) (string, error) {
	if !utf8.ValidString(s) {
		return "", field(pointerProductName, CodeInvalidText, "The product name is not valid text.")
	}
	name := strings.TrimSpace(s)
	if name == "" {
		return "", field(pointerProductName, CodeRequired, "The product name must not be empty or blank.")
	}
	if err := textcheck.DisplayText(name); err != nil {
		var te *textcheck.Error
		detail := "The product name contains a forbidden character."
		if errors.As(err, &te) {
			detail = "The product name contains a forbidden character at position " + strconv.Itoa(te.Offset) + "."
		}
		return "", field(pointerProductName, CodeInvalidText, detail)
	}
	if utf8.RuneCountInString(name) > maxChars {
		return "", field(pointerProductName, CodeTooLong, "The product name has more than "+strconv.Itoa(maxChars)+" characters.")
	}
	return name, nil
}

// ValidateAccentColor checks that key is exactly an accent key of the design system
// (designkeys.IsAccentColor, ADR 0044); anything else, a hex value, a CSS colour or an emoji
// included, is unknown_color_key at /accent_color.
func ValidateAccentColor(key string) error {
	if !designkeys.IsAccentColor(key) {
		return field(pointerAccentColor, CodeUnknownColorKey, "The accent is not a key of the colour palette: "+strings.Join(designkeys.AccentColors(), ", ")+".")
	}
	return nil
}

// UseBrandingEndpoint is the error of putInstanceSetting for a key with RouteBranding: 400
// validation_failed with the field #path/key and code use_branding_endpoint.
func UseBrandingEndpoint() error {
	return field("#path/key", CodeUseBrandingEndpoint, "Branding keys change through PUT /instance/branding.")
}

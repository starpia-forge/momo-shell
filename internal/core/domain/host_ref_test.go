package domain

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestHostRef_ExcludesCredentials pins HostRef's exact field set. AI-facing
// projections exclude credentials by construction (design doc 16
// §핵심원칙①) -- this fails loudly if a future edit widens HostRef past
// name/labels, e.g. by adding Address or Username back in.
func TestHostRef_ExcludesCredentials(t *testing.T) {
	got := fieldNames(reflect.TypeOf(HostRef{}))
	want := []string{"Labels", "Name"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("HostRef fields = %v, want exactly %v", got, want)
	}
}

// TestAIFacingDTOs_NoSensitiveFields is a table-driven regression guard: no
// AI-facing projection may carry a field whose name suggests credential or
// secret material, however it got there.
func TestAIFacingDTOs_NoSensitiveFields(t *testing.T) {
	sensitive := []string{"password", "secret", "keypath", "authtype", "token", "passphrase", "address", "username"}

	cases := []struct {
		name string
		typ  reflect.Type
	}{
		{"HostRef", reflect.TypeOf(HostRef{})},
		{"SessionView", reflect.TypeOf(SessionView{})},
		{"MaskedChunk", reflect.TypeOf(MaskedChunk{})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, field := range fieldNames(tc.typ) {
				lower := strings.ToLower(field)
				for _, s := range sensitive {
					if strings.Contains(lower, s) {
						t.Errorf("%s has field %q, which looks credential-related (matched %q)", tc.name, field, s)
					}
				}
			}
		})
	}
}

func fieldNames(t reflect.Type) []string {
	names := make([]string, t.NumField())
	for i := range names {
		names[i] = t.Field(i).Name
	}
	sort.Strings(names)
	return names
}

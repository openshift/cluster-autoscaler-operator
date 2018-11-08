package util

import (
	"reflect"
	"testing"
)

var filterStringTests = []struct {
	label    string
	needle   string
	haystack []string
	output   []string
}{
	{
		label:    "single instance",
		needle:   "foo",
		haystack: []string{"foo", "bar", "baz"},
		output:   []string{"bar", "baz"},
	},
	{
		label:    "multiple instances",
		needle:   "foo",
		haystack: []string{"foo", "bar", "foo"},
		output:   []string{"bar"},
	},
	{
		label:    "zero instances",
		needle:   "buzz",
		haystack: []string{"foo", "bar", "foo"},
		output:   []string{"foo", "bar", "foo"},
	},
}

func TestFilterString(t *testing.T) {
	for _, tt := range filterStringTests {
		t.Run(tt.label, func(t *testing.T) {
			got := FilterString(tt.haystack, tt.needle)
			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("got %q, want %q", got, tt.output)
			}
		})
	}
}

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
	count    int
}{
	{
		label:    "single instance",
		needle:   "foo",
		haystack: []string{"foo", "bar", "baz"},
		output:   []string{"bar", "baz"},
		count:    1,
	},
	{
		label:    "multiple instances",
		needle:   "foo",
		haystack: []string{"foo", "bar", "foo"},
		output:   []string{"bar"},
		count:    2,
	},
	{
		label:    "zero instances",
		needle:   "buzz",
		haystack: []string{"foo", "bar", "foo"},
		output:   []string{"foo", "bar", "foo"},
		count:    0,
	},
}

func TestFilterString(t *testing.T) {
	for _, tt := range filterStringTests {
		tt := tt // capture range variable
		t.Run(tt.label, func(t *testing.T) {
			t.Parallel()
			got, count := FilterString(tt.haystack, tt.needle)

			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("got %q, want %q", got, tt.output)
			}

			if count != tt.count {
				t.Errorf("got count %d, want count %d", count, tt.count)
			}
		})
	}
}

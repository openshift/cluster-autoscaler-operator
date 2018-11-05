package util

// FilterString removes any instances of the needle from haystack.
func FilterString(haystack []string, needle string) []string {
	for i := range haystack {
		if haystack[i] == needle {
			haystack = append(haystack[:i], haystack[i+1:]...)
		}
	}

	return haystack
}

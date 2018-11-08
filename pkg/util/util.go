package util

// FilterString removes any instances of the needle from haystack.
func FilterString(haystack []string, needle string) []string {
	newSlice := haystack[:0] // Share the backing array.

	for _, x := range haystack {
		if x != needle {
			newSlice = append(newSlice, x)
		}
	}

	return newSlice
}

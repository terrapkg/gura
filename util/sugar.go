package util

// Return new slice with mapped values
//
// Turn every element of type `T` in `slice` into type `U` using `fn`.
func SliceMap[T, U any](slice []T, fn func(T) U) []U {
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

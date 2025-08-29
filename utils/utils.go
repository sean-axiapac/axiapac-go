package utils

func Ptr[T any](v T) *T {
	return &v
}

func Find[T any](slice []T, predicate func(*T) bool) *T {
	for i := range slice {
		if predicate(&slice[i]) {
			return &slice[i]
		}
	}
	return nil
}

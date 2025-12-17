package utils

func Filter[T any](src []T, predicate func(T) bool) []T {
	dst := make([]T, 0, len(src))
	for _, item := range src {
		if predicate(item) {
			dst = append(dst, item)
		}
	}
	return dst
}

func Map[T any, U any](src []T, mapper func(T) U) []U {
	dst := make([]U, 0, len(src))
	for _, item := range src {
		dst = append(dst, mapper(item))
	}
	return dst
}

func Find[T any](items []T, predicate func(T) bool) *T {
	for _, item := range items {
		if predicate(item) {
			return &item
		}
	}
	return nil
}

func GroupBy[T any, K comparable](items []T, keyFunc func(T) K) map[K][]T {
	result := make(map[K][]T)
	for _, item := range items {
		key := keyFunc(item)
		result[key] = append(result[key], item)
	}
	return result
}

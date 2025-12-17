package common

type StatusAPIResponse[T any] struct {
	Status bool        `json:"status"`
	Data   T           `json:"data,omitempty"`
	Error  interface{} `json:"error,omitempty"`
}

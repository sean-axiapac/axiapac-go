package common

type ErrorResponse struct {
	// Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewErrorResponse(message string) *ErrorResponse {
	return &ErrorResponse{
		Message: message,
	}
}

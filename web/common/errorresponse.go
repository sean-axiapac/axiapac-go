package common

type Error struct {
	// Code    int    `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	// Code    int    `json:"code"`
	Error *Error `json:"error"`
}

func NewErrorResponse(message string) *ErrorResponse {
	return &ErrorResponse{
		Error: &Error{Message: message},
	}
}

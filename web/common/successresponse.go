package common

type SuccessResponse struct {
	Data interface{} `json:"data"`
}

func NewSuccessResponse(data interface{}) *SuccessResponse {
	return &SuccessResponse{
		Data: data,
	}
}

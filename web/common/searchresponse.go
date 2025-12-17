package common

type Pagination struct {
	Total int64 `json:"total"`
}

type SearchResponse struct {
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

func NewSearchResponse(data interface{}, total int64) *SearchResponse {
	return &SearchResponse{
		Data: data,
		Pagination: Pagination{
			Total: total,
		},
	}
}

package common

type IdCodeDTO struct {
	ID   int32  `json:"id"`
	Code string `json:"code"`
}

type JobNoDTO struct {
	JobNo string `json:"JobNo"`
}

type FullCodeDTO struct {
	FullCode string `json:"FullCode"`
}

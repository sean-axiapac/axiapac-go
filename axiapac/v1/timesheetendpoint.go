package v1

import (
	"encoding/json"
	"fmt"

	"axiapac.com/axiapac/axiapac/v1/common"
	"axiapac.com/axiapac/axiapac/v1/common/eraid"
)

type TimesheetDTO struct {
	ID             int                `json:"Id"`
	EraId          eraid.EraID        `json:"EraId"`
	Employee       common.IdCodeDTO   `json:"Employee"`
	Date           string             `json:"Date"` // yyyy-MM-dd
	WorkedHours    *float64           `json:"WorkedHours,omitempty"`
	PaidHours      float64            `json:"PaidHours"`
	TimesheetItems []TimesheetItemDTO `json:"TimesheetItems"`
}

type TimesheetItemDTO struct {
	ID                  int                 `json:"Id"`
	Description         string              `json:"Description"`
	LabourRate          *common.IdCodeDTO   `json:"LabourRate,omitempty"`
	Cost                float64             `json:"Cost"`
	Hours               float64             `json:"Hours"`
	ChargeHours         float64             `json:"ChargeHours"`
	Job                 *common.JobNoDTO    `json:"Job,omitempty"`
	CostCentre          *common.FullCodeDTO `json:"CostCentre,omitempty"`
	PayrollTimeType     *common.IdCodeDTO   `json:"PayrollTimeType,omitempty"`
	StartTime           *string             `json:"StartTime,omitempty"`
	FinishTime          *string             `json:"FinishTime,omitempty"`
	Service             *common.IdCodeDTO   `json:"Service,omitempty"`
	ServiceIncludeHours *float64            `json:"ServiceIncludeHours,omitempty"`
}

type TimesheetEndpoint struct {
	transport *Transport
}

func (this *TimesheetEndpoint) Search(take int) (map[string]interface{}, error) {
	payload := map[string]int{"take": take}

	resp, err := this.transport.Post("/api/v1/timesheets/search", payload, nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (this *TimesheetEndpoint) Save(dto *TimesheetDTO) (*common.StatusAPIResponse[*TimesheetDTO], error) {
	resp, err := this.transport.Post(fmt.Sprintf("/api/v1/timesheets/%d", dto.ID), dto, nil)
	if err != nil {
		return nil, err
	}
	var result common.StatusAPIResponse[*TimesheetDTO]
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

package v1

import (
	"encoding/json"
)

type Timesheet struct {
	ID int `json:"id"`
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

// func (this *TimesheetEndpoint) Create(timesheet Timesheet) error {
// 	resp, err := this.transport.Post("/api/v1/timesheets", timesheet, nil)
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode >= 300 {
// 		body, _ := io.ReadAll(resp.Body)
// 		return fmt.Errorf("error: %s", body)
// 	}

// 	return nil

// }

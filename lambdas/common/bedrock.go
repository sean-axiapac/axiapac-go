package common

import (
	"encoding/json"
	"strings"
)

type BedrockParameter struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type BedrockEvent struct {
	ActionGroup string             `json:"actionGroup"`
	ApiPath     string             `json:"apiPath"`
	HTTPMethod  string             `json:"httpMethod"`
	Function    string             `json:"function"`
	Parameters  []BedrockParameter `json:"parameters"`
}

type BedrockFunctionResponse struct {
	ResponseBody interface{} `json:"responseBody"`
}

type BedrockResponseContainer struct {
	ActionGroup      string                  `json:"actionGroup"`
	Function         string                  `json:"function"`
	FunctionResponse BedrockFunctionResponse `json:"functionResponse"`
}

type BedrockOutput struct {
	MessageVersion string                   `json:"messageVersion"`
	Response       BedrockResponseContainer `json:"response"`
}

func (e *BedrockEvent) GetParameter(name string) string {
	for _, p := range e.Parameters {
		if strings.EqualFold(p.Name, name) {
			return p.Value
		}
	}
	return ""
}

func NewBedrockResponse(actionGroup, function string, results interface{}) BedrockOutput {
	resBody, _ := json.Marshal(results)
	return BedrockOutput{
		MessageVersion: "1.0",
		Response: BedrockResponseContainer{
			ActionGroup: actionGroup,
			Function:    function,
			FunctionResponse: BedrockFunctionResponse{
				ResponseBody: map[string]interface{}{
					"TEXT": map[string]string{
						"body": string(resBody),
					},
				},
			},
		},
	}
}

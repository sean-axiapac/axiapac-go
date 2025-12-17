package main

import (
	"context"
	"fmt"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
)

// Define the input structure for the tool
type WeatherInput struct {
	Location string `json:"location" jsonschema_description:"Location to get weather for"`
}

func main() {
	ctx := context.Background()

	g := genkit.Init(ctx,
		genkit.WithPlugins(&googlegenai.GoogleAI{APIKey: "AIzaSyCLF56iuoWZhjS2n3eKfhwBRO2Oa0XM0io"}),
		genkit.WithDefaultModel("googleai/gemini-2.5-flash"), // Updated model name
	)

	weatherTool := genkit.DefineTool(g, "weather", "Get current weather for a location",
		func(ctx *ai.ToolContext, input WeatherInput) (WeatherData, error) {
			// Get weather data (simulated)
			return simulateWeather(input.Location), nil
		},
	)

	resp, err := genkit.Generate(ctx, g,
		// ai.WithSystem("call all defined function before answering"),
		ai.WithPrompt("What is the weather in San Francisco?"),
		ai.WithTools(weatherTool),
		// ai.WithReturnToolRequests(true),
	)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println(resp.Text())
}

type WeatherData struct {
	Location  string  `json:"location"`
	TempC     float64 `json:"temp_c"`
	TempF     float64 `json:"temp_f"`
	Condition string  `json:"condition"`
}

func simulateWeather(location string) WeatherData {
	// In a real app, this would call a weather API
	// For demonstration, we'll return mock data
	tempC := 22.5
	if location == "Tokyo" || location == "Tokyo, Japan" {
		tempC = 24.0
	} else if location == "Paris" || location == "Paris, France" {
		tempC = 18.5
	} else if location == "New York" || location == "New York, USA" {
		tempC = 15.0
	}

	conditions := []string{"Sunny", "Partly Cloudy", "Cloudy", "Rainy", "Stormy"}
	condition := conditions[time.Now().Unix()%int64(len(conditions))]

	return WeatherData{
		Location:  location,
		TempC:     tempC,
		TempF:     tempC*9/5 + 32,
		Condition: condition,
	}
}

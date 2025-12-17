package main

import (
	"context"
	"fmt"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"google.golang.org/genai"
)

func main2() {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  "AIzaSyCLF56iuoWZhjS2n3eKfhwBRO2Oa0XM0io",
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatal(err)
	}

	// maxTokens := int32(50)
	result, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash",
		genai.Text("Explain how AI works in a few words"),
		&genai.GenerateContentConfig{
			MaxOutputTokens: 1200, // Set your desired maximum output tokens here
			Temperature:     genai.Ptr[float32](0.7),
			TopP:            genai.Ptr[float32](0.9),
			TopK:            genai.Ptr[float32](40),
			ThinkingConfig: &genai.ThinkingConfig{
				ThinkingBudget: genai.Ptr[int32](0),
			},
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	if usage := result.UsageMetadata; usage != nil {
		fmt.Printf("Prompt tokens: %d\n", usage.PromptTokenCount)
		fmt.Printf("Thoughts tokens: %d\n", usage.ThoughtsTokenCount)
		fmt.Printf("Total tokens: %d\n", usage.TotalTokenCount)
	}

	fmt.Println(result.Text())
}

func main() {
	ctx := context.Background()
	g := genkit.Init(ctx,
		genkit.WithPlugins(&googlegenai.GoogleAI{APIKey: "AIzaSyCLF56iuoWZhjS2n3eKfhwBRO2Oa0XM0io"}))

	model := googlegenai.GoogleAIModelRef("gemini-2.5-flash", &genai.GenerateContentConfig{
		// MaxOutputTokens: genai.Ptr(500),
		MaxOutputTokens: 500,
		// StopSequences:   []string{"<end>", "<fin>"},
		// Temperature:     genai.Ptr[float32](0.5),
		// TopP:            genai.Ptr[float32](0.4),
		// TopK:            genai.Ptr[float32](50),
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingBudget: genai.Ptr[int32](0),
		},
	})

	resp, err := genkit.Generate(ctx, g,
		ai.WithSystem("You are a food industry marketing consultant."),
		ai.WithPrompt("Invent a menu item for a pirate themed restaurant."),
		// ai.WithModelName("googleai/gemini-2.5-flash"),
		ai.WithModel(model),
		// ai.WithConfig(&genai.GenerateContentConfig{
		// 	MaxOutputTokens: 500,
		// 	// StopSequences:   []string{"<end>", "<fin>"},
		// 	// Temperature:     genai.Ptr[float32](0.5),
		// 	// TopP:            genai.Ptr[float32](0.4),
		// 	// TopK:            genai.Ptr[float32](50),
		// }),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// Print token usage
	if usage := resp.Usage; usage != nil {
		fmt.Printf("Prompt tokens: %d\n", usage.InputTokens)
		fmt.Printf("Thoughts tokens: %d\n", usage.ThoughtsTokens)
		fmt.Printf("Generated tokens: %d\n", usage.OutputTokens)
		fmt.Printf("Total tokens: %d\n", usage.TotalTokens)
	}

	fmt.Println(resp.Text())
}

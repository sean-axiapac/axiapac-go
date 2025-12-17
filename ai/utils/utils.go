package utils

import (
	"fmt"

	"github.com/firebase/genkit/go/ai"
)

func PrintUsage(res *ai.ModelResponse) {
	fmt.Printf("Prompt tokens: %d\n", res.Usage.InputTokens)
	fmt.Printf("Thoughts tokens: %d\n", res.Usage.ThoughtsTokens)
	fmt.Printf("Output tokens: %d\n", res.Usage.OutputTokens)
	fmt.Printf("Total tokens: %d\n", res.Usage.TotalTokens)
}

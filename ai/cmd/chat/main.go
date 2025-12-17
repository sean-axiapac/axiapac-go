//TODO
// define the process
// ai to check current step and diagnostic
// updating ui
// save conversation

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"axiapac.com/axiapac/ai/axiapac"
	"axiapac.com/axiapac/core"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/firebase/genkit/go/plugins/server"
	"google.golang.org/genai"
	"gorm.io/gorm"
)

var history = []*ai.Message{}

var model = googlegenai.GoogleAIModelRef("gemini-2.5-flash", &genai.GenerateContentConfig{
	MaxOutputTokens: 500,
	StopSequences:   []string{"<end>", "<fin>"},
	Temperature:     genai.Ptr[float32](0.0), // large (1) -> creative
	TopP:            genai.Ptr[float32](0.4), // large (1) -> diversity
	TopK:            genai.Ptr[float32](5),   // 1 -> determistic (the first one)
	ThinkingConfig: &genai.ThinkingConfig{
		ThinkingBudget: genai.Ptr[int32](0),
	},
	Tools: []*genai.Tool{
		{
			CodeExecution: &genai.ToolCodeExecution{},
		},
	},
})

type BalanceInput struct {
	Date string `json:"date" jsonschema_description:"Date in YYYY-MM-DD format"`
}

type SQLInput struct {
	Query string `json:"query"`
}

func main() {
	ctx := context.Background()

	// Initialize Genkit with the Google AI plugin. When you pass nil for the
	// Config parameter, the Google AI plugin will get the API key from the
	// GEMINI_API_KEY or GOOGLE_API_KEY environment variable, which is the recommended
	// practice.
	g := genkit.Init(ctx, genkit.WithPlugins(&googlegenai.GoogleAI{APIKey: "AIzaSyCLF56iuoWZhjS2n3eKfhwBRO2Oa0XM0io"}, &axiapac.AxiapacPlugin{}))

	// findBalance := genkit.DefineTool(g, "findBalance", "Get current bank balance",
	// 	func(ctx *ai.ToolContext, input *BalanceInput) (float64, error) {
	// 		t, err := time.Parse("2006-01-02", input.Date)
	// 		if err != nil {
	// 			return 0, fmt.Errorf("invalid date format, expected YYYY-MM-DD: %w", err)
	// 		}
	// 		return getBalance(&t)
	// 	},
	// )

	sqlExecution := genkit.DefineTool(g, "sql", "Execute a SQL query",
		func(ctx *ai.ToolContext, input SQLInput) (string, error) {
			dm, err := core.New("root:development@tcp(localhost:3306)/development?parseTime=true", 10)
			if err != nil {
				return "", nil
			}
			result := ""
			fmt.Println(input.Query)
			if err := dm.Exec(context.Background(), "development", func(db *gorm.DB) error {
				result, err = axiapac.ExecuteSQL(db, input.Query)
				return err
			}); err != nil {
				return "", err
			}

			return result, nil
		},
	)

	// Define a simple flow that generates jokes about a given topic
	bot := genkit.DefineStreamingFlow(g, "bankrec", func(ctx context.Context, input string, cb ai.ModelStreamCallback) (string, error) {
		resp, err := genkit.Generate(ctx, g,
			ai.WithModel(model),
			ai.WithSystem(`
		You are an expert financial assistant specializing in bank reconciliation. 
		Your role is to help the user compare their internal accounting records with bank statements, identify discrepancies, and suggest adjustments or explanations. 

		Guidelines:
		1. Ask for relevant details such as bank statement balances, ledger balances, outstanding checks, deposits in transit, and any fees or interest.
		2. Provide step-by-step guidance on reconciling accounts, including how to adjust for missing transactions or errors.
		3. Explain your reasoning clearly and in plain language, avoiding unnecessary technical jargon.
		4. When suggesting actions, always indicate whether it’s a confirmation, adjustment, or reconciliation step.
		5. If the user provides partial information, request only the information necessary to continue reconciliation.
		6. Do not provide financial advice unrelated to reconciliation.

		Example responses:
		- "The ledger shows $5,000 while the bank statement shows $5,200. There is an outstanding check of $150. Adjust the ledger by $50 to reconcile."
		- "You have a deposit in transit of $200 that is not reflected in the bank statement. Include it when comparing balances."

		an account is identified by code. the accountId is only for internal use. that is, when user talking about account 7220, the "7220" is account code not accountId.

		Schema Design

accounts
---------
- AccountId (INT, PK)
- Code (VARCHAR)
- Description (VARCHAR)

banking
---------
- BankingId (INT, PK, FK BankingId → accounts.AccountId)
- DivisionId (INT, FK → divisions.DivisionId)
- StatementFrequency (VARCHAR)
- LastStatementNo (INT, nullable)
- LastStatementDate (DATE)
- LastStatementBalance (DECIMAL(11,2))
- ClosingReconcileDate (DATE)
- ClosingReconcileBalance (DECIMAL(11,2))
- CurrentReconciliationDate (DATE)
- WIPNextStatementDate (DATE, nullable)
- CurrentReconciliationBalance (DECIMAL(11,2))
- BankId (INT, FK → banks.BankId)
- AccountNo (VARCHAR)
- NextChequeNo (INT, nullable)
- BankAccountType (INT)
- EFTUserNo (VARCHAR, nullable)
- EFTUserName (VARCHAR, nullable)
- EFTTraceAccountNo (VARCHAR, nullable)
- EFTSelfBalance (TINYINT)
- ShouldStartNextStatement (TINYINT)
- EFTTraceBankId (INT, FK → banks.BankId)
- DataVersion (INT, default=1)
- EraId (INT, default=1)
- Circa (DATETIME)

bankingstatements
-----------------
- BankingStatementId (INT, PK, AUTO_INCREMENT)
- BankingId (INT, FK → banking.BankingId)
- Date (DATE)
- Amount (DECIMAL(11,2))
- OpeningBalance (DECIMAL(11,2))
- ClosingBalance (DECIMAL(11,2))
- Reconciled (TINYINT, default=0)

Relationships
--------------
- banking.BankingId → accounts.AccountId
- banking.DivisionId → divisions.DivisionId
- banking.BankId → banks.BankId
- banking.EFTTraceBankId → banks.BankId
- bankingstatements.BankingId → banking.BankingId

		`),
			ai.WithStreaming(cb),
			ai.WithTools(sqlExecution),
			ai.WithMessages(history...),
			ai.WithPrompt(input))
		if err != nil {
			return "", err
		}

		displayCodeExecution(resp.Message)

		history = resp.History()

		return resp.Text(), nil
	})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /chat", genkit.Handler(bot))
	log.Fatal(server.Start(ctx, "127.0.0.1:3400", mux))
}

func displayCodeExecution(msg *ai.Message) {
	// Extract and display executable code
	code := googlegenai.GetExecutableCode(msg)
	if code == nil {
		return
	}
	fmt.Printf("Language: %s\n", code.Language)
	fmt.Printf("```%s\n%s\n```\n", code.Language, code.Code)

	// Extract and display execution results
	result := googlegenai.GetCodeExecutionResult(msg)
	fmt.Printf("\nExecution result:\n")
	fmt.Printf("Status: %s\n", result.Outcome)
	fmt.Printf("Output:\n")
	if strings.TrimSpace(result.Output) == "" {
		fmt.Printf("  <no output>\n")
	} else {
		lines := strings.SplitSeq(result.Output, "\n")
		for line := range lines {
			fmt.Printf("  %s\n", line)
		}
	}

	// Display any explanatory text
	for _, part := range msg.Content {
		if part.IsText() {
			fmt.Printf("\nExplanation:\n%s\n", part.Text)
		}
	}
}

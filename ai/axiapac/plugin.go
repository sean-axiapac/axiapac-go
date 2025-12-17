package axiapac

import (
	"context"

	"github.com/firebase/genkit/go/core/api"
)

const providerID = "axiapac"

type AxiapacPlugin struct {
}

func (p *AxiapacPlugin) Name() string {
	return providerID
}

func (m *AxiapacPlugin) Init(ctx context.Context) []api.Action {
	return []api.Action{}
}

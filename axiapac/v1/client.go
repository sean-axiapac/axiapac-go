package v1

type AxiapacClient struct {
	Transport         *Transport
	TimesheetEndpoint *TimesheetEndpoint
}

// NewAPIClient initializes the API client
func NewAxiapacClient(baseURL string, token string) *AxiapacClient {
	t := NewTransport(baseURL, token)
	return &AxiapacClient{
		Transport:         t,
		TimesheetEndpoint: &TimesheetEndpoint{transport: t},
	}
}

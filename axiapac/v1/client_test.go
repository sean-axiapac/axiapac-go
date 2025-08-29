package v1

import (
	"fmt"
	"testing"

	"axiapac.com/axiapac/security"
)

func TestTransportPost(t *testing.T) {

	token, err := security.CreateIdentityToken(&security.Identity{
		Id:       5,
		UserName: "sean",
		Provider: "local",
		Email:    "sean.tang@axiapac.com.au",
	}, "IxrAjDoa2FqElO7IhrSrUJELhUckePEPVpaePlS/Xaw=", 3600)

	if err != nil {
		t.Fatalf("failed to create identity token: %v", err)
	}

	// Create transport with test server base URL
	client := NewAxiapacClient("http://localhost", token)

	dd, err := client.TimesheetEndpoint.Search(5)
	if err != nil {
		t.Fatalf("failed to search timesheet: %v", err)
	}
	fmt.Println(dd)

}

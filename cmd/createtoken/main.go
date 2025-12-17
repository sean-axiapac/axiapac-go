package main

import (
	"fmt"

	"axiapac.com/axiapac/security"
)

func main() {
	token, _ := security.CreateIdentityToken(&security.AxiapacIdentity{
		Id:       5,
		UserName: "sean",
		Provider: "local",
		Email:    "sean.tang@axiapac.com.au",
	}, "IxrAjDoa2FqElO7IhrSrUJELhUckePEPVpaePlS/Xaw=", 3600)
	fmt.Println(token)
}

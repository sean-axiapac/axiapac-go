package helper

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"axiapac.com/axiapac/console"
)

func FindCustomerEmailByName(name string) (string, error) {
	db, err := console.Connect(context.Background())
	if err != nil {
		return "", err
	}

	subscription, err := console.FindSubscriptionByDomain(db, fmt.Sprintf("%s.axiapac.net.au", name))
	if err != nil {
		return "", err
	}
	if subscription != nil {
		email := strings.TrimSpace(subscription.Customer.Email)
		return email, nil
	}

	return "", nil
}

func FindCustomerNameFromAxiapacEmail(address string) string {
	re := regexp.MustCompile(`^([a-zA-Z0-9._%+-]+)@email\.axiapac\.net\.au$`)
	matches := re.FindStringSubmatch(address)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

package main

import (
	"fmt"
	"time"

	"axiapac.com/axiapac/web/middlewares"
)

func main() {
	token, _ := middlewares.CreateJWT("device-id", time.Hour)
	fmt.Println(token)
}

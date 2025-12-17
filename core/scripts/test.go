package main

import (
	"context"
	"log"

	"axiapac.com/axiapac/core"
)

func Dain() {
	pool, err := core.New("axiapac:Tingalpa2019@tcp(production-2.cixs43nsk6u5.ap-southeast-2.rds.amazonaws.com:3306)/", 10)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Example: handle one request for tenant1
	db, conn, err := pool.GetDB(ctx, "axiapacmvc")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close() // IMPORTANT: release connection back to pool

	var rows []map[string]interface{}
	if err := db.Raw("SELECT code, tradingname FROM entity LIMIT 1").Scan(&rows).Error; err != nil {
		log.Fatal(err)
	}
	log.Println("Tenant1 result:", rows)
}

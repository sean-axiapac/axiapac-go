package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"axiapac.com/axiapac/core"
	"gorm.io/gorm"
)

type TestCase struct {
	Site        string
	Code        string
	TradingName string
}

func main() {
	// Setup shared dm
	dm, err := core.New("axiapac:Tingalpa2019@tcp(production-2.cixs43nsk6u5.ap-southeast-2.rds.amazonaws.com:3306)/", 10)
	if err != nil {
		log.Fatal(err)
	}
	defer dm.Close()

	// Test data
	cases := []TestCase{
		{Site: "conradmartens", Code: "CONR"},
		{Site: "axiapacmvc", Code: "DEMO"},
		{Site: "axiapacinternal", Code: "APAC"},
		{Site: "nlecommercial", Code: "NLEC"},
		{Site: "wokman", Code: "WOKT"},
		{Site: "roulstonbuilders", Code: "PROJ"},
		{Site: "updaterenovate", Code: "UPD8"},
		{Site: "rjhkptyltd", Code: "RJHK"},
		{Site: "knightselectrical", Code: "MEPL"},
		{Site: "powerme", Code: "POWM"},
		{Site: "powermeservice", Code: "PSER"},
		{Site: "custommetal", Code: "CMS"},
		{Site: "sewellelectrical", Code: "SEWE"},
		{Site: "tweedvalleyelectrical", Code: "TWVE"},
		{Site: "tinuselectrical", Code: "TINU2"},
		{Site: "erismccarthy", Code: "ERIS"},
		{Site: "northernswitchboardsolutions", Code: "NSS"},
		{Site: "astarr", Code: "ASTA"},
		{Site: "wtlmgt", Code: "WTLM"},
		{Site: "data", Code: "DEMO"},
		{Site: "cairconditioningcontracting", Code: "TANFLEX"},
		{Site: "e3electrical", Code: "KAYE"},
		{Site: "paulgrey", Code: "GREY"},
		{Site: "hembrows", Code: "HEMB"},
		{Site: "ter", Code: "TELE"},
		{Site: "mbelectrical", Code: "BERG"},
		{Site: "findyourmo", Code: "FYMO"},
		{Site: "tabubilengineering", Code: "TENG"},
		{Site: "cjwproperty", Code: "CJW"},
		{Site: "pnyangsolutions", Code: "PNYA"},
		{Site: "tdc-limited", Code: "TDC"},
		{Site: "adiyap", Code: "ADIY"},
		{Site: "fcs", Code: "FCS"},
		{Site: "z-one-limited", Code: "ZONE"},
		{Site: "faiwol", Code: "FAIW"},
		{Site: "buguminvestment", Code: "BUGU"},
		{Site: "horebinkia", Code: "HORE"},
		{Site: "faiwolholdings", Code: "FAIW2"},
		{Site: "keracs", Code: "KERA"},
		{Site: "globalengineering", Code: "GLOB"},
		{Site: "makbros", Code: "MAKB"},
		{Site: "gapkon", Code: "GAPK"},
		{Site: "tumnir", Code: "TUMN"},
		{Site: "kumtex", Code: "KUMT"},
		{Site: "campadmin", Code: "CAMP"},
		{Site: "highwaytransport", Code: "HIGH"},
		{Site: "fubilansecurity", Code: "FSS"},
		{Site: "megal", Code: "MEGA"},
		{Site: "dhk-limited", Code: "DHK"},
		{Site: "kayop", Code: "KAYO"},
		{Site: "kanakumgit", Code: "KANA"},
		{Site: "ksinvestment", Code: "KSIN"},
		{Site: "simtronix", Code: "SIMT"},
		{Site: "handup", Code: "HAND"},
		{Site: "tabubilsecurity", Code: "TASS"},
		{Site: "wwconstructions", Code: "WWCO"},
		{Site: "dablan", Code: "DABL"},
		{Site: "wtlproj", Code: "WTLP"},
		{Site: "crjproperty", Code: "CRJ"},
		{Site: "wwconstructions", Code: "WWCO"},
		{Site: "crj3", Code: "CRJ3"},
		{Site: "ahengineering", Code: "AHE"},
		{Site: "jeta1", Code: "JETA1"},
		{Site: "sceestitches", Code: "SST"},
		{Site: "tapl", Code: "TAPL"},
		{Site: "ote", Code: "OTE"},
		{Site: "oktedi", Code: "OTML"},
	}

	ctx := context.Background()

	numCalls := 10 // number of times to call CheckCode per case
	var wg sync.WaitGroup
	// resultsCh := make(chan CheckResult, len(cases)*numCalls)
	for _, tc := range cases {
		for i := 0; i < numCalls; i++ {
			wg.Add(1)
			tc := tc   // capture loop variable
			index := i // capture loop variable
			go func() {
				defer wg.Done()
				code, err := CheckCode2(ctx, dm, tc.Site, tc.Code)
				// resultsCh <- CheckResult{
				// 	Site:  tc.Site,
				// 	Index: index,
				// 	Err:   err,
				// }
				if err != nil {
					fmt.Printf("[ERROR]  site[%d] %s: %v\n", index, tc.Site, err)
				} else {
					fmt.Printf("site[%d] %s, %s\n", index, tc.Site, code)
				}
			}()
		}
	}
	wg.Wait()
}

type CheckResult struct {
	Site  string
	Index int
	Err   error
}

func CheckCode(ctx context.Context, dm *core.DatabaseManager, site string, code string) (string, error) {
	db, conn, err := dm.GetDB(ctx, site)
	if err != nil {
		return "", err
	}
	defer conn.Close() // always release connection

	// Run query: adjust table/column names
	var result struct {
		Code string
	}
	if err := db.Raw("SELECT Code FROM entity LIMIT 1").Scan(&result).Error; err != nil {
		return "", err
	}

	// Compare expected
	if result.Code != code {
		return "", fmt.Errorf("unexpected code for site %s, expected %s, got %s", site, code, result.Code)
	}
	return code, nil
}

func CheckCode2(ctx context.Context, dm *core.DatabaseManager, site string, code string) (string, error) {
	var c = ""
	err := dm.Exec(ctx, site, func(db *gorm.DB) error {
		var result struct {
			Code string
		}
		if err := db.Raw("SELECT Code FROM entity LIMIT 1").Scan(&result).Error; err != nil {
			return err
		}
		// Compare expected
		if result.Code != code {
			return fmt.Errorf("unexpected code for site %s, expected %s, got %s", site, code, result.Code)
		}
		c = result.Code
		return nil
	})

	return c, err
}

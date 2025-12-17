package main

import (
	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
)

func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath:      "../../models",
		ModelPkgPath: "models",                                                           // avoid helper functions
		Mode:         gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface, // generate mode
	})

	g.WithDataTypeMap(map[string]func(gorm.ColumnType) (dataType string){
		"time": func(gorm.ColumnType) string {
			return "string"
		},
		"decimal": func(gorm.ColumnType) string {
			return "float64"
		},
	})

	gormdb, _ := gorm.Open(mysql.Open("root:development@tcp(10.37.129.2:3306)/Axiapac?parseTime=true"))
	g.UseDB(gormdb)

	g.GenerateAllTable()
	g.ApplyBasic()

	// Generate the code
	g.Execute()
}

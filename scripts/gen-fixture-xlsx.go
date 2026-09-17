//go:build ignore

package main

import (
	"log"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

func main() {
	out := filepath.Join("testdata", "fixtures", "resources.xlsx")
	f := excelize.NewFile()
	headers := []string{
		"Resurs-ID", "Produktnamn", "Kategori", "Version",
		"Enhet för klimatpåverkan",
		"A1-A3 byggproduktens klimatpåverkan GWP-GHG, typiskt värde",
		"Omräkningsfaktor", "Enhet för omräkningsfaktor", "Teknisk beskrivning",
	}
	rows := [][]any{
		{"6000000991", "Betong", "Betong", "02.07.000-fixture", "kg CO₂e/kg", 0.12, 2400, "kg/m³", "Generisk betong för stomme och grund."},
		{"6000000992", "Konstruktionsstål", "Stål", "02.07.000-fixture", "kg CO₂e/kg", 1.55, 1, "kg", "Stål för bärande konstruktioner."},
		{"6000000993", "Konstruktionsvirke", "Trä", "02.07.000-fixture", "kg CO₂e/kg", 0.08, 500, "kg/m³", "Sågat virke av furu eller gran."},
		{"6000000994", "Gipsskiva", "Byggskivor", "02.07.000-fixture", "kg CO₂e/kg", 0.24, 9, "kg/m²", "Standardgipsskiva för innerväggar."},
		{"6000000995", "Mineralull", "Isolering", "02.07.000-fixture", "kg CO₂e/kg", 0.89, 18.7, "kg/m³", "Isolering av mineralull i skivor."},
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue("Sheet1", cell, h)
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			_ = f.SetCellValue("Sheet1", cell, v)
		}
	}
	if err := f.SaveAs(out); err != nil {
		log.Fatal(err)
	}
}

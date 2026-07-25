package seed

import (
	"github.com/0funct0ry/squad/internal/seed/data"
	"github.com/brianvoe/gofakeit/v7"
)

func pickFrom(pool []string) string {
	if len(pool) == 0 {
		return ""
	}
	return pool[gofakeit.Number(0, len(pool)-1)]
}

func domainGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "healthcare", Group: "domain-lookup", Description: "Healthcare domain lookup", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"drugName", "icdCode", "hospitalName"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "drugName") {
			case "icdCode":
				return pickFrom(data.HealthcareICDCodes), nil
			case "hospitalName":
				return pickFrom(data.HealthcareHospitalNames), nil
			default:
				return pickFrom(data.HealthcareDrugNames), nil
			}
		}},
		{Name: "banking", Group: "domain-lookup", Description: "Banking domain lookup", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"bankName", "swiftCode"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "bankName") {
			case "swiftCode":
				return pickFrom(data.BankingSWIFTCodes), nil
			default:
				return pickFrom(data.BankingBankNames), nil
			}
		}},
		{Name: "construction", Group: "domain-lookup", Description: "Construction domain lookup", Affinities: []string{"TEXT"}, OptionsSchema: categoryOption([]string{"equipment", "trade", "material"}), Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "category", "equipment") {
			case "trade":
				return pickFrom(data.ConstructionTrades), nil
			case "material":
				return pickFrom(data.ConstructionMaterials), nil
			default:
				return pickFrom(data.ConstructionEquipment), nil
			}
		}},
	}
}

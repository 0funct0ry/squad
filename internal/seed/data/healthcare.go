package data

// HealthcareDrugNames is a small curated list of generic (non-branded) drug
// name examples for seed data. Public generic terminology only.
var HealthcareDrugNames = []string{
	"Acetaminophen", "Ibuprofen", "Amoxicillin", "Lisinopril", "Metformin",
	"Atorvastatin", "Amlodipine", "Omeprazole", "Albuterol", "Metoprolol",
	"Losartan", "Gabapentin", "Hydrochlorothiazide", "Sertraline", "Simvastatin",
}

// HealthcareICDCodes is a small curated list of ICD-10-shaped codes
// (format only, not a claim of clinical accuracy).
var HealthcareICDCodes = []string{
	"A00.0", "E11.9", "I10", "J45.909", "K21.9",
	"M54.5", "N39.0", "R51", "F32.9", "G43.909",
	"L20.9", "H52.4", "C34.90", "S72.001A", "Z00.00",
}

// HealthcareHospitalNames is a small curated list of generic hospital-style
// names (no real institutions).
var HealthcareHospitalNames = []string{
	"Riverside General Hospital", "St. Mary's Medical Center", "Lakeview Regional Hospital",
	"Sunrise Community Hospital", "Cedar Grove Medical Center", "Northfield Health System",
	"Willowbrook General Hospital", "Harborview Medical Center", "Pinecrest Regional Medical Center",
	"Maple Ridge Hospital",
}

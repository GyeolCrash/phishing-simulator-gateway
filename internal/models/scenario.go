package models

/*
// Define Scenario
type Scenario struct {
	Name        string
	Description string
}

// 이제 진짜 시나리오를 작성하고, ID도 반환할 수 있도록 해야 함
// Define types of scenarios
var scenarios = map[string]Scenario{
	"institution_impersonation": {
		Name:        "Institution Impersonation",
		Description: "A scenario where the user receives an call impersonating a trusted institution.",
	},
	"loan_scam": {
		Name:        "Loan Scam",
		Description: "A scenario where the user is targeted with a loan scam call.",
	},
	"delivery_notification": {
		Name:        "Delivery Notification",
		Description: "A scenario where the user receives a fake delivery notification call.",
	},
	"friends_impersonation": {
		Name:        "Friends Impersonation",
		Description: "A scenario where the user receives a call impersonating a friend in need.",
	},

	// Getter for scenarios
	func GetScenario(scenarioKey string) (Scenario, bool) {
	scenario, exists := scenarios[scenarioKey]
	return scenario, exists
}
}
*/

var scenarioList = map[string]string{
	"loan-gov-01":              "loan-gov-01",
	"org-chain-01":             "org-chain-01",
	"org-chain-02":             "org-chain-02",
	"org-chain-03":             "org-chain-03",
	"smishing-01":              "smishing-01",
	"team-bec-invoice-01":      "team-bec-invoice-01",
	"team-crypto-pump-01":      "team-crypto-pump-01",
	"team-edu-foreign-01":      "team-edu-foreign-01",
	"team-family-emergency-01": "team-family-emergency-01",
	"team-impersonation-01":    "team-impersonation-01",
	"team-market-escrow-01":    "team-market-escrow-01",
	"team-refund-01":           "team-refund-01",
	"team-spoof-sim-01":        "team-spoof-sim-01",
	"team-taxi-bill-01":        "team-taxi-bill-01",
	"team-tech-01":             "team-tech-01",
	"team-telecom-arrears-01":  "team-telecom-arrears-01",
}

func GetScenario(scenario string) (string, bool) {
	scenario, exists := scenarioList[scenario]
	return scenario, exists
}

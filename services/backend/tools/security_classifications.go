package tools

import (
	"backend/utils"
	"encoding/json"
)

// SecurityClassifications represents sectors and industries for securities
type SecurityClassifications struct {
	Sectors    []string `json:"sectors"`
	Industries []string `json:"industries"`
}

// GetSecurityClassifications returns a list of sectors and industries
func GetSecurityClassifications(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
	// Mock data for now - in a real implementation, this would come from a database
	classifications := SecurityClassifications{
		Sectors: []string{
			"Technology",
			"Healthcare",
			"Finance",
			"Consumer Discretionary",
			"Consumer Staples",
			"Energy",
			"Materials",
			"Industrials",
			"Utilities",
			"Real Estate",
			"Communication Services",
		},
		Industries: []string{
			"Software",
			"Hardware",
			"Semiconductors",
			"Pharmaceuticals",
			"Biotechnology",
			"Banks",
			"Insurance",
			"Retail",
			"Automotive",
			"Aerospace & Defense",
			"Telecommunications",
			"Media",
			"Oil & Gas",
			"Chemicals",
			"Metals & Mining",
		},
	}

	return classifications, nil
}

package strategy

import (
	"encoding/json"
	"fmt"
	"strings"

	"backend/internal/app/helpers"
	"backend/internal/data"
	"backend/internal/data/postgres"
)

// SecurityFeatureValuesArgs defines arguments for GetSecurityFeatureValues.
type SecurityFeatureValuesArgs struct {
	Field  string `json:"field"`
	Search string `json:"search,omitempty"`
}

// GetSecurityFeatureValues returns allowed values for a given SecurityFeature.
// For ticker searches it returns a slice of helpers.GetSecurityFromTickerResults.
func GetSecurityFeatureValues(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args SecurityFeatureValuesArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}

	switch strings.ToLower(args.Field) {
	case "sector":
		return postgres.GetSectors(conn)
	case "industry":
		return postgres.GetIndustries(conn)
	case "market":
		return postgres.GetMarkets(conn)
	case "locale":
		return postgres.GetLocales(conn)
	case "primaryexchange", "primary_exchange":
		return postgres.GetPrimaryExchanges(conn)
	case "ticker", "securityid":
		q, _ := json.Marshal(helpers.GetSecurityFromTickerArgs{Ticker: args.Search})
		return helpers.GetSecuritiesFromTicker(conn, 0, q)
	default:
		return nil, fmt.Errorf("unsupported field %s", args.Field)
	}
}

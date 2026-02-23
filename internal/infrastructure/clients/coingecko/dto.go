package coingecko

// Response handles the nested JSON structure returned by the /simple/price endpoint.
// Map structure: { "id": { "currency": value } }. Example: {"bitcoin": {"usd": 54321.0}}
type Response map[string]map[string]float64

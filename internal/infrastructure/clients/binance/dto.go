package binance

// TickerResponse represents the flat ticker payload from Binance's /api/v3/ticker/price.
type TickerResponse struct {
	// Symbol is the base/quote pair identifier (e.g., "BTCUSDT").
	Symbol string `json:"symbol"`

	// Price is the current mark price. Note: Binance returns this as a string to preserve precision.
	Price string `json:"price"`
}

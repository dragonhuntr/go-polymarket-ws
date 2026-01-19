package ws

type BaseMessage struct {
	EventType string `json:"event_type"`
	Market    string `json:"market,omitempty"`
	AssetID   string `json:"asset_id,omitempty"`
}

type TradeMessage struct {
	BaseMessage
	ID          string       `json:"id"`
	Outcome     string       `json:"outcome"`
	Price       string       `json:"price"`
	Side        string       `json:"side"`
	Size        string       `json:"size"`
	Timestamp   int64        `json:"timestamp"`
	MakerOrders []MakerOrder `json:"maker_orders"`
}

type MakerOrder struct {
	OrderID string `json:"order_id"`
	Price   string `json:"price"`
	Size    string `json:"size"`
}

type OrderMessage struct {
	BaseMessage
	ID        string `json:"id"`
	Side      string `json:"side"`
	Price     string `json:"price"`
	Size      string `json:"size"`
	Timestamp int64  `json:"timestamp"`
}

type BookMessage struct {
	BaseMessage
	Timestamp int64          `json:"timestamp"`
	Hash      string         `json:"hash"`
	Bids      []OrderSummary `json:"bids"`
	Asks      []OrderSummary `json:"asks"`
}

type OrderSummary struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

type PriceChangeMessage struct {
	BaseMessage
	Changes []PriceChange `json:"changes"`
}

type PriceChange struct {
	TokenID string `json:"token_id"`
	Price   string `json:"price"`
}

type TickSizeChangeMessage struct {
	BaseMessage
	TickSize string `json:"tick_size"`
}

type LastTradePriceMessage struct {
	BaseMessage
	Price     string `json:"price"`
	Timestamp int64  `json:"timestamp"`
}

type BestBidAskMessage struct {
	BaseMessage
	BestBid string `json:"best_bid"`
	BestAsk string `json:"best_ask"`
}

type NewMarketMessage struct {
	BaseMessage
	MarketID string `json:"market_id"`
}

type MarketResolvedMessage struct {
	BaseMessage
	Outcome   string `json:"outcome"`
	Timestamp int64  `json:"timestamp"`
}

type EventMessage struct {
	BaseMessage
	Data map[string]interface{} `json:"data,omitempty"`
}

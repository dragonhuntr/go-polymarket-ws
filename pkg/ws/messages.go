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
	Timestamp string         `json:"timestamp"`
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
	PriceChanges []PriceChange `json:"price_changes"`
	Timestamp    string        `json:"timestamp"`
}

type PriceChange struct {
	AssetID string `json:"asset_id"`
	Price   string `json:"price"`
	Size    string `json:"size"`
	Side    string `json:"side"`
	Hash    string `json:"hash"`
	BestBid string `json:"best_bid"`
	BestAsk string `json:"best_ask"`
}

type TickSizeChangeMessage struct {
	BaseMessage
	OldTickSize string `json:"old_tick_size"`
	NewTickSize string `json:"new_tick_size"`
	Side        string `json:"side"`
	Timestamp   string `json:"timestamp"`
}

type LastTradePriceMessage struct {
	BaseMessage
	Price      string `json:"price"`
	Side       string `json:"side"`
	Size       string `json:"size"`
	FeeRateBps string `json:"fee_rate_bps"`
	Timestamp  string `json:"timestamp"`
}

type BestBidAskMessage struct {
	BaseMessage
	BestBid   string `json:"best_bid"`
	BestAsk   string `json:"best_ask"`
	Spread    string `json:"spread"`
	Timestamp string `json:"timestamp"`
}

type NewMarketMessage struct {
	BaseMessage
	ID           string               `json:"id"`
	Question     string               `json:"question"`
	Slug         string               `json:"slug"`
	Description  string               `json:"description"`
	AssetsIDs    []string             `json:"assets_ids"`
	Outcomes     []string             `json:"outcomes"`
	EventMessage EventMessageMetadata `json:"event_message"`
	Timestamp    string               `json:"timestamp"`
}

type MarketResolvedMessage struct {
	BaseMessage
	ID             string               `json:"id"`
	Question       string               `json:"question"`
	Slug           string               `json:"slug"`
	Description    string               `json:"description"`
	AssetsIDs      []string             `json:"assets_ids"`
	Outcomes       []string             `json:"outcomes"`
	WinningAssetID string               `json:"winning_asset_id"`
	WinningOutcome string               `json:"winning_outcome"`
	EventMessage   EventMessageMetadata `json:"event_message"`
	Timestamp      string               `json:"timestamp"`
}

type EventMessageMetadata struct {
	ID          string `json:"id"`
	Ticker      string `json:"ticker"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

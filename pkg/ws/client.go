package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var (
	ErrNotConnected   = errors.New("not connected")
	ErrInvalidChannel = errors.New("invalid channel: must be user or market")
)

type WSClient struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	channel string
}

func Connect(ctx context.Context, channel string, auth *Auth, markets, assetIDs []string) (*WSClient, error) {
	url := BaseURL + channel

	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	c.SetReadLimit(10 * 1024 * 1024)

	client := &WSClient{conn: c, channel: channel}

	// Send auth + initial subscription
	if err := client.Subscribe(ctx, markets, assetIDs, auth); err != nil {
		c.Close(websocket.StatusInternalError, "handshake failed")
		return nil, fmt.Errorf("write auth/sub: %w", err)
	}

	return client, nil
}

func (c *WSClient) ReadLoop(ctx context.Context, events chan<- any) error {
	defer c.conn.Close(websocket.StatusNormalClosure, "")

	go c.pingLoop(ctx)

	for {
		messageType, data, err := c.conn.Read(ctx)
		if err != nil {
			return err
		}

		if messageType == websocket.MessageText {
			text := string(data)
			if text == "PING" || text == "PONG" {
				continue
			}
		}

		// Check if it's an array (book messages come as arrays)
		var dataToParse []byte = data
		if len(data) > 0 && data[0] == '[' {
			var arr []json.RawMessage
			if err := json.Unmarshal(data, &arr); err != nil {
				fmt.Printf("error parsing array message: %v\n", err)
				continue
			}
			if len(arr) == 0 {
				continue // no data to parse
			}
			dataToParse = arr[0]
		}

		var base struct {
			EventType string `json:"event_type"`
		}

		if err := json.Unmarshal(dataToParse, &base); err != nil {
			fmt.Printf("error parsing message: %v\n", err)
			continue
		}

		var msg any
		var parseErr error

		switch base.EventType {
		case EventTrade:
			var tradeMsg TradeMessage
			parseErr = json.Unmarshal(dataToParse, &tradeMsg)
			msg = tradeMsg
		case EventOrder:
			var orderMsg OrderMessage
			parseErr = json.Unmarshal(dataToParse, &orderMsg)
			msg = orderMsg
		case EventBook:
			var bookMsg BookMessage
			parseErr = json.Unmarshal(dataToParse, &bookMsg)
			msg = bookMsg
		case EventPriceChange:
			var priceChangeMsg PriceChangeMessage
			parseErr = json.Unmarshal(dataToParse, &priceChangeMsg)
			msg = priceChangeMsg
		case EventTickSizeChange:
			var tickSizeChangeMsg TickSizeChangeMessage
			parseErr = json.Unmarshal(dataToParse, &tickSizeChangeMsg)
			msg = tickSizeChangeMsg
		case EventLastTradePrice:
			var lastTradePriceMsg LastTradePriceMessage
			parseErr = json.Unmarshal(dataToParse, &lastTradePriceMsg)
			msg = lastTradePriceMsg
		case EventBestBidAsk:
			var bestBidAskMsg BestBidAskMessage
			parseErr = json.Unmarshal(dataToParse, &bestBidAskMsg)
			msg = bestBidAskMsg
		case EventNewMarket:
			var newMarketMsg NewMarketMessage
			parseErr = json.Unmarshal(dataToParse, &newMarketMsg)
			msg = newMarketMsg
		case EventMarketResolved:
			var marketResolvedMsg MarketResolvedMessage
			parseErr = json.Unmarshal(dataToParse, &marketResolvedMsg)
			msg = marketResolvedMsg
		default:
			// return raw JSON for unknown types
			msg = data
			continue
		}

		if parseErr != nil {
			fmt.Printf("error parsing message: %v\n", parseErr)
			continue
		}

		// Push to channel. Select ensures we don't deadlock if user stops reading.
		select {
		case events <- msg:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *WSClient) Subscribe(ctx context.Context, markets, assetIDs []string, auth *Auth) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	msg := subscriptionMessage{}

	if auth != nil {
		msg.Auth = *auth
	} else {
		msg.Operation = "subscribe"
	}

	// Only set markets for user channel, asset_ids for market channel
	switch c.channel {
	case ChannelUser:
		msg.Markets = markets
	case ChannelMarket:
		msg.AssetIDs = assetIDs
	}

	return wsjson.Write(ctx, c.conn, msg)
}

func (c *WSClient) Unsubscribe(ctx context.Context, markets, assetIDs []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	msg := subscriptionMessage{
		Operation: "unsubscribe",
	}

	switch c.channel {
	case ChannelUser:
		msg.Markets = markets
	case ChannelMarket:
		msg.AssetIDs = assetIDs
	}

	return wsjson.Write(ctx, c.conn, msg)
}

func (c *WSClient) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second) // 10 seconds in docs
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			err := c.conn.Write(ctx, websocket.MessageText, []byte("PING"))
			c.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

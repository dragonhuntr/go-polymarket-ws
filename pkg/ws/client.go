package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var (
	ErrNotConnected   = errors.New("not connected")
	ErrInvalidChannel = errors.New("invalid channel: must be user or market")
)

type Auth struct {
	APIKey     string
	Secret     string
	Passphrase string
}

type MessageHandler func(msg interface{})

type WSClient struct {
	conn           atomic.Pointer[websocket.Conn]
	baseURL        string
	channel        string
	handlers       sync.Map
	ctx            context.Context
	cancel         context.CancelFunc
	reconnect      atomic.Bool
	customFeatures bool
	connected      atomic.Bool
	wg             sync.WaitGroup

	// Protects connection configuration
	mu       sync.Mutex
	markets  []string
	assetIDs []string
	auth     *Auth

	writeMu sync.Mutex
}

type subscriptionMsg struct {
	Type           string   `json:"type"`
	Markets        []string `json:"markets,omitempty"`
	AssetIDs       []string `json:"assets_ids,omitempty"`
	Auth           *authMsg `json:"auth,omitempty"`
	CustomFeatures bool     `json:"custom_features_enabled,omitempty"`
}

type authMsg struct {
	APIKey     string `json:"apikey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

func NewClient(channel string) (*WSClient, error) {
	if channel != ChannelUser && channel != ChannelMarket {
		return nil, ErrInvalidChannel
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &WSClient{
		baseURL:        BaseURL + channel,
		channel:        channel,
		ctx:            ctx,
		cancel:         cancel,
		customFeatures: false,
	}
	c.reconnect.Store(true)

	return c, nil
}

func (c *WSClient) On(eventType string, handler MessageHandler) {
	c.handlers.Store(eventType, handler)
}

func (c *WSClient) writeSafe(ctx context.Context, msg interface{}) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	conn := c.conn.Load()
	if conn == nil {
		return ErrNotConnected
	}

	return wsjson.Write(ctx, conn, msg)
}

// pingSafe wraps the ping write with mutex protection
// Note: Polymarket uses application-level text "PING" messages instead of RFC 6455 Ping frames
// If you need standard WebSocket Ping, use: conn.Ping(ctx) instead
func (c *WSClient) pingSafe(ctx context.Context, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	conn := c.conn.Load()
	if conn == nil {
		return ErrNotConnected
	}

	return conn.Write(ctx, websocket.MessageText, data)
}

func (c *WSClient) IsConnected() bool {
	return c.connected.Load() && c.conn.Load() != nil
}

func (c *WSClient) Connect(auth *Auth, markets, assetIDs []string, customFeatures bool) error {
	dialCtx, dialCancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer dialCancel()

	conn, _, err := websocket.Dial(dialCtx, c.baseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	conn.SetReadLimit(10 * 1024 * 1024) // 10MB

	c.mu.Lock()
	c.markets = markets
	c.assetIDs = assetIDs
	c.auth = auth
	c.customFeatures = customFeatures
	c.mu.Unlock()

	var subMsg subscriptionMsg
	subMsg.Type = "subscribe"
	subMsg.CustomFeatures = customFeatures

	switch c.channel {
	case ChannelUser:
		if auth != nil {
			subMsg.Auth = &authMsg{
				APIKey:     auth.APIKey,
				Secret:     auth.Secret,
				Passphrase: auth.Passphrase,
			}
		}
	case ChannelMarket:
		subMsg.Markets = markets
		subMsg.AssetIDs = assetIDs
	}

	if err := wsjson.Write(c.ctx, conn, subMsg); err != nil {
		conn.Close(websocket.StatusInternalError, "failed to subscribe")
		return fmt.Errorf("failed to write subscription: %w", err)
	}

	c.conn.Store(conn)
	c.connected.Store(true)

	c.wg.Add(2)
	go c.readLoop()
	go c.pingLoop()

	return nil
}

func (c *WSClient) Subscribe(markets, assetIDs []string) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	msg := subscriptionMsg{
		Type:     "subscribe",
		Markets:  markets,
		AssetIDs: assetIDs,
	}

	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	return c.writeSafe(ctx, msg)
}

func (c *WSClient) Unsubscribe(markets, assetIDs []string) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	msg := subscriptionMsg{
		Type:     "unsubscribe",
		Markets:  markets,
		AssetIDs: assetIDs,
	}

	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	return c.writeSafe(ctx, msg)
}

func (c *WSClient) readLoop() {
	defer c.wg.Done()
	defer c.connected.Store(false)
	defer func() {
		if conn := c.conn.Load(); conn != nil {
			conn.Close(websocket.StatusNormalClosure, "")
		}
	}()
	// Trigger reconnect on error if context not canceled
	// Use goroutine to avoid deep recursion and clean separation
	defer func() {
		if c.reconnect.Load() && c.ctx.Err() == nil {
			go c.attemptReconnect()
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		conn := c.conn.Load()
		if conn == nil {
			return
		}

		msgType, data, err := conn.Read(c.ctx)
		if err != nil {
			return
		}

		if msgType == websocket.MessageText {
			text := string(data)
			if text == "PING" || text == "PONG" {
				continue
			}
		}

		c.handleMessage(data)
	}
}

func (c *WSClient) pingLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if !c.connected.Load() {
				continue
			}

			conn := c.conn.Load()
			if conn == nil {
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := c.pingSafe(ctx, []byte("PING"))
			cancel()
			if err != nil {
				// If ping fails, close connection to trigger readLoop error and reconnect
				if conn := c.conn.Load(); conn != nil {
					conn.Close(websocket.StatusAbnormalClosure, "ping failed")
				}
				return
			}
		}
	}
}

func (c *WSClient) attemptReconnect() {
	if !c.reconnect.Load() {
		return
	}

	delay := initialReconnectDelay
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(delay):
		}

		c.mu.Lock()
		markets := make([]string, len(c.markets))
		copy(markets, c.markets)
		assetIDs := make([]string, len(c.assetIDs))
		copy(assetIDs, c.assetIDs)
		auth := c.auth
		customFeatures := c.customFeatures
		c.mu.Unlock()

		err := c.Connect(auth, markets, assetIDs, customFeatures)
		if err == nil {
			return
		}

		delay *= reconnectBackoffMultiplier
		if delay > maxReconnectDelay {
			delay = maxReconnectDelay
		}
	}
}

func (c *WSClient) handleMessage(data []byte) {
	var base BaseMessage
	if err := json.Unmarshal(data, &base); err != nil {
		return
	}

	handlerVal, ok := c.handlers.Load(base.EventType)
	if !ok {
		return
	}

	handler, ok := handlerVal.(MessageHandler)
	if !ok {
		return
	}

	var msg interface{}
	switch base.EventType {
	case "trade":
		var tradeMsg TradeMessage
		if err := json.Unmarshal(data, &tradeMsg); err != nil {
			return
		}
		msg = tradeMsg
	case "order":
		var orderMsg OrderMessage
		if err := json.Unmarshal(data, &orderMsg); err != nil {
			return
		}
		msg = orderMsg
	case "book":
		var bookMsg BookMessage
		if err := json.Unmarshal(data, &bookMsg); err != nil {
			return
		}
		msg = bookMsg
	case "price_change":
		var priceMsg PriceChangeMessage
		if err := json.Unmarshal(data, &priceMsg); err != nil {
			return
		}
		msg = priceMsg
	case "tick_size_change":
		var tickMsg TickSizeChangeMessage
		if err := json.Unmarshal(data, &tickMsg); err != nil {
			return
		}
		msg = tickMsg
	case "last_trade_price":
		var lastTradeMsg LastTradePriceMessage
		if err := json.Unmarshal(data, &lastTradeMsg); err != nil {
			return
		}
		msg = lastTradeMsg
	case "best_bid_ask":
		var bidAskMsg BestBidAskMessage
		if err := json.Unmarshal(data, &bidAskMsg); err != nil {
			return
		}
		msg = bidAskMsg
	case "new_market":
		var newMarketMsg NewMarketMessage
		if err := json.Unmarshal(data, &newMarketMsg); err != nil {
			return
		}
		msg = newMarketMsg
	case "market_resolved":
		var resolvedMsg MarketResolvedMessage
		if err := json.Unmarshal(data, &resolvedMsg); err != nil {
			return
		}
		msg = resolvedMsg
	default:
		return
	}

	// CRITICAL: Execute handler in goroutine to prevent blocking the read loop
	// If handler takes 500ms, read loop would stop processing pings/pongs
	go handler(msg)
}

func (c *WSClient) Close() error {
	c.reconnect.Store(false)

	c.cancel()

	c.writeMu.Lock()
	conn := c.conn.Load()
	if conn != nil {
		conn.Close(websocket.StatusNormalClosure, "client closing")
	}
	c.writeMu.Unlock()

	// Wait for loops to finish
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		return errors.New("shutdown timed out")
	}
}

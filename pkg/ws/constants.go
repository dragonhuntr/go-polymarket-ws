package ws

import "time"

const (
	ChannelUser   = "/ws/user"
	ChannelMarket = "/ws/market"
	BaseURL       = "wss://ws-subscriptions-clob.polymarket.com"

	writeTimeout               = 5 * time.Second
	pingInterval               = 10 * time.Second
	initialReconnectDelay      = 1 * time.Second
	maxReconnectDelay          = 60 * time.Second
	reconnectBackoffMultiplier = 2
)

package main

import (
	"time"

	"validation-bot/module"
)

type config struct {
	Log        logConfig
	Dispatcher dispatcherConfig
	Auditor    auditorConfig
	Observer   observerConfig
	Module     moduleConfig
	Lotus      lotusConfig
}

type logConfig struct {
	Pretty bool
	Level  string
}

type dispatcherConfig struct {
	Enabled                  bool
	DatabaseConnectionString string
	PrivateKey               string
	APIAddress               string
	ListenAddr               string
	TopicName                string
	CheckInterval            time.Duration
	AuthenticationTokens     []string
	Jitter                   time.Duration
}

type auditorConfig struct {
	Enabled      bool
	TrustedPeers []string
	PrivateKey   string
	ListenAddr   string
	TopicNames   []string
	W3S          w3sConfig
}

type w3sConfig struct {
	Token        string
	RetryWait    time.Duration
	RetryWaitMax time.Duration
	RetryCount   int
}

type observerConfig struct {
	Enabled                  bool
	DatabaseConnectionString string
	TrustedPeers             []string
	RetryInterval            time.Duration
	PollInterval             time.Duration
	W3S                      w3sConfig
}

type moduleConfig struct {
	Echo       echoConfig
	QueryAsk   queryAskConfig
	Retrieval  retrievalConfig
	Traceroute tracerouteConfig
}

type echoConfig struct {
	Enabled bool
}

type tracerouteConfig struct {
	Enabled bool
}

type queryAskConfig struct {
	Enabled bool
}

type retrievalConfig struct {
	Enabled        bool
	TmpDir         string
	Timeout        time.Duration
	MinInterval    time.Duration
	MaxJobs        int64
	LocationFilter module.LocationFilterConfig
}

type lotusConfig struct {
	URL                             string
	Token                           string
	StateMarketDealsURL             string
	StateMarketDealsRefreshInterval time.Duration
	SQLInsertBatchSize              int
}

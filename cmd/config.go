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
	W3S        w3sConfig
	Database   databaseConfig
	Trust      trustConfig
	Topic      topicConfig
	DealStates dealStatesConfig
	IPInfo     ipInfoConfig
}

type logConfig struct {
	Pretty bool
	Level  string
}

type ipInfoConfig struct {
	Token string
}

type databaseConfig struct {
	ConnectionString string
}

type topicConfig struct {
	TopicName string
}

type trustConfig struct {
	Trustors      []string
	RetryInterval time.Duration
	PollInterval  time.Duration
}

type dispatcherConfig struct {
	Enabled              bool
	PrivateKey           string
	APIAddress           string
	ListenAddr           string
	CheckInterval        time.Duration
	AuthenticationTokens []string
	Jitter               time.Duration
}

type rpcClientConfig struct {
	BaseDir string
	Timeout time.Duration
}

type auditorConfig struct {
	Enabled     bool
	PrivateKey  string
	ListenAddr  string
	BiddingWait time.Duration
	RPCConfig   rpcClientConfig
}

type w3sConfig struct {
	Token                   string
	ClientRetryWait         time.Duration
	ClientRetryWaitMax      time.Duration
	ClientRetryCount        int
	SubscriberRetryInterval time.Duration
	SubscriberPollInterval  time.Duration
}

type observerConfig struct {
	Enabled       bool
	RetryInterval time.Duration
	PollInterval  time.Duration
}

type moduleConfig struct {
	Echo          echoConfig
	QueryAsk      queryAskConfig
	Retrieval     retrievalConfig
	Traceroute    tracerouteConfig
	IndexProvider indexProviderConfig
}

type echoConfig struct {
	Enabled bool
}

type indexProviderConfig struct {
	Enabled bool
}

type tracerouteConfig struct {
	Enabled bool
	UseSudo bool
}

type queryAskConfig struct {
	Enabled bool
}

type retrievalConfig struct {
	Enabled        bool
	TmpDir         string
	Timeout        time.Duration
	MaxJobs        int64
	LocationFilter module.LocationFilterConfig
}

type lotusConfig struct {
	URL   string
	Token string
}

type dealStatesConfig struct {
	DownloadURL        string
	RefreshInterval    time.Duration
	SQLInsertBatchSize int
}

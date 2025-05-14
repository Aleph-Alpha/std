package rabbit

type Config struct {
	Connection Connection
	Channel    Channel
	DeadLetter DeadLetter
}

type Connection struct {
	Host           string
	Port           uint
	User           string
	Password       string
	IsSSLEnabled   bool
	UseCert        bool
	CACertPath     string
	ClientCertPath string
	ClientKeyPath  string
	ServerName     string
}
type Channel struct {
	ExchangeName     string
	ExchangeType     string
	RoutingKey       string
	QueueName        string
	DelayToReconnect int
	PrefetchCount    int
	IsConsumer       bool
	ContentType      string
}

type DeadLetter struct {
	ExchangeName string
	QueueName    string
	RoutingKey   string
	Ttl          int
}

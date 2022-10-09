package config

type Config struct {
	Name           string
	Ip             string
	Port           string
	SocketPort     string
	TLSEnabled     bool
	CertPath       string
	LogFileEnabled bool
	LogFilePath    string
}

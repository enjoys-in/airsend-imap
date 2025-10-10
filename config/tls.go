package config

import "crypto/tls"

func (c *Config) LoadTLS() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(c.IMAP.TLS_CERT_FILE, c.IMAP.TLS_KEY_FILE)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

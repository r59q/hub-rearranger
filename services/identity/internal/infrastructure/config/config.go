// Package config validates identity configuration without logging secret values.
package config

import (
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Address, Origin, DBPath, ClientID, ClientSecret string
	AppID                                           int64
	Key                                             []byte
	Secure, Enabled                                 bool
}

func Load() (Config, error) {
	c := Config{Address: value("IDENTITY_ADDR", "127.0.0.1:8083"), Origin: value("APP_ORIGIN", "http://localhost:3000"), DBPath: value("IDENTITY_DB_PATH", "data/identity.db"), ClientID: os.Getenv("GITHUB_APP_CLIENT_ID"), ClientSecret: os.Getenv("GITHUB_APP_CLIENT_SECRET")}
	origin, err := url.Parse(c.Origin)
	if err != nil || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || origin.Path != "" || (origin.Scheme != "http" && origin.Scheme != "https") {
		return Config{}, errors.New("APP_ORIGIN must be an HTTP(S) origin without a path")
	}

	host := origin.Hostname()
	ip := net.ParseIP(host)
	c.Secure = origin.Scheme == "https"
	if !c.Secure && host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return Config{}, errors.New("HTTP identity requires a loopback APP_ORIGIN; use HTTPS elsewhere")
	}

	id, key := os.Getenv("GITHUB_APP_ID"), os.Getenv("IDENTITY_ENCRYPTION_KEY")
	if id == "" && key == "" && c.ClientID == "" && c.ClientSecret == "" {
		return c, nil
	}
	c.AppID, err = strconv.ParseInt(id, 10, 64)
	if err != nil || c.AppID <= 0 || strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(c.ClientSecret) == "" {
		return Config{}, errors.New("set GITHUB_APP_ID, GITHUB_APP_CLIENT_ID, and GITHUB_APP_CLIENT_SECRET together")
	}

	c.Key, err = base64.StdEncoding.DecodeString(key)
	if err != nil || len(c.Key) != 32 {
		return Config{}, errors.New("IDENTITY_ENCRYPTION_KEY must encode 32 random bytes in base64")
	}

	c.Enabled = true
	return c, nil
}

func value(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

package config

import (
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	ACS      ACSConfig
	Crypto   CryptoConfig
	Log      LogConfig
	Networks map[string]NetworkConfig `mapstructure:"networks"`
	Identity IdentityConfig           `mapstructure:"identity"`
	OTP      OTPConfig                `mapstructure:"otp"`
}

// OTPConfig holds OTP generation settings.
type OTPConfig struct {
	// Source is the random number source: "internal" (default) or "hsm".
	Source string `mapstructure:"source"`
	// HsmEndpointId is the HSM endpoint ID for random generation when Source is "hsm".
	HsmEndpointId int `mapstructure:"hsm_endpoint_id"`
	// SecretsManagerURL is the base URL of the secrets-manager service.
	SecretsManagerURL string `mapstructure:"secrets_manager_url"`
}

// IdentityConfig holds Identity Server connection settings.
type IdentityConfig struct {
	// URL is the base URL of the Identity Server for token verification.
	// When empty, auth middleware is disabled (dev/test only).
	URL string `mapstructure:"url"`
}

// NetworkConfig holds per-network overrides for AAV generation and flow control.
// Keys match entity.Network values: "visa", "mastercard", "amex", "discover", "unknown".
type NetworkConfig struct {
	// MasterKeyHex overrides crypto.master_key_hex for this network's AAV/CAVV generation.
	// Falls back to the global crypto.master_key_hex when empty.
	MasterKeyHex string `mapstructure:"master_key_hex"`
	// AllowFrictionless controls whether frictionless flow is permitted for this network.
	// When false, every transaction on this network is challenged regardless of risk score.
	AllowFrictionless bool `mapstructure:"allow_frictionless"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	ReadTimeout  int    `mapstructure:"read_timeout_seconds"`
	WriteTimeout int    `mapstructure:"write_timeout_seconds"`
	TLSCertFile  string `mapstructure:"tls_cert_file"`
	TLSKeyFile   string `mapstructure:"tls_key_file"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	DSN             string `mapstructure:"dsn"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime_minutes"`
}

// ACSConfig holds ACS-specific configuration.
type ACSConfig struct {
	// BaseURL is the publicly reachable URL of this ACS (used to build challenge URLs).
	BaseURL string `mapstructure:"base_url"`
	// ReferenceNumber is the EMVCo-assigned ACS Reference Number (from Letter of Approval).
	ReferenceNumber string `mapstructure:"reference_number"`
	// OperatorID is the DS-assigned operator identifier.
	OperatorID string `mapstructure:"operator_id"`
	// SupportedVersions lists supported spec versions e.g. ["2.2.0","2.3.1"]
	SupportedVersions []string `mapstructure:"supported_versions"`
	// SDKPrivateKeyFile is the path to the ACS RSA private key (PEM) used to decrypt
	// the sdkEncData JWE sent by the SDK in app-based (DeviceChannel=01) AReq messages.
	// Required for app-channel authentication. Leave empty to skip SDK decryption (dev only).
	SDKPrivateKeyFile string `mapstructure:"sdk_private_key_file"`
	// DSURL is the base URL of the Directory Server for sending RReq messages.
	// When set, the ACS sends real RReq to the DS and records the RRes response.
	// Leave empty to skip RReq delivery (test/development only).
	DSURL string `mapstructure:"ds_url"`
}

// CryptoConfig holds key material paths for CAVV/auth value generation.
type CryptoConfig struct {
	// MasterKeyHex is the ACS master key (hex-encoded) for CAVV computation.
	// In production load this from a vault/HSM, not from config files.
	MasterKeyHex string `mapstructure:"master_key_hex"`
	// PANEncryptionKeyHex is used to encrypt stored PANs at rest.
	PANEncryptionKeyHex string `mapstructure:"pan_encryption_key_hex"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug | info | warn | error
	Format string `mapstructure:"format"` // json | console
}

// Load reads configuration from file and environment variables.
// Environment variables override file values. Prefix: ACS_
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8443)
	v.SetDefault("server.read_timeout_seconds", 30)
	v.SetDefault("server.write_timeout_seconds", 30)
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime_minutes", 30)
	v.SetDefault("acs.supported_versions", []string{"2.2.0", "2.3.1"})
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("identity.url", "")
	v.SetDefault("otp.source", "internal")
	v.SetDefault("otp.secrets_manager_url", "http://secrets:8445")

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	// Environment variable binding: ACS_SERVER_PORT, ACS_DATABASE_DSN, etc.
	v.SetEnvPrefix("ACS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !stderrors.As(err, &notFound) {
			return nil, fmt.Errorf("read config: %w", err)
		}
		// Config file optional; env vars alone are sufficient
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}



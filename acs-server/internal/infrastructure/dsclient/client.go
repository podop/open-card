// Package dsclient provides an HTTP client for sending RReq messages to the Directory Server
// and receiving RRes responses, per EMV 3DS v2.3.1 protocol.
package dsclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/repository"
	"go.uber.org/zap"
)

// RReqPayload holds the fields for the Results Request message.
type RReqPayload struct {
	ACSTransactionID    uuid.UUID
	DSTransactionID     uuid.UUID
	ServerTransactionID uuid.UUID
	TransStatus         entity.TransStatus
	AuthValue           *string
	ECI                 *string
	MessageVersion      entity.SpecVersion
	Network             entity.Network // used for per-network client cert selection
}

// RResResponse holds the parsed DS response.
type RResResponse struct {
	MessageType          string `json:"messageType"`
	MessageVersion       string `json:"messageVersion"`
	ACSTransID           string `json:"acsTransID"`
	DSTransID            string `json:"dsTransID"`
	ThreeDSServerTransID string `json:"threeDSServerTransID"`
	ResultsStatus        string `json:"resultsStatus"`
}

// ClientCertProvider returns a TLS client certificate for a given network.
// Returns nil, nil if no client cert is configured for that network.
type ClientCertProvider func(network string) (*tls.Certificate, error)

// Client sends RReq messages to the DS and persists both RReq and RRes as protocol messages.
type Client struct {
	dsURL        string
	defaultHTTP  *http.Client
	certProvider ClientCertProvider
	tlsClients   map[string]*http.Client // cached per-network TLS clients
	tlsMu        sync.RWMutex
	msgRepo      repository.MessageRepository
	log          *zap.Logger
}

// New creates a DS client. If dsURL is empty, SendRReq is a no-op.
func New(dsURL string, msgRepo repository.MessageRepository, log *zap.Logger) *Client {
	return &Client{
		dsURL: dsURL,
		defaultHTTP: &http.Client{
			Timeout: 10 * time.Second,
		},
		tlsClients: make(map[string]*http.Client),
		msgRepo:    msgRepo,
		log:        log,
	}
}

// SetClientCertProvider configures per-network mutual-TLS client certificates.
func (c *Client) SetClientCertProvider(provider ClientCertProvider) {
	c.certProvider = provider
}

// httpClientForNetwork returns an HTTP client configured with the network's client
// certificate for mutual TLS, or the default client if none is configured.
func (c *Client) httpClientForNetwork(network string) *http.Client {
	if c.certProvider == nil || network == "" {
		return c.defaultHTTP
	}

	cert, err := c.certProvider(network)
	if err != nil || cert == nil {
		return c.defaultHTTP
	}

	// Build a TLS-configured client (not cached — cert may change at runtime)
	tlsCfg := &tls.Config{
		Certificates:       []tls.Certificate{*cert},
		InsecureSkipVerify: true, // dev: DS may use self-signed cert
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}
}

// SendRReq sends a Results Request to the DS, saves both RReq (outbound) and RRes (inbound)
// to the messages table. If dsURL is not configured, saves synthetic messages.
func (c *Client) SendRReq(ctx context.Context, rreq *RReqPayload) {
	if rreq == nil {
		return
	}

	now := time.Now().UTC()

	rreqMsg := map[string]interface{}{
		"messageType":          "RReq",
		"messageVersion":       string(rreq.MessageVersion),
		"acsTransID":           rreq.ACSTransactionID.String(),
		"dsTransID":            rreq.DSTransactionID.String(),
		"threeDSServerTransID": rreq.ServerTransactionID.String(),
		"transStatus":          string(rreq.TransStatus),
		"authenticationValue":  rreq.AuthValue,
		"eci":                  rreq.ECI,
	}

	// Save outbound RReq
	rreqRaw, _ := json.Marshal(rreqMsg)
	acsTransID := rreq.ACSTransactionID
	c.saveMessage(ctx, &acsTransID, "RReq", "outbound", rreqRaw, now)

	if c.dsURL == "" {
		// No DS configured — save synthetic RRes
		c.log.Debug("DS URL not configured, saving synthetic RRes",
			zap.String("acsTransID", rreq.ACSTransactionID.String()))
		rresMsg := map[string]interface{}{
			"messageType":          "RRes",
			"messageVersion":       string(rreq.MessageVersion),
			"acsTransID":           rreq.ACSTransactionID.String(),
			"dsTransID":            rreq.DSTransactionID.String(),
			"threeDSServerTransID": rreq.ServerTransactionID.String(),
			"resultsStatus":        "01",
		}
		rresRaw, _ := json.Marshal(rresMsg)
		c.saveMessage(ctx, &acsTransID, "RRes", "inbound", rresRaw, now)
		return
	}

	// Send real RReq to DS (using per-network client cert if configured)
	httpClient := c.httpClientForNetwork(string(rreq.Network))
	rresBody, err := c.postRReq(ctx, httpClient, rreqRaw)
	if err != nil {
		c.log.Error("failed to send RReq to DS", zap.Error(err),
			zap.String("acsTransID", rreq.ACSTransactionID.String()),
			zap.String("dsURL", c.dsURL))
		// Save error as synthetic RRes
		errRes := map[string]interface{}{
			"messageType":    "RRes",
			"messageVersion": string(rreq.MessageVersion),
			"acsTransID":     rreq.ACSTransactionID.String(),
			"dsTransID":      rreq.DSTransactionID.String(),
			"error":          err.Error(),
		}
		errRaw, _ := json.Marshal(errRes)
		c.saveMessage(ctx, &acsTransID, "RRes", "inbound", errRaw, now)
		return
	}

	// Save real RRes from DS
	c.saveMessage(ctx, &acsTransID, "RRes", "inbound", rresBody, now)

	c.log.Info("RReq/RRes exchange with DS completed",
		zap.String("acsTransID", rreq.ACSTransactionID.String()),
		zap.String("dsURL", c.dsURL))
}

func (c *Client) postRReq(ctx context.Context, httpClient *http.Client, body []byte) ([]byte, error) {
	url := c.dsURL + "/ds/v2/rreq"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DS request failed: %w", err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("read DS response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DS returned status %d: %s", resp.StatusCode, buf.String())
	}

	return buf.Bytes(), nil
}

func (c *Client) saveMessage(ctx context.Context, acsTransID *uuid.UUID, msgType, dir string, content []byte, ts time.Time) {
	if c.msgRepo == nil {
		return
	}
	if err := c.msgRepo.Save(ctx, &entity.Message{
		ACSTransactionID: acsTransID,
		MessageType:      msgType,
		Direction:        dir,
		Content:          content,
		CreatedAt:        ts,
	}); err != nil {
		c.log.Warn("failed to save message", zap.String("type", msgType), zap.Error(err))
	}
}


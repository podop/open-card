package handler

import (
	"encoding/base64"
	"encoding/json"
	"html/template"

	"github.com/gin-gonic/gin"
	"github.com/your-org/acs-server/internal/adapter/http/dto"
	"github.com/your-org/acs-server/internal/domain/entity"

	_ "embed"
)

// challengePageData is the template context for the OTP challenge page.
type challengePageData struct {
	ACSTransID   string
	MerchantName string
	// Amount is a pre-formatted string like "1 000.00 USD", empty when not available.
	Amount       string
	AttemptsLeft int
	// Error is non-empty when a previous OTP attempt failed.
	Error string
	// CSRFToken is a one-time token that protects form submissions against
	// cross-site request forgery and duplicate submissions.
	CSRFToken string
}

// challengeCompleteData is the template context for the auto-posting completion page.
type challengeCompleteData struct {
	NotifURL string
	// CResB64 is the base64url-encoded CRes JSON posted to the 3DS Server notification URL.
	CResB64 string
}

// Embedded template sources. go:embed resolves relative to this source file.

//go:embed templates/challenge_visa.html
var tmplVisa string

//go:embed templates/challenge_mastercard.html
var tmplMastercard string

//go:embed templates/challenge_amex.html
var tmplAmex string

//go:embed templates/challenge_discover.html
var tmplDiscover string

//go:embed templates/challenge_default.html
var tmplDefault string

//go:embed templates/challenge_oob.html
var tmplOOBDefault string

//go:embed templates/challenge_oob_visa.html
var tmplOOBVisa string

//go:embed templates/challenge_oob_mastercard.html
var tmplOOBMastercard string

//go:embed templates/challenge_oob_amex.html
var tmplOOBAmex string

//go:embed templates/challenge_oob_discover.html
var tmplOOBDiscover string

//go:embed templates/challenge_cancelled.html
var tmplCancelled string

//go:embed templates/challenge_complete.html
var tmplComplete string

//go:embed templates/challenge_fallback.html
var tmplFallback string

//go:embed templates/challenge_pin_default.html
var tmplPINDefault string

//go:embed templates/challenge_pin_visa.html
var tmplPINVisa string

//go:embed templates/challenge_pin_mastercard.html
var tmplPINMastercard string

//go:embed templates/challenge_pin_amex.html
var tmplPINAmex string

//go:embed templates/challenge_pin_discover.html
var tmplPINDiscover string

// Parsed templates — parsed once at init, reused on every request.
var (
	parsedNetworkTemplates    map[entity.Network]*template.Template
	parsedOOBNetworkTemplates map[entity.Network]*template.Template
	parsedPINNetworkTemplates map[entity.Network]*template.Template
	parsedCancelledTemplate   *template.Template
	parsedCompleteTemplate    *template.Template
	parsedFallbackTemplate    *template.Template
)

func init() {
	raw := map[entity.Network]string{
		entity.NetworkVisa:       tmplVisa,
		entity.NetworkMastercard: tmplMastercard,
		entity.NetworkAmex:       tmplAmex,
		entity.NetworkDiscover:   tmplDiscover,
	}
	parsedNetworkTemplates = make(map[entity.Network]*template.Template, len(raw)+1)
	for n, src := range raw {
		parsedNetworkTemplates[n] = template.Must(template.New(string(n)).Parse(src))
	}
	parsedNetworkTemplates["default"] = template.Must(template.New("default").Parse(tmplDefault))

	oobRaw := map[entity.Network]string{
		entity.NetworkVisa:       tmplOOBVisa,
		entity.NetworkMastercard: tmplOOBMastercard,
		entity.NetworkAmex:       tmplOOBAmex,
		entity.NetworkDiscover:   tmplOOBDiscover,
	}
	parsedOOBNetworkTemplates = make(map[entity.Network]*template.Template, len(oobRaw)+1)
	for n, src := range oobRaw {
		parsedOOBNetworkTemplates[n] = template.Must(template.New("oob_"+string(n)).Parse(src))
	}
	parsedOOBNetworkTemplates["default"] = template.Must(template.New("oob_default").Parse(tmplOOBDefault))

	pinRaw := map[entity.Network]string{
		entity.NetworkVisa:       tmplPINVisa,
		entity.NetworkMastercard: tmplPINMastercard,
		entity.NetworkAmex:       tmplPINAmex,
		entity.NetworkDiscover:   tmplPINDiscover,
	}
	parsedPINNetworkTemplates = make(map[entity.Network]*template.Template, len(pinRaw)+1)
	for n, src := range pinRaw {
		parsedPINNetworkTemplates[n] = template.Must(template.New("pin_"+string(n)).Parse(src))
	}
	parsedPINNetworkTemplates["default"] = template.Must(template.New("pin_default").Parse(tmplPINDefault))

	parsedCancelledTemplate = template.Must(template.New("cancelled").Parse(tmplCancelled))
	parsedCompleteTemplate = template.Must(template.New("complete").Parse(tmplComplete))
	parsedFallbackTemplate = template.Must(template.New("fallback").Parse(tmplFallback))
}

// renderChallengePage selects the network-specific OTP page template and writes it.
// Falls back to challenge_default.html for unknown or unconfigured networks.
func renderChallengePage(c *gin.Context, network entity.Network, data challengePageData) {
	tmpl, ok := parsedNetworkTemplates[network]
	if !ok {
		tmpl = parsedNetworkTemplates["default"]
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		_ = err // headers already sent
	}
}

// renderOOBPage renders the network-specific OOB/decoupled challenge page.
// Falls back to challenge_oob.html for unknown networks.
func renderOOBPage(c *gin.Context, network entity.Network, data challengePageData) {
	tmpl, ok := parsedOOBNetworkTemplates[network]
	if !ok {
		tmpl = parsedOOBNetworkTemplates["default"]
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		_ = err // headers already sent
	}
}

// renderCancelledPage renders a static "transaction cancelled" page (no auto-post).
func renderCancelledPage(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = parsedCancelledTemplate.Execute(c.Writer, nil)
}

// renderCompletePage renders the self-submitting completion page that delivers the
// CRes (base64url-encoded JSON) to the 3DS Server notification URL.
func renderCompletePage(c *gin.Context, notifURL string, cres dto.CResDTO) {
	cresJSON, _ := json.Marshal(cres)
	cresB64 := base64.RawURLEncoding.EncodeToString(cresJSON)
	c.Header("Content-Type", "text/html; charset=utf-8")
	parsedCompleteTemplate.Execute(c.Writer, challengeCompleteData{NotifURL: notifURL, CResB64: cresB64}) //nolint:errcheck
}

// renderPINChallengePage selects the network-specific Card PIN page template and writes it.
func renderPINChallengePage(c *gin.Context, network entity.Network, data challengePageData) {
	tmpl, ok := parsedPINNetworkTemplates[network]
	if !ok {
		tmpl = parsedPINNetworkTemplates["default"]
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		_ = err
	}
}

// renderFallbackPage renders the "Verification Unavailable" page that auto-posts
// a CRes with transStatus=U to the notification URL after a short delay.
func renderFallbackPage(c *gin.Context, data challengeCompleteData) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	parsedFallbackTemplate.Execute(c.Writer, data) //nolint:errcheck
}
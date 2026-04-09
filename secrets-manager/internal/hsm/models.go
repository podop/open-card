package hsm

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ── Nullable JSON wrappers ─────────────────────────────────────
// These wrap sql.Null* types to serialize as flat values (or null) in JSON
// instead of {"String":"...","Valid":true}.

type NullString sql.NullString

func (n NullString) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.String)
}

func (n *NullString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	n.Valid = true
	return json.Unmarshal(data, &n.String)
}

func (n *NullString) Scan(value interface{}) error {
	s := sql.NullString{String: n.String, Valid: n.Valid}
	if err := s.Scan(value); err != nil {
		return err
	}
	n.String, n.Valid = s.String, s.Valid
	return nil
}

func (n NullString) Value() (driver.Value, error) {
	return sql.NullString(n).Value()
}

type NullInt32 sql.NullInt32

func (n NullInt32) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Int32)
}

func (n *NullInt32) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	n.Valid = true
	return json.Unmarshal(data, &n.Int32)
}

func (n *NullInt32) Scan(value interface{}) error {
	s := sql.NullInt32{Int32: n.Int32, Valid: n.Valid}
	if err := s.Scan(value); err != nil {
		return err
	}
	n.Int32, n.Valid = s.Int32, s.Valid
	return nil
}

func (n NullInt32) Value() (driver.Value, error) {
	return sql.NullInt32(n).Value()
}

type NullTime sql.NullTime

func (n NullTime) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Time)
}

func (n *NullTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	n.Valid = true
	return json.Unmarshal(data, &n.Time)
}

func (n *NullTime) Scan(value interface{}) error {
	s := sql.NullTime{Time: n.Time, Valid: n.Valid}
	if err := s.Scan(value); err != nil {
		return err
	}
	n.Time, n.Valid = s.Time, s.Valid
	return nil
}

func (n NullTime) Value() (driver.Value, error) {
	return sql.NullTime(n).Value()
}

// ── Database Models ─────────────────────────────────────────────

type HSMEndpoint struct {
	ID                 int             `db:"id" json:"id"`
	Name               string          `db:"name" json:"name"`
	Model              string          `db:"model" json:"model"`
	Address            string          `db:"address" json:"address"`
	Port               int             `db:"port" json:"port"`
	Protocol           string          `db:"protocol" json:"protocol"`
	HeaderLength       int             `db:"header_length" json:"headerLength"`
	FirmwareVersion    string          `db:"firmware_version" json:"firmwareVersion"`
	HostCommandTimeout int             `db:"host_command_timeout" json:"hostCommandTimeout"`
	ConnectionTimeout  int             `db:"connection_timeout" json:"connectionTimeout"`
	MaxConnections     int             `db:"max_connections" json:"maxConnections"`
	LMKIdentifier      int             `db:"lmk_identifier" json:"lmkIdentifier"`
	Config             json.RawMessage `db:"config" json:"config"`
	Weight             int             `db:"weight" json:"weight"`
	Priority           int             `db:"priority" json:"priority"`
	Status             string          `db:"status" json:"status"`
	Capabilities       StringArray     `db:"capabilities" json:"capabilities"`
	Enabled            bool            `db:"enabled" json:"enabled"`
	GroupID            NullInt32       `db:"group_id" json:"groupId,omitempty"`
	LastHealthCheck    NullTime        `db:"last_health_check" json:"lastHealthCheck,omitempty"`
	LastHealthOK       bool            `db:"last_health_ok" json:"lastHealthOk"`
	ErrorMessage       NullString      `db:"error_message" json:"errorMessage,omitempty"`
	CreatedBy          int             `db:"created_by" json:"createdBy"`
	CreatedAt          time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt          time.Time       `db:"updated_at" json:"updatedAt"`
}

type HSMEndpointGroup struct {
	ID          int       `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	Strategy    string    `db:"strategy" json:"strategy"`
	Enabled     bool      `db:"enabled" json:"enabled"`
	CreatedBy   int       `db:"created_by" json:"createdBy"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

type HSMAuditEntry struct {
	ID           int64      `db:"id" json:"id"`
	GroupID      NullInt32  `db:"group_id" json:"groupId,omitempty"`
	EndpointID   NullInt32  `db:"endpoint_id" json:"endpointId,omitempty"`
	CommandCode  string     `db:"command_code" json:"commandCode"`
	APIOperation string     `db:"api_operation" json:"apiOperation"`
	RequestSize  int        `db:"request_size" json:"requestSize"`
	ResponseSize int        `db:"response_size" json:"responseSize"`
	ResponseCode NullString `db:"response_code" json:"responseCode,omitempty"`
	DurationMS   int        `db:"duration_ms" json:"durationMs"`
	ErrorMessage NullString `db:"error_message" json:"errorMessage,omitempty"`
	UserID       int        `db:"user_id" json:"userId"`
	IPAddress    NullString `db:"ip_address" json:"ipAddress,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"createdAt"`
}

type HSMMetrics struct {
	ID            int64           `db:"id" json:"id"`
	EndpointID    int             `db:"endpoint_id" json:"endpointId"`
	PeriodStart   time.Time       `db:"period_start" json:"periodStart"`
	PeriodEnd     time.Time       `db:"period_end" json:"periodEnd"`
	TotalRequests int             `db:"total_requests" json:"totalRequests"`
	SuccessCount  int             `db:"success_count" json:"successCount"`
	ErrorCount    int             `db:"error_count" json:"errorCount"`
	AvgLatencyMS  float64         `db:"avg_latency_ms" json:"avgLatencyMs"`
	P99LatencyMS  float64         `db:"p99_latency_ms" json:"p99LatencyMs"`
	Commands      json.RawMessage `db:"commands" json:"commands"`
	CreatedAt     time.Time       `db:"created_at" json:"createdAt"`
}

// ── Runtime Types ───────────────────────────────────────────────

type EndpointStatus struct {
	HSMEndpoint
	Pool PoolStats `json:"pool"`
}

type PoolStats struct {
	Active   int  `json:"active"`
	Idle     int  `json:"idle"`
	Total    int  `json:"total"`
	Draining bool `json:"draining"`
}

// ── API Request/Response Types ──────────────────────────────────

type KeyGenerateRequest struct {
	KeyType    string `json:"keyType"`    // ZMK, ZPK, TMK, TAK, ZEK, etc.
	Scheme     string `json:"scheme"`     // Z (variant), U (TR-31), T (TR-31 AES)
	KeyUsage   string `json:"keyUsage"`   // optional TR-31 key usage
	Algorithm  string `json:"algorithm"`  // D (DES), T (3DES), A (AES)
	ModeOfUse  string `json:"modeOfUse"`  // TR-31 mode of use
	LMKKeyType int    `json:"lmkKeyType"` // 0-based LMK key type code
}

type KeyGenerateResponse struct {
	Key        string `json:"key"`
	CheckValue string `json:"checkValue"`
	KeyScheme  string `json:"keyScheme"`
	ErrorCode  string `json:"errorCode"`
}

type RSAGenerateRequest struct {
	KeyLength      int    `json:"keyLength"`      // 320-4096 bits
	KeyType        int    `json:"keyType"`        // 0=sign, 1=key mgmt, 2=both
	PublicExponent string `json:"publicExponent"` // 3 or 65537
}

type RSAGenerateResponse struct {
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"` // encrypted under LMK
	ErrorCode  string `json:"errorCode"`
}

type KeyImportRequest struct {
	KeyType     int    `json:"keyType"`
	ZMKKey      string `json:"zmkKey"`      // ZMK under LMK
	ImportedKey string `json:"importedKey"` // key under ZMK
	Scheme      string `json:"scheme"`
}

type KeyExportRequest struct {
	KeyType     int    `json:"keyType"`
	ZMKKey      string `json:"zmkKey"`
	KeyToExport string `json:"keyToExport"` // key under LMK
	Scheme      string `json:"scheme"`
}

type KeyTranslateRequest struct {
	KeyType      int    `json:"keyType"`
	Key          string `json:"key"`
	SourceScheme string `json:"sourceScheme"`
	DestScheme   string `json:"destScheme"`
}

type CheckValueRequest struct {
	KeyType     int    `json:"keyType"`
	Key         string `json:"key"`
	CheckLength int    `json:"checkLength"` // 0=16 hex, 1=6 hex
}

type PINVerifyRequest struct {
	TPK                 string `json:"tpk"`
	PINBlock            string `json:"pinBlock"`
	PINBlockFormat      string `json:"pinBlockFormat"` // 01=ISO 9564-1
	PAN                 string `json:"pan"`
	Offset              string `json:"offset"`
	CheckLength         int    `json:"checkLength"`
	ValidationData      string `json:"validationData"`
	DecimalisationTable string `json:"decimalisationTable"`
}

type PINTranslateRequest struct {
	SourceTPK      string `json:"sourceTPK"`
	DestTPK        string `json:"destTPK"`
	MaxPINLength   int    `json:"maxPINLength"`
	SourcePINBlock string `json:"sourcePINBlock"`
	SourceFormat   string `json:"sourceFormat"`
	DestFormat     string `json:"destFormat"`
	PAN            string `json:"pan"`
}

type PINOffsetRequest struct {
	PVK                 string `json:"pvk"`
	PINBlock            string `json:"pinBlock"`
	PINBlockFormat      string `json:"pinBlockFormat"`
	CheckLength         int    `json:"checkLength"`
	PAN                 string `json:"pan"`
	DecimalisationTable string `json:"decimalisationTable"`
	ValidationData      string `json:"validationData"`
}

type PINChangeRequest struct {
	CurrentTPK      string `json:"currentTPK"`
	NewTPK          string `json:"newTPK"`
	MaxPINLength    int    `json:"maxPINLength"`
	CurrentPINBlock string `json:"currentPINBlock"`
	NewPINBlock     string `json:"newPINBlock"`
	SourceFormat    string `json:"sourceFormat"`
	DestFormat      string `json:"destFormat"`
	PAN             string `json:"pan"`
}

type CVVVerifyRequest struct {
	CVK         string `json:"cvk"`
	PAN         string `json:"pan"`
	ExpDate     string `json:"expDate"` // YYMM
	ServiceCode string `json:"serviceCode"`
	CVV         string `json:"cvv"`
}

type CVVGenerateRequest struct {
	CVK         string `json:"cvk"`
	PAN         string `json:"pan"`
	ExpDate     string `json:"expDate"`
	ServiceCode string `json:"serviceCode"`
}

type ARQCValidateRequest struct {
	SchemeID  int    `json:"schemeId"` // 0=Visa, 1=MC
	MKdac     string `json:"mkAc"`     // Master key for AC
	PAN       string `json:"pan"`
	PANSeqNo  string `json:"panSeqNo"`
	ATC       string `json:"atc"`
	UN        string `json:"un"` // unpredictable number
	TransData string `json:"transData"`
	ARQC      string `json:"arqc"`
}

type ARPCGenerateRequest struct {
	SchemeID int    `json:"schemeId"`
	MKdac    string `json:"mkAc"`
	PAN      string `json:"pan"`
	PANSeqNo string `json:"panSeqNo"`
	ATC      string `json:"atc"`
	ARQC     string `json:"arqc"`
	ARC      string `json:"arc"` // authorisation response code
}

type EMVMACRequest struct {
	MACKeyType int    `json:"macKeyType"`
	MACKey     string `json:"macKey"`
	Data       string `json:"data"`
	PAN        string `json:"pan"`
	PANSeqNo   string `json:"panSeqNo"`
	ATC        string `json:"atc"`
}

type PVVVerifyRequest struct {
	TPK            string `json:"tpk"`
	PVK            string `json:"pvk"`
	PINBlock       string `json:"pinBlock"`
	PINBlockFormat string `json:"pinBlockFormat"`
	PAN            string `json:"pan"`
	PVKI           string `json:"pvki"`
	PVV            string `json:"pvv"`
}

type TokenProvisionRequest struct {
	KeyType   string `json:"keyType"`
	Key       string `json:"key"`
	TokenData string `json:"tokenData"`
	PAN       string `json:"pan"`
}

type P2PERequest struct {
	BDK  string `json:"bdk"`
	KSN  string `json:"ksn"`
	Data string `json:"data"`
	Mode string `json:"mode"` // ECB, CBC
}

type RKLLoadKeyRequest struct {
	TMK        string `json:"tmk"` // terminal master key
	NewKey     string `json:"newKey"`
	KeyType    string `json:"keyType"` // TPK, TAK, etc.
	TerminalID string `json:"terminalId"`
}

type RawCommandRequest struct {
	EndpointID int    `json:"endpointId"`
	CommandHex string `json:"commandHex"`
}

// StringArray implements sql.Scanner and driver.Valuer for PostgreSQL text[] columns.
type StringArray []string

func (a *StringArray) Scan(src interface{}) error {
	if src == nil {
		*a = StringArray{}
		return nil
	}
	var s string
	switch v := src.(type) {
	case []byte:
		s = string(v)
	case string:
		s = v
	default:
		return fmt.Errorf("StringArray.Scan: expected []byte or string, got %T", src)
	}
	if s == "{}" || s == "" {
		*a = StringArray{}
		return nil
	}
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	*a = strings.Split(s, ",")
	return nil
}

func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	return "{" + strings.Join(a, ",") + "}", nil
}

type HSMCommandResponse struct {
	Success      bool            `json:"success"`
	ErrorCode    string          `json:"errorCode"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	Data         json.RawMessage `json:"data,omitempty"`
	DurationMS   int             `json:"durationMs"`
	EndpointID   int             `json:"endpointId"`
	CommandCode  string          `json:"commandCode"`
}

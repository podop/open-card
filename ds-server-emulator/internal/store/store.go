package store

import (
	"sort"
	"sync"
	"time"
)

// RReqRecord holds a received RReq and the generated RRes for inspection.
type RReqRecord struct {
	ID          int64                  `json:"id"`
	ACSTransID  string                 `json:"acsTransID"`
	DSTransID   string                 `json:"dsTransID"`
	ServerTxID  string                 `json:"serverTransID"`
	TransStatus string                 `json:"transStatus"`
	MsgVersion  string                 `json:"messageVersion"`
	RReq        map[string]interface{} `json:"rreq"`
	RRes        map[string]interface{} `json:"rres"`
	ReceivedAt  time.Time              `json:"receivedAt"`
}

// OReqRecord holds a received OReq and the generated ORes for inspection.
type OReqRecord struct {
	ID             int64                  `json:"id"`
	ACSOperatorID  string                 `json:"acsOperatorID"`
	ACSRefNumber   string                 `json:"acsReferenceNumber"`
	Availability   string                 `json:"availability"`
	MsgVersion     string                 `json:"messageVersion"`
	CardRangeCount int                    `json:"cardRangeCount"`
	OReq           map[string]interface{} `json:"oreq"`
	ORes           map[string]interface{} `json:"ores"`
	ReceivedAt     time.Time              `json:"receivedAt"`
}

// InMemoryStore keeps RReq/RRes and OReq/ORes records in memory for the DS emulator dashboard.
type InMemoryStore struct {
	mu         sync.RWMutex
	records    []RReqRecord
	oreqRecords []OReqRecord
	nextID     int64
	oreqNextID int64
}

func New() *InMemoryStore {
	return &InMemoryStore{nextID: 1, oreqNextID: 1}
}

func (s *InMemoryStore) Add(rec RReqRecord) RReqRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec.ID = s.nextID
	s.nextID++
	s.records = append(s.records, rec)
	// Keep last 1000 records
	if len(s.records) > 1000 {
		s.records = s.records[len(s.records)-1000:]
	}
	return rec
}

func (s *InMemoryStore) List(page, perPage int) ([]RReqRecord, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := int64(len(s.records))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	// Return in reverse chronological order
	start := len(s.records) - page*perPage
	end := len(s.records) - (page-1)*perPage
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if end > len(s.records) {
		end = len(s.records)
	}
	slice := s.records[start:end]
	// Reverse for newest-first
	out := make([]RReqRecord, len(slice))
	for i, r := range slice {
		out[len(slice)-1-i] = r
	}
	return out, total
}

func (s *InMemoryStore) Stats() map[string]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := map[string]int64{
		"total":      int64(len(s.records)),
		"oreq_total": int64(len(s.oreqRecords)),
	}
	for _, r := range s.records {
		stats["status_"+r.TransStatus]++
	}
	return stats
}

func (s *InMemoryStore) AddOReq(rec OReqRecord) OReqRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec.ID = s.oreqNextID
	s.oreqNextID++
	s.oreqRecords = append(s.oreqRecords, rec)
	if len(s.oreqRecords) > 1000 {
		s.oreqRecords = s.oreqRecords[len(s.oreqRecords)-1000:]
	}
	return rec
}

func (s *InMemoryStore) ListOReq(page, perPage int) ([]OReqRecord, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := int64(len(s.oreqRecords))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	start := len(s.oreqRecords) - page*perPage
	end := len(s.oreqRecords) - (page-1)*perPage
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if end > len(s.oreqRecords) {
		end = len(s.oreqRecords)
	}
	slice := s.oreqRecords[start:end]
	out := make([]OReqRecord, len(slice))
	for i, r := range slice {
		out[len(slice)-1-i] = r
	}
	return out, total
}

// CommLogEntry is a unified communication log entry covering all message types.
type CommLogEntry struct {
	ID          int64                  `json:"id"`
	MessageType string                 `json:"messageType"` // RReq, RRes, OReq, ORes
	Direction   string                 `json:"direction"`   // inbound, outbound
	Summary     string                 `json:"summary"`
	Content     map[string]interface{} `json:"content"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ListCommunication returns a unified, time-sorted, paginated view of all messages.
func (s *InMemoryStore) ListCommunication(page, perPage int) ([]CommLogEntry, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build unified list
	var all []CommLogEntry
	for _, r := range s.records {
		all = append(all, CommLogEntry{
			MessageType: "RReq",
			Direction:   "inbound",
			Summary:     "transStatus=" + r.TransStatus + " acsTransID=" + r.ACSTransID[:8] + "…",
			Content:     r.RReq,
			Timestamp:   r.ReceivedAt,
		})
		all = append(all, CommLogEntry{
			MessageType: "RRes",
			Direction:   "outbound",
			Summary:     "resultsStatus=01 acsTransID=" + r.ACSTransID[:8] + "…",
			Content:     r.RRes,
			Timestamp:   r.ReceivedAt,
		})
	}
	for _, r := range s.oreqRecords {
		all = append(all, CommLogEntry{
			MessageType: "OReq",
			Direction:   "outbound",
			Summary:     "availability=" + r.Availability,
			Content:     r.OReq,
			Timestamp:   r.ReceivedAt,
		})
		all = append(all, CommLogEntry{
			MessageType: "ORes",
			Direction:   "inbound",
			Summary:     "ack",
			Content:     r.ORes,
			Timestamp:   r.ReceivedAt,
		})
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp.After(all[j].Timestamp)
	})

	total := int64(len(all))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	start := (page - 1) * perPage
	end := start + perPage
	if start >= len(all) {
		return []CommLogEntry{}, total
	}
	if end > len(all) {
		end = len(all)
	}

	// Assign sequential IDs
	slice := all[start:end]
	for i := range slice {
		slice[i].ID = int64(start + i + 1)
	}
	return slice, total
}
package events

import (
	"encoding/json"
	"sync"

	"llm-api-uptime/internal/domain"
)

type Message struct {
	Event string
	Data  []byte
}

type Hub struct {
	mu          sync.RWMutex
	adminSubs   map[chan Message]struct{}
	publicSubs  map[chan Message]struct{}
}

func NewHub() *Hub {
	return &Hub{
		adminSubs:  make(map[chan Message]struct{}),
		publicSubs: make(map[chan Message]struct{}),
	}
}

func (h *Hub) SubscribeAdmin() (chan Message, func()) {
	ch := make(chan Message, 32)
	h.mu.Lock()
	h.adminSubs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.adminSubs, ch)
		h.mu.Unlock()
		close(ch)
	}
}

func (h *Hub) SubscribePublic() (chan Message, func()) {
	ch := make(chan Message, 32)
	h.mu.Lock()
	h.publicSubs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.publicSubs, ch)
		h.mu.Unlock()
		close(ch)
	}
}

func (h *Hub) BroadcastHeartbeat(m domain.Monitor, hb domain.Heartbeat) {
	if adminBytes, err := json.Marshal(hb); err == nil {
		h.sendAdmin(Message{Event: "heartbeat", Data: adminBytes})
	}
	if m.IsPublic {
		pubEv := domain.PublicHeartbeatEvent{
			MonitorID: m.ID,
			Heartbeat: domain.PublicHeartbeat{
				Status:          hb.Status,
				ModelsLatencyMs: hb.ModelsLatencyMs,
				TTFTMs:          hb.TTFTMs,
				TotalLatencyMs:  hb.TotalLatencyMs,
				TokensPerSec:    hb.TokensPerSec,
				CheckedAt:       hb.CheckedAt,
			},
		}
		if pubBytes, err := json.Marshal(pubEv); err == nil {
			h.sendPublic(Message{Event: "heartbeat", Data: pubBytes})
		}
	}
}

func (h *Hub) sendAdmin(msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.adminSubs {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (h *Hub) sendPublic(msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.publicSubs {
		select {
		case ch <- msg:
		default:
		}
	}
}

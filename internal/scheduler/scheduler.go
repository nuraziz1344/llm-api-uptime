package scheduler

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"time"

	"llm-api-uptime/internal/domain"
	"llm-api-uptime/internal/events"
	"llm-api-uptime/internal/prober"
	"llm-api-uptime/internal/repo"
)

type runner struct {
	monitor domain.Monitor
	cancel  context.CancelFunc
	done    chan struct{}
}

type Scheduler struct {
	monitors   *repo.MonitorRepo
	heartbeats *repo.HeartbeatRepo
	settings   *repo.SettingsRepo
	prober     *prober.Prober
	hub        *events.Hub

	mu      sync.Mutex
	baseCtx context.Context
	runners map[string]*runner
	pruneCh chan struct{}
}

func New(
	monitors *repo.MonitorRepo,
	heartbeats *repo.HeartbeatRepo,
	settings *repo.SettingsRepo,
	prb *prober.Prober,
	hub *events.Hub,
) *Scheduler {
	return &Scheduler{
		monitors:   monitors,
		heartbeats: heartbeats,
		settings:   settings,
		prober:     prb,
		hub:        hub,
		runners:    make(map[string]*runner),
		pruneCh:    make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	s.baseCtx = ctx
	s.mu.Unlock()

	list, err := s.monitors.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, m := range list {
		if m.IsActive {
			s.Upsert(m)
		}
	}

	go s.pruneLoop(ctx)
	return nil
}

func (s *Scheduler) Upsert(m domain.Monitor) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.runners[m.ID]; ok {
		existing.cancel()
		<-existing.done
		delete(s.runners, m.ID)
	}

	if !m.IsActive || s.baseCtx == nil {
		return
	}

	runCtx, cancel := context.WithCancel(s.baseCtx)
	r := &runner{
		monitor: m,
		cancel:  cancel,
		done:    make(chan struct{}),
	}
	s.runners[m.ID] = r
	go s.runMonitor(runCtx, r)
}

func (s *Scheduler) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.runners[id]; ok {
		existing.cancel()
		<-existing.done
		delete(s.runners, id)
	}
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	runners := make([]*runner, 0, len(s.runners))
	for id, r := range s.runners {
		runners = append(runners, r)
		delete(s.runners, id)
	}
	s.mu.Unlock()

	for _, r := range runners {
		r.cancel()
		<-r.done
	}
}

func (s *Scheduler) TriggerNow(ctx context.Context, m domain.Monitor) (domain.Heartbeat, error) {
	hb := s.prober.Probe(ctx, m)
	if err := s.heartbeats.Save(ctx, &hb); err != nil {
		return hb, err
	}
	s.hub.BroadcastHeartbeat(m, hb)
	return hb, nil
}

func (s *Scheduler) runMonitor(ctx context.Context, r *runner) {
	defer close(r.done)

	jitter := time.Duration(rand.Intn(2500)) * time.Millisecond
	select {
	case <-ctx.Done():
		return
	case <-time.After(jitter):
	}

	s.executeOnce(ctx, r.monitor)

	interval := time.Duration(r.monitor.IntervalSec) * time.Second
	if interval < 10*time.Second {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.executeOnce(ctx, r.monitor)
		}
	}
}

func (s *Scheduler) executeOnce(ctx context.Context, m domain.Monitor) {
	hb := s.prober.Probe(ctx, m)
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.heartbeats.Save(saveCtx, &hb); err != nil {
		log.Printf("save heartbeat for %s: %v", m.ID, err)
		return
	}
	s.hub.BroadcastHeartbeat(m, hb)
}

func (s *Scheduler) pruneLoop(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			appCfg, err := s.settings.GetAppSettings(ctx)
			if err != nil {
				continue
			}
			rawCutoff := time.Now().UTC().AddDate(0, 0, -appCfg.RetentionDays)
			hourlyCutoff := time.Now().UTC().AddDate(0, 0, -365)
			if deleted, err := s.heartbeats.PruneOlderThan(ctx, rawCutoff, hourlyCutoff); err == nil && deleted > 0 {
				log.Printf("pruned %d old heartbeats", deleted)
			}
		}
	}
}

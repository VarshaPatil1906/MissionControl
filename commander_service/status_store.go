package main

import (
	"sync"
	"time"
)

type MissionStatus string

const (
	StatusQueued     MissionStatus = "QUEUED"
	StatusInProgress MissionStatus = "IN_PROGRESS"
	StatusCompleted  MissionStatus = "COMPLETED"
	StatusFailed     MissionStatus = "FAILED"
)

type StatusEvent struct {
	Status  MissionStatus `json:"status"`
	Time    time.Time     `json:"time"`
	Message string        `json:"message,omitempty"` // optional
	Soldier string        `json:"soldier,omitempty"` // which soldier handled this status
}

type MissionStatusStore struct {
	mu  sync.RWMutex
	byID map[string][]StatusEvent // mission_id -> ordered events
}

func NewMissionStatusStore() *MissionStatusStore {
	return &MissionStatusStore{
		byID: make(map[string][]StatusEvent),
	}
}

// Old helper kept for compatibility (no soldier info)
func (s *MissionStatusStore) AddEvent(missionID string, status MissionStatus, msg string) {
	s.AddEventWithSoldier(missionID, status, "", msg)
}

// New helper used from main.go, includes soldier name
func (s *MissionStatusStore) AddEventWithSoldier(missionID string, status MissionStatus, soldier, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[missionID] = append(s.byID[missionID], StatusEvent{
		Status:  status,
		Time:    time.Now().UTC(),
		Message: msg,
		Soldier: soldier,
	})
}

func (s *MissionStatusStore) GetHistory(missionID string) []StatusEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := s.byID[missionID]
	out := make([]StatusEvent, len(events))
	copy(out, events)
	return out
}

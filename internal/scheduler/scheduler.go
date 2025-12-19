package scheduler

import (
	"context"
	"fmt"
	"sync"

	"github.com/robfig/cron/v3"
	"gopkg.in/telebot.v3"
)

type Job struct {
	Bot     *telebot.Bot
	ChatID  int64
	UserIDs []int64
	Message string
}

func (j *Job) Run() {
	for _, userID := range j.UserIDs {
		recipient := &telebot.User{ID: userID}
		if _, err := j.Bot.Send(recipient, j.Message); err != nil {
			// Логирование ошибки (можно использовать log или zap)
			fmt.Printf("Failed to send reminder to user %d: %v\n", userID, err)
		}
	}
}

type Scheduler struct {
	cron    *cron.Cron
	jobs    map[string]cron.EntryID
	mu      sync.RWMutex
	started bool
	Bot     *telebot.Bot // <-- добавь это
}

func NewScheduler(bot *telebot.Bot) *Scheduler {
	c := cron.New(cron.WithChain(cron.Recover(cron.DefaultLogger)))
	return &Scheduler{
		cron: c,
		jobs: make(map[string]cron.EntryID),
		Bot:  bot, // <-- здесь
	}
}

func (s *Scheduler) Start() {
	if !s.started {
		s.cron.Start()
		s.started = true
	}
}

func (s *Scheduler) AddJob(name, cronExpr string, job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[name]; exists {
		return fmt.Errorf("job with name %q already exists", name)
	}

	entryID, err := s.cron.AddJob(cronExpr, job)
	if err != nil {
		return err
	}
	s.jobs[name] = entryID
	return nil
}

func (s *Scheduler) RemoveJob(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.jobs[name]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, name)
	}
}

func (s *Scheduler) Stop(ctx context.Context) {
	if s.started {
		s.cron.Stop()
		// cron не блокирует, но можно подождать, если нужно
	}
}

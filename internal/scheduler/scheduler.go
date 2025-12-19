package scheduler

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/telebot.v3"
)

type Job struct {
	Bot       *telebot.Bot
	ChatID    int64
	Usernames []string
	Message   string
}

func (j *Job) Run() {
	if j.Bot == nil {
		fmt.Println("Bot is nil, cannot send reminder")
		return
	}

	chat := &telebot.Chat{ID: j.ChatID}

	// Собираем строку с упоминаниями через @, без HTML-обрамления
	var mentions string
	if len(j.Usernames) > 0 {
		mentions = strings.Join(j.Usernames, " ")
	}

	// Формируем текст: упоминания + сообщение
	text := mentions
	if mentions != "" {
		text += "\n"
	}
	text += "Напоминаю: " + j.Message

	// Отправляем в чат без HTML-форматирования, чтобы Telegram не ругался
	_, err := j.Bot.Send(chat, text)
	if err != nil {
		fmt.Printf("Failed to send reminder to chat %d: %v\n", j.ChatID, err)
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
	// Читаем смещение из environment

	offsetStr := os.Getenv("TZ_OFFSET")
	offsetHours := 0
	if offsetStr != "" {
		if h, err := strconv.Atoi(offsetStr); err == nil {
			offsetHours = h
		}
	}

	loc := time.FixedZone(fmt.Sprintf("UTC%+d", offsetHours), offsetHours*3600)
	c := cron.New(
		cron.WithLocation(loc),
		cron.WithChain(cron.Recover(cron.DefaultLogger)),
	)
	fmt.Println("Scheduler location:", c.Location())
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

func (s *Scheduler) ListJobs() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fmt.Println("Scheduled jobs:")
	for name, entryID := range s.jobs {
		entry := s.cron.Entry(entryID)
		fmt.Printf("- Name: %s, Next: %s, Prev: %s\n", name, entry.Next, entry.Prev)
	}
}

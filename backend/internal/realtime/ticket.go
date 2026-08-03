package realtime

import (
	"backend/internal/cache"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"
)

const ticketTTL = 30 * time.Second

type TicketService struct {
	cache *cache.Client
}

func NewTicketService(cacheClient *cache.Client) *TicketService {
	return &TicketService{cache: cacheClient}
}

func (s *TicketService) Available() bool {
	return s != nil && s.cache != nil
}

func (s *TicketService) Issue(ctx context.Context, accountID uint) (string, time.Duration, error) {
	if accountID == 0 {
		return "", 0, errors.New("invalid account id")
	}
	if !s.Available() {
		return "", 0, errors.New("realtime service requires redis")
	}

	random := make([]byte, 24)
	if _, err := rand.Read(random); err != nil {
		return "", 0, err
	}
	ticket := hex.EncodeToString(random)
	if err := s.cache.Set(ctx, ticketKey(ticket), strconv.FormatUint(uint64(accountID), 10), ticketTTL); err != nil {
		return "", 0, err
	}
	return ticket, ticketTTL, nil
}

func (s *TicketService) Consume(ctx context.Context, ticket string) (uint, error) {
	if !s.Available() {
		return 0, errors.New("realtime service requires redis")
	}
	if len(ticket) != 48 {
		return 0, errors.New("invalid websocket ticket")
	}
	value, err := s.cache.GetDel(ctx, ticketKey(ticket))
	if err != nil {
		return 0, errors.New("invalid or expired websocket ticket")
	}
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid websocket ticket")
	}
	return uint(id), nil
}

func ticketKey(ticket string) string {
	return fmt.Sprintf("videohub:realtime:ticket:%s", ticket)
}

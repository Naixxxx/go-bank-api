package scheduler

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"go-bank-api-max/internal/service"
)

type Scheduler struct {
	svc      *service.Service
	interval time.Duration
	log      *logrus.Logger
}

func New(svc *service.Service, interval time.Duration, log *logrus.Logger) *Scheduler {
	return &Scheduler{svc: svc, interval: interval, log: log}
}
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := s.svc.ProcessDuePayments(ctx)
				if err != nil {
					s.log.WithError(err).Error("scheduled payment processing failed")
				} else {
					s.log.WithField("processed", n).Info("scheduled payment processing finished")
				}
			}
		}
	}()
}

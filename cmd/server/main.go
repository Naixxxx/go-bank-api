package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"go-bank-api-max/internal/config"
	"go-bank-api-max/internal/db"
	"go-bank-api-max/internal/handler"
	"go-bank-api-max/internal/integrations"
	"go-bank-api-max/internal/repository"
	"go-bank-api-max/internal/scheduler"
	"go-bank-api-max/internal/service"
)

func main() {
	cfg := config.Load()

	log := logrus.New()

	lvl, err := logrus.ParseLevel(cfg.LogLevel)
	if err == nil {
		log.SetLevel(lvl)
	}

	log.SetFormatter(&logrus.JSONFormatter{})

	database, err := db.Connect(cfg.DSN())
	if err != nil {
		log.WithError(err).Fatal("db connect")
	}

	defer func() {
		err := database.Close()
		if err != nil {
			log.WithError(err).Error("database connection close failure")
		}
	}()

	if err := db.Migrate(database); err != nil {
		log.WithError(err).Fatal("migrate")
	}

	repo := repository.New(database)

	mailer := integrations.NewMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPFrom)

	svc := service.New(repo, cfg.JWTSecret, cfg.HMACSecret, cfg.PGPPassphrase, cfg.CreditMargin, integrations.NewCBRClient(), mailer)

	h := handler.New(svc, repo)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	scheduler.New(svc, cfg.SchedulerInterval, log).Start(ctx)

	srv := &http.Server{Addr: ":" + cfg.AppPort, Handler: h.Routes(), ReadHeaderTimeout: 5 * time.Second}

	go func() {
		log.WithField("addr", srv.Addr).Info("server started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("server")
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = srv.Shutdown(shutdownCtx)

	log.Info("server stopped")
}

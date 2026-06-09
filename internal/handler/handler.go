package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"go-bank-api-max/internal/middleware"
	"go-bank-api-max/internal/models"
	"go-bank-api-max/internal/repository"
	"go-bank-api-max/internal/service"
	"go-bank-api-max/pkg/response"
)

type Handler struct {
	svc  *service.Service
	repo *repository.Repository
}

func New(svc *service.Service, repo *repository.Repository) *Handler {
	return &Handler{svc: svc, repo: repo}
}

func (h *Handler) Routes() http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { response.JSON(w, 200, map[string]string{"status": "ok"}) }).Methods("GET")
	r.HandleFunc("/register", h.Register).Methods("POST")
	r.HandleFunc("/login", h.Login).Methods("POST")

	auth := r.PathPrefix("/").Subrouter()

	auth.Use(middleware.Auth(h.svc.JWTSecret()))
	auth.HandleFunc("/accounts", h.CreateAccount).Methods("POST")
	auth.HandleFunc("/accounts", h.Accounts).Methods("GET")
	auth.HandleFunc("/accounts/{id}", h.Account).Methods("GET")
	auth.HandleFunc("/accounts/{id}/deposit", h.Deposit).Methods("POST")
	auth.HandleFunc("/accounts/{id}/withdraw", h.Withdraw).Methods("POST")
	auth.HandleFunc("/accounts/{id}/transactions", h.Transactions).Methods("GET")
	auth.HandleFunc("/accounts/{id}/cards", h.CardsByAccount).Methods("GET")
	auth.HandleFunc("/accounts/{id}/predict", h.Predict).Methods("GET")
	auth.HandleFunc("/transfer", h.Transfer).Methods("POST")
	auth.HandleFunc("/cards", h.CreateCard).Methods("POST")
	auth.HandleFunc("/cards/{id}", h.Card).Methods("GET")
	auth.HandleFunc("/cards/pay", h.PayByCard).Methods("POST")
	auth.HandleFunc("/credits", h.CreateCredit).Methods("POST")
	auth.HandleFunc("/credits/{creditId}/schedule", h.CreditSchedule).Methods("GET")
	auth.HandleFunc("/analytics", h.Analytics).Methods("GET")

	return r
}

func decode(r *http.Request, dst any) error {
	defer func() {
		err := r.Body.Close()
		if err != nil {
			logrus.WithError(err).Error("body close failure")
		}
	}()

	return json.NewDecoder(r.Body).Decode(dst)
}

func uid(r *http.Request) int64 { id, _ := middleware.UserID(r); return id }

func idVar(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(mux.Vars(r)[name], 10, 64)
}

func handleErr(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, repository.ErrNotFound):
		response.Error(w, 404, err.Error())
	case errors.Is(err, repository.ErrAlreadyExists):
		response.Error(w, 409, err.Error())
	case errors.Is(err, repository.ErrForbidden):
		response.Error(w, 403, err.Error())
	case errors.Is(err, repository.ErrInsufficientFunds):
		response.Error(w, 400, err.Error())
	default:
		response.Error(w, 400, err.Error())
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest

	if err := decode(r, &req); err != nil {
		response.Error(w, 400, "bad json")

		return
	}

	out, err := h.svc.Register(r.Context(), req)
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 201, out)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest

	if err := decode(r, &req); err != nil {
		response.Error(w, 400, "bad json")

		return
	}

	out, err := h.svc.Login(r.Context(), req)
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 200, out)
}

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.CreateAccount(r.Context(), uid(r))

	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 201, out)
}

func (h *Handler) Accounts(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Accounts(r.Context(), uid(r))
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 200, out)
}

func (h *Handler) Account(w http.ResponseWriter, r *http.Request) {
	id, err := idVar(r, "id")
	if err != nil {
		response.Error(w, 400, "bad id")

		return
	}

	out, err := h.svc.Account(r.Context(), uid(r), id)
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 200, out)
}

func (h *Handler) Deposit(w http.ResponseWriter, r *http.Request) {
	id, err := idVar(r, "id")
	if err != nil {
		response.Error(w, 400, "bad id")

		return
	}

	var req models.MoneyRequest

	if err := decode(r, &req); err != nil {
		response.Error(w, 400, "bad json")

		return
	}

	out, err := h.svc.Deposit(r.Context(), uid(r), id, req)
	if err != nil {
		handleErr(w, err)
		return
	}

	response.JSON(w, 200, out)
}
func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {
	id, err := idVar(r, "id")
	if err != nil {
		response.Error(w, 400, "bad id")

		return
	}

	var req models.MoneyRequest

	if err := decode(r, &req); err != nil {
		response.Error(w, 400, "bad json")

		return
	}

	out, err := h.svc.Withdraw(r.Context(), uid(r), id, req)
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 200, out)
}
func (h *Handler) Transactions(w http.ResponseWriter, r *http.Request) {
	id, err := idVar(r, "id")

	if err != nil {
		response.Error(w, 400, "bad id")

		return
	}

	out, err := h.svc.Transactions(r.Context(), uid(r), id)
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 200, out)
}

func (h *Handler) Transfer(w http.ResponseWriter, r *http.Request) {
	var req models.TransferRequest

	if err := decode(r, &req); err != nil {
		response.Error(w, 400, "bad json")

		return
	}

	if err := h.svc.Transfer(r.Context(), uid(r), req); err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) CreateCard(w http.ResponseWriter, r *http.Request) {
	var req models.CreateCardRequest

	if err := decode(r, &req); err != nil {
		response.Error(w, 400, "bad json")

		return
	}

	out, err := h.svc.CreateCard(r.Context(), uid(r), req)
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 201, out)
}

func (h *Handler) CardsByAccount(w http.ResponseWriter, r *http.Request) {
	id, err := idVar(r, "id")
	if err != nil {
		response.Error(w, 400, "bad id")

		return
	}

	out, err := h.svc.CardsByAccount(r.Context(), uid(r), id)
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 200, out)
}

func (h *Handler) Card(w http.ResponseWriter, r *http.Request) {
	id, err := idVar(r, "id")
	if err != nil {
		response.Error(w, 400, "bad id")

		return
	}

	out, err := h.svc.Card(r.Context(), uid(r), id)
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 200, out)
}

func (h *Handler) PayByCard(w http.ResponseWriter, r *http.Request) {
	var req models.CardPaymentRequest
	if err := decode(r, &req); err != nil {
		response.Error(w, 400, "bad json")

		return
	}

	if err := h.svc.PayByCard(r.Context(), uid(r), req); err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 200, map[string]string{"status": "paid"})
}

func (h *Handler) CreateCredit(w http.ResponseWriter, r *http.Request) {
	var req models.CreateCreditRequest
	if err := decode(r, &req); err != nil {
		response.Error(w, 400, "bad json")

		return
	}

	u, _ := h.repo.GetUserByID(r.Context(), uid(r))

	out, err := h.svc.CreateCredit(r.Context(), uid(r), req, u.Email)
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 201, out)
}

func (h *Handler) CreditSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := idVar(r, "creditId")
	if err != nil {
		response.Error(w, 400, "bad id")

		return
	}

	out, err := h.svc.CreditSchedule(r.Context(), uid(r), id)
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 200, out)
}

func (h *Handler) Analytics(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Analytics(r.Context(), uid(r))
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 200, out)
}

func (h *Handler) Predict(w http.ResponseWriter, r *http.Request) {
	id, err := idVar(r, "id")
	if err != nil {
		response.Error(w, 400, "bad id")

		return
	}

	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days == 0 {
		days = 30
	}

	out, err := h.svc.Predict(r.Context(), uid(r), id, days)
	if err != nil {
		handleErr(w, err)

		return
	}

	response.JSON(w, 200, out)
}

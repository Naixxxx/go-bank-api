package models

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

var emailRx = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r RegisterRequest) Validate() error {
	if !emailRx.MatchString(strings.TrimSpace(r.Email)) {
		return errors.New("invalid email")
	}

	if len(strings.TrimSpace(r.Username)) < 3 || len(r.Username) > 32 {
		return errors.New("username must be 3..32 characters")
	}

	if len(r.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	return nil
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type Account struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Number         string    `json:"number"`
	Currency       string    `json:"currency"`
	BalanceKopecks int64     `json:"balance_kopecks"`
	Balance        float64   `json:"balance"`
	IsBlocked      bool      `json:"is_blocked"`
	CreatedAt      time.Time `json:"created_at"`
}

type MoneyRequest struct {
	Amount float64 `json:"amount"`
}

func (r MoneyRequest) Validate() error {
	if r.Amount <= 0 {
		return errors.New("amount must be positive")
	}

	return nil
}

type TransferRequest struct {
	FromAccountID int64   `json:"from_account_id"`
	ToAccountID   int64   `json:"to_account_id"`
	Amount        float64 `json:"amount"`
	Description   string  `json:"description"`
}

func (r TransferRequest) Validate() error {
	if r.FromAccountID <= 0 || r.ToAccountID <= 0 || r.FromAccountID == r.ToAccountID {
		return errors.New("invalid accounts")
	}

	if r.Amount <= 0 {
		return errors.New("amount must be positive")
	}

	return nil
}

type Card struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	AccountID int64     `json:"account_id"`
	Number    string    `json:"number,omitempty"`
	Expiry    string    `json:"expiry,omitempty"`
	Masked    string    `json:"masked"`
	Last4     string    `json:"last4"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateCardRequest struct {
	AccountID int64 `json:"account_id"`
}

type CardPaymentRequest struct {
	CardID      int64   `json:"card_id"`
	CVV         string  `json:"cvv"`
	Amount      float64 `json:"amount"`
	Merchant    string  `json:"merchant"`
	Description string  `json:"description"`
}

type Credit struct {
	ID                    int64     `json:"id"`
	UserID                int64     `json:"user_id"`
	AccountID             int64     `json:"account_id"`
	PrincipalKopecks      int64     `json:"principal_kopecks"`
	Principal             float64   `json:"principal"`
	AnnualRate            float64   `json:"annual_rate"`
	Months                int       `json:"months"`
	AnnuityPaymentKopecks int64     `json:"annuity_payment_kopecks"`
	AnnuityPayment        float64   `json:"annuity_payment"`
	Status                string    `json:"status"`
	CreatedAt             time.Time `json:"created_at"`
}

type CreateCreditRequest struct {
	AccountID int64   `json:"account_id"`
	Amount    float64 `json:"amount"`
	Months    int     `json:"months"`
}

func (r CreateCreditRequest) Validate() error {
	if r.AccountID <= 0 {
		return errors.New("invalid account")
	}

	if r.Amount <= 0 {
		return errors.New("amount must be positive")
	}

	if r.Months <= 0 || r.Months > 360 {
		return errors.New("months must be 1..360")
	}

	return nil
}

type PaymentSchedule struct {
	ID                   int64      `json:"id"`
	CreditID             int64      `json:"credit_id"`
	AccountID            int64      `json:"account_id"`
	DueDate              time.Time  `json:"due_date"`
	PaymentKopecks       int64      `json:"payment_kopecks"`
	Payment              float64    `json:"payment"`
	PrincipalPartKopecks int64      `json:"principal_part_kopecks"`
	InterestPartKopecks  int64      `json:"interest_part_kopecks"`
	PenaltyKopecks       int64      `json:"penalty_kopecks"`
	Status               string     `json:"status"`
	PaidAt               *time.Time `json:"paid_at,omitempty"`
}

type Transaction struct {
	ID               int64     `json:"id"`
	AccountID        int64     `json:"account_id"`
	RelatedAccountID *int64    `json:"related_account_id,omitempty"`
	AmountKopecks    int64     `json:"amount_kopecks"`
	Amount           float64   `json:"amount"`
	Type             string    `json:"type"`
	Description      string    `json:"description"`
	CreatedAt        time.Time `json:"created_at"`
}

type Analytics struct {
	MonthIncome          float64 `json:"month_income"`
	MonthExpense         float64 `json:"month_expense"`
	ActiveCreditDebt     float64 `json:"active_credit_debt"`
	MonthlyCreditPayment float64 `json:"monthly_credit_payment"`
	CreditLoadPercent    float64 `json:"credit_load_percent"`
}

type BalancePrediction struct {
	AccountID        int64   `json:"account_id"`
	Days             int     `json:"days"`
	CurrentBalance   float64 `json:"current_balance"`
	ScheduledOutflow float64 `json:"scheduled_outflow"`
	PredictedBalance float64 `json:"predicted_balance"`
}

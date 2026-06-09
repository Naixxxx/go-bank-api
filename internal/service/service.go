package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-bank-api-max/internal/integrations"
	"go-bank-api-max/internal/models"
	"go-bank-api-max/internal/repository"
	"go-bank-api-max/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo          *repository.Repository
	jwtSecret     []byte
	hmacSecret    []byte
	pgpPassphrase string
	margin        float64
	cbr           *integrations.CBRClient
	mailer        *integrations.Mailer
}

func New(repo *repository.Repository, jwtSecret, hmacSecret, pgpPassphrase string, margin float64, cbr *integrations.CBRClient, mailer *integrations.Mailer) *Service {
	return &Service{repo: repo, jwtSecret: []byte(jwtSecret), hmacSecret: []byte(hmacSecret), pgpPassphrase: pgpPassphrase, margin: margin, cbr: cbr, mailer: mailer}
}

func (s *Service) JWTSecret() []byte { return s.jwtSecret }

func (s *Service) Register(ctx context.Context, req models.RegisterRequest) (models.AuthResponse, error) {
	if err := req.Validate(); err != nil {
		return models.AuthResponse{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return models.AuthResponse{}, err
	}

	u, err := s.repo.CreateUser(ctx, strings.ToLower(strings.TrimSpace(req.Email)), strings.TrimSpace(req.Username), string(hash))
	if err != nil {
		return models.AuthResponse{}, err
	}

	tok, err := s.token(u.ID)
	if err != nil {
		return models.AuthResponse{}, err
	}

	return models.AuthResponse{Token: tok, User: u}, nil
}

func (s *Service) Login(ctx context.Context, req models.LoginRequest) (models.AuthResponse, error) {
	u, err := s.repo.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		return models.AuthResponse{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return models.AuthResponse{}, repository.ErrForbidden
	}

	tok, err := s.token(u.ID)
	if err != nil {
		return models.AuthResponse{}, err
	}

	return models.AuthResponse{Token: tok, User: u}, nil
}

func (s *Service) token(userID int64) (string, error) {
	claims := jwt.RegisteredClaims{Subject: fmt.Sprint(userID), ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), IssuedAt: jwt.NewNumericDate(time.Now())}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func (s *Service) CreateAccount(ctx context.Context, userID int64) (models.Account, error) {
	return s.repo.CreateAccount(ctx, userID, fmt.Sprintf("40817810%012d", time.Now().UnixNano()%1_000_000_000_000))
}

func (s *Service) Accounts(ctx context.Context, userID int64) ([]models.Account, error) {
	return s.repo.ListAccounts(ctx, userID)
}

func (s *Service) Account(ctx context.Context, userID, id int64) (models.Account, error) {
	a, err := s.repo.GetAccount(ctx, id)
	if err != nil {
		return a, err
	}

	if a.UserID != userID {
		return a, repository.ErrForbidden
	}

	return a, nil
}

func (s *Service) Deposit(ctx context.Context, userID, accountID int64, req models.MoneyRequest) (models.Account, error) {
	if err := req.Validate(); err != nil {
		return models.Account{}, err
	}

	return s.repo.Deposit(ctx, userID, accountID, utils.ToKopecks(req.Amount), "deposit", "Deposit")
}
func (s *Service) Withdraw(ctx context.Context, userID, accountID int64, req models.MoneyRequest) (models.Account, error) {
	if err := req.Validate(); err != nil {
		return models.Account{}, err
	}

	return s.repo.Withdraw(ctx, userID, accountID, utils.ToKopecks(req.Amount), "withdraw", "Withdrawal")
}
func (s *Service) Transfer(ctx context.Context, userID int64, req models.TransferRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}

	return s.repo.Transfer(ctx, userID, req.FromAccountID, req.ToAccountID, utils.ToKopecks(req.Amount), req.Description)
}
func (s *Service) Transactions(ctx context.Context, userID, accountID int64) ([]models.Transaction, error) {
	return s.repo.ListTransactions(ctx, userID, accountID)
}

func (s *Service) CreateCard(ctx context.Context, userID int64, req models.CreateCardRequest) (map[string]any, error) {
	if req.AccountID <= 0 {
		return nil, fmt.Errorf("invalid account")
	}

	number, err := utils.GenerateCardNumber()
	if err != nil {
		return nil, err
	}

	expiry := utils.Expiry()

	cvv, err := utils.GenerateCVV()
	if err != nil {
		return nil, err
	}

	cvvHash, err := bcrypt.GenerateFromPassword([]byte(cvv), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	card, err := s.repo.CreateCard(ctx, userID, req.AccountID, number, expiry, string(cvvHash), utils.ComputeHMAC(number, s.hmacSecret), utils.ComputeHMAC(expiry, s.hmacSecret), number[len(number)-4:], s.pgpPassphrase)
	if err != nil {
		return nil, err
	}

	return map[string]any{"card": card, "number": number, "expiry": expiry, "cvv": cvv, "warning": "CVV is shown only once"}, nil
}

func (s *Service) CardsByAccount(ctx context.Context, userID, accountID int64) ([]models.Card, error) {
	return s.repo.ListCardsByAccount(ctx, userID, accountID)
}

func (s *Service) Card(ctx context.Context, userID, cardID int64) (models.Card, error) {
	c, _, err := s.repo.GetCardDecrypted(ctx, userID, cardID, s.pgpPassphrase)
	return c, err
}

func (s *Service) PayByCard(ctx context.Context, userID int64, req models.CardPaymentRequest) error {
	c, hash, err := s.repo.GetCardDecrypted(ctx, userID, req.CardID, s.pgpPassphrase)
	if err != nil {
		return err
	}

	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.CVV)) != nil {
		return repository.ErrForbidden
	}

	_, err = s.repo.Withdraw(ctx, userID, c.AccountID, utils.ToKopecks(req.Amount), "card_payment", "Card payment: "+req.Merchant+" "+req.Description)

	return err
}

func (s *Service) CreateCredit(ctx context.Context, userID int64, req models.CreateCreditRequest, email string) (models.Credit, error) {
	if err := req.Validate(); err != nil {
		return models.Credit{}, err
	}

	rate, err := s.cbr.KeyRate()
	if err != nil {
		rate = 16.0
	}

	rate += s.margin

	principal := utils.ToKopecks(req.Amount)

	annuity := calcAnnuity(principal, rate, req.Months)

	schedule := buildSchedule(principal, annuity, rate, req.Months)

	credit, err := s.repo.CreateCreditWithSchedule(ctx, userID, req.AccountID, principal, annuity, rate, req.Months, schedule)
	if err == nil && email != "" {
		_ = s.mailer.Send(email, "Кредит оформлен", fmt.Sprintf("<h1>Кредит оформлен</h1><p>Сумма %.2f RUB, ставка %.2f%%, платеж %.2f RUB</p>", req.Amount, rate, utils.ToRub(annuity)))
	}

	return credit, err
}

func calcAnnuity(principal int64, annualRate float64, months int) int64 {
	m := annualRate / 100 / 12

	p := float64(principal)

	if m == 0 {
		return int64(math.Ceil(p / float64(months)))
	}

	return int64(math.Ceil(p * m * math.Pow(1+m, float64(months)) / (math.Pow(1+m, float64(months)) - 1)))
}

func buildSchedule(principal, annuity int64, annualRate float64, months int) []models.PaymentSchedule {
	balance := principal
	m := annualRate / 100 / 12
	out := make([]models.PaymentSchedule, 0, months)

	for i := 1; i <= months; i++ {
		interest := int64(math.Round(float64(balance) * m))
		principalPart := annuity - interest

		if i == months || principalPart > balance {
			principalPart = balance
			annuity = principalPart + interest
		}

		balance -= principalPart
		out = append(out, models.PaymentSchedule{DueDate: time.Now().AddDate(0, i, 0), PaymentKopecks: annuity, PrincipalPartKopecks: principalPart, InterestPartKopecks: interest})
	}

	return out
}

func (s *Service) CreditSchedule(ctx context.Context, userID, creditID int64) ([]models.PaymentSchedule, error) {
	return s.repo.CreditSchedule(ctx, userID, creditID)
}

func (s *Service) Analytics(ctx context.Context, userID int64) (models.Analytics, error) {
	return s.repo.Analytics(ctx, userID)
}

func (s *Service) Predict(ctx context.Context, userID, accountID int64, days int) (models.BalancePrediction, error) {
	if days < 1 || days > 365 {
		return models.BalancePrediction{}, fmt.Errorf("days must be 1..365")
	}

	return s.repo.Predict(ctx, userID, accountID, days)
}

func (s *Service) ProcessDuePayments(ctx context.Context) (int, error) {
	return s.repo.ProcessDuePayments(ctx, time.Now())
}

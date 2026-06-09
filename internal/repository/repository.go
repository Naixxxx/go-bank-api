package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"go-bank-api-max/internal/models"
	"go-bank-api-max/internal/utils"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrAlreadyExists     = errors.New("already exists")
	ErrForbidden         = errors.New("forbidden")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) DB() *sql.DB { return r.db }

func convertPQ(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return ErrAlreadyExists
	}

	return err
}

func scanAccount(row interface{ Scan(dest ...any) error }) (models.Account, error) {
	var a models.Account

	err := row.Scan(&a.ID, &a.UserID, &a.Number, &a.Currency, &a.BalanceKopecks, &a.IsBlocked, &a.CreatedAt)

	a.Balance = utils.ToRub(a.BalanceKopecks)

	return a, convertPQ(err)
}

func (r *Repository) CreateUser(ctx context.Context, email, username, passHash string) (models.User, error) {
	var u models.User

	err := r.db.QueryRowContext(ctx, `INSERT INTO users(email, username, password_hash) VALUES($1,$2,$3) RETURNING id,email,username,password_hash,created_at`, email, username, passHash).
		Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.CreatedAt)

	return u, convertPQ(err)
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	var u models.User

	err := r.db.QueryRowContext(ctx, `SELECT id,email,username,password_hash,created_at FROM users WHERE email=$1`, email).
		Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.CreatedAt)

	return u, convertPQ(err)
}

func (r *Repository) CreateAccount(ctx context.Context, userID int64, number string) (models.Account, error) {
	return scanAccount(r.db.QueryRowContext(ctx, `INSERT INTO accounts(user_id, number) VALUES($1,$2) RETURNING id,user_id,number,currency,balance_kopecks,is_blocked,created_at`, userID, number))
}

func (r *Repository) GetAccount(ctx context.Context, id int64) (models.Account, error) {
	return scanAccount(r.db.QueryRowContext(ctx, `SELECT id,user_id,number,currency,balance_kopecks,is_blocked,created_at FROM accounts WHERE id=$1`, id))
}

func (r *Repository) ListAccounts(ctx context.Context, userID int64) ([]models.Account, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,user_id,number,currency,balance_kopecks,is_blocked,created_at FROM accounts WHERE user_id=$1 ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var res []models.Account

	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, a)
	}

	return res, rows.Err()
}

func (r *Repository) Deposit(ctx context.Context, userID, accountID, amount int64, typ, desc string) (models.Account, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Account{}, err
	}

	defer tx.Rollback()

	var owner int64
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM accounts WHERE id=$1 FOR UPDATE`, accountID).Scan(&owner); err != nil {
		return models.Account{}, convertPQ(err)
	}

	if owner != userID {
		return models.Account{}, ErrForbidden
	}

	row := tx.QueryRowContext(ctx, `UPDATE accounts SET balance_kopecks=balance_kopecks+$1 WHERE id=$2 RETURNING id,user_id,number,currency,balance_kopecks,is_blocked,created_at`, amount, accountID)

	a, err := scanAccount(row)
	if err != nil {
		return models.Account{}, err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO transactions(account_id,amount_kopecks,type,description) VALUES($1,$2,$3,$4)`, accountID, amount, typ, desc)

	if err != nil {
		return models.Account{}, err
	}

	return a, tx.Commit()
}

func (r *Repository) Withdraw(ctx context.Context, userID, accountID, amount int64, typ, desc string) (models.Account, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Account{}, err
	}

	defer tx.Rollback()

	var owner, balance int64

	if err := tx.QueryRowContext(ctx, `SELECT user_id,balance_kopecks FROM accounts WHERE id=$1 FOR UPDATE`, accountID).Scan(&owner, &balance); err != nil {
		return models.Account{}, convertPQ(err)
	}

	if owner != userID {
		return models.Account{}, ErrForbidden
	}

	if balance < amount {
		return models.Account{}, ErrInsufficientFunds
	}

	row := tx.QueryRowContext(ctx, `UPDATE accounts SET balance_kopecks=balance_kopecks-$1 WHERE id=$2 RETURNING id,user_id,number,currency,balance_kopecks,is_blocked,created_at`, amount, accountID)

	a, err := scanAccount(row)
	if err != nil {
		return models.Account{}, err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO transactions(account_id,amount_kopecks,type,description) VALUES($1,$2,$3,$4)`, accountID, -amount, typ, desc)
	if err != nil {
		return models.Account{}, err
	}

	return a, tx.Commit()
}

func (r *Repository) Transfer(ctx context.Context, userID, fromID, toID, amount int64, desc string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var owner, balance int64

	if err := tx.QueryRowContext(ctx, `SELECT user_id,balance_kopecks FROM accounts WHERE id=$1 FOR UPDATE`, fromID).Scan(&owner, &balance); err != nil {
		return convertPQ(err)
	}

	if owner != userID {
		return ErrForbidden
	}

	if balance < amount {
		return ErrInsufficientFunds
	}

	var toExists bool

	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounts WHERE id=$1)`, toID).Scan(&toExists); err != nil {
		return err
	}

	if !toExists {
		return ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET balance_kopecks=balance_kopecks-$1 WHERE id=$2`, amount, fromID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET balance_kopecks=balance_kopecks+$1 WHERE id=$2`, amount, toID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO transactions(account_id,related_account_id,amount_kopecks,type,description) VALUES($1,$2,$3,'transfer_out',$5),($2,$1,$4,'transfer_in',$5)`, fromID, toID, -amount, amount, desc); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *Repository) CreateCard(ctx context.Context, userID, accountID int64, number, expiry, cvvHash, numberHMAC, expiryHMAC, last4, passphrase string) (models.Card, error) {
	var owner int64

	if err := r.db.QueryRowContext(ctx, `SELECT user_id FROM accounts WHERE id=$1`, accountID).Scan(&owner); err != nil {
		return models.Card{}, convertPQ(err)
	}

	if owner != userID {
		return models.Card{}, ErrForbidden
	}

	var c models.Card

	err := r.db.QueryRowContext(ctx, `INSERT INTO cards(user_id,account_id,number_encrypted,expiry_encrypted,cvv_hash,number_hmac,expiry_hmac,last4) VALUES($1,$2,pgp_sym_encrypt($3,$8),pgp_sym_encrypt($4,$8),$5,$6,$7,$9) RETURNING id,user_id,account_id,last4,created_at`, userID, accountID, number, expiry, cvvHash, numberHMAC, expiryHMAC, passphrase, last4).
		Scan(&c.ID, &c.UserID, &c.AccountID, &c.Last4, &c.CreatedAt)

	c.Masked = utils.MaskCard(number)

	return c, convertPQ(err)
}

func (r *Repository) ListCardsByAccount(ctx context.Context, userID, accountID int64) ([]models.Card, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,user_id,account_id,last4,created_at FROM cards WHERE user_id=$1 AND account_id=$2 ORDER BY id`, userID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []models.Card

	for rows.Next() {
		var c models.Card
		if err := rows.Scan(&c.ID, &c.UserID, &c.AccountID, &c.Last4, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Masked = "**** **** **** " + c.Last4

		res = append(res, c)
	}

	return res, rows.Err()
}

func (r *Repository) GetCardDecrypted(ctx context.Context, userID, cardID int64, passphrase string) (models.Card, string, error) {
	var (
		c       models.Card
		cvvHash string
	)

	err := r.db.QueryRowContext(ctx, `SELECT id,user_id,account_id,pgp_sym_decrypt(number_encrypted,$2),pgp_sym_decrypt(expiry_encrypted,$2),last4,created_at,cvv_hash FROM cards WHERE id=$1`, cardID, passphrase).
		Scan(&c.ID, &c.UserID, &c.AccountID, &c.Number, &c.Expiry, &c.Last4, &c.CreatedAt, &cvvHash)
	if err != nil {
		return c, "", convertPQ(err)
	}

	if c.UserID != userID {
		return c, "", ErrForbidden
	}

	c.Masked = utils.MaskCard(c.Number)

	return c, cvvHash, nil
}

func (r *Repository) ListTransactions(ctx context.Context, userID, accountID int64) ([]models.Transaction, error) {
	var owner int64

	if err := r.db.QueryRowContext(ctx, `SELECT user_id FROM accounts WHERE id=$1`, accountID).Scan(&owner); err != nil {
		return nil, convertPQ(err)
	}

	if owner != userID {
		return nil, ErrForbidden
	}

	rows, err := r.db.QueryContext(ctx, `SELECT id,account_id,related_account_id,amount_kopecks,type,description,created_at FROM transactions WHERE account_id=$1 ORDER BY created_at DESC LIMIT 200`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []models.Transaction

	for rows.Next() {
		var (
			t       models.Transaction
			related sql.NullInt64
		)

		if err := rows.Scan(&t.ID, &t.AccountID, &related, &t.AmountKopecks, &t.Type, &t.Description, &t.CreatedAt); err != nil {
			return nil, err
		}

		if related.Valid {
			t.RelatedAccountID = &related.Int64
		}

		t.Amount = utils.ToRub(t.AmountKopecks)

		res = append(res, t)
	}

	return res, rows.Err()
}

func (r *Repository) CreateCreditWithSchedule(ctx context.Context, userID, accountID, principal, annuity int64, rate float64, months int, schedules []models.PaymentSchedule) (models.Credit, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Credit{}, err
	}
	defer tx.Rollback()

	var owner int64

	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM accounts WHERE id=$1 FOR UPDATE`, accountID).Scan(&owner); err != nil {
		return models.Credit{}, convertPQ(err)
	}

	if owner != userID {
		return models.Credit{}, ErrForbidden
	}

	var c models.Credit

	err = tx.QueryRowContext(ctx, `INSERT INTO credits(user_id,account_id,principal_kopecks,annual_rate,months,annuity_payment_kopecks) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,user_id,account_id,principal_kopecks,annual_rate,months,annuity_payment_kopecks,status,created_at`, userID, accountID, principal, rate, months, annuity).
		Scan(&c.ID, &c.UserID, &c.AccountID, &c.PrincipalKopecks, &c.AnnualRate, &c.Months, &c.AnnuityPaymentKopecks, &c.Status, &c.CreatedAt)
	if err != nil {
		return models.Credit{}, err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET balance_kopecks=balance_kopecks+$1 WHERE id=$2`, principal, accountID); err != nil {
		return models.Credit{}, err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO transactions(account_id,amount_kopecks,type,description) VALUES($1,$2,'credit_issue',$3)`, accountID, principal, fmt.Sprintf("Credit #%d issued", c.ID)); err != nil {
		return models.Credit{}, err
	}

	for _, s := range schedules {
		if _, err := tx.ExecContext(ctx, `INSERT INTO payment_schedules(credit_id,account_id,due_date,payment_kopecks,principal_part_kopecks,interest_part_kopecks) VALUES($1,$2,$3,$4,$5,$6)`, c.ID, accountID, s.DueDate, s.PaymentKopecks, s.PrincipalPartKopecks, s.InterestPartKopecks); err != nil {
			return models.Credit{}, err
		}
	}

	c.Principal = utils.ToRub(c.PrincipalKopecks)
	c.AnnuityPayment = utils.ToRub(c.AnnuityPaymentKopecks)

	return c, tx.Commit()
}

func (r *Repository) CreditSchedule(ctx context.Context, userID, creditID int64) ([]models.PaymentSchedule, error) {
	var owner int64

	if err := r.db.QueryRowContext(ctx, `SELECT user_id FROM credits WHERE id=$1`, creditID).Scan(&owner); err != nil {
		return nil, convertPQ(err)
	}

	if owner != userID {
		return nil, ErrForbidden
	}

	return r.scheduleRows(ctx, `SELECT id,credit_id,account_id,due_date,payment_kopecks,principal_part_kopecks,interest_part_kopecks,penalty_kopecks,status,paid_at FROM payment_schedules WHERE credit_id=$1 ORDER BY due_date`, creditID)
}

func (r *Repository) scheduleRows(ctx context.Context, q string, args ...any) ([]models.PaymentSchedule, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []models.PaymentSchedule

	for rows.Next() {
		var (
			s    models.PaymentSchedule
			paid sql.NullTime
		)

		if err := rows.Scan(&s.ID, &s.CreditID, &s.AccountID, &s.DueDate, &s.PaymentKopecks, &s.PrincipalPartKopecks, &s.InterestPartKopecks, &s.PenaltyKopecks, &s.Status, &paid); err != nil {
			return nil, err
		}

		if paid.Valid {
			s.PaidAt = &paid.Time
		}

		s.Payment = utils.ToRub(s.PaymentKopecks + s.PenaltyKopecks)

		res = append(res, s)
	}

	return res, rows.Err()
}

func (r *Repository) Analytics(ctx context.Context, userID int64) (models.Analytics, error) {
	var (
		a                          models.Analytics
		income, expense, debt, pay sql.NullInt64
	)

	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN t.amount_kopecks>0 THEN t.amount_kopecks ELSE 0 END),0), COALESCE(SUM(CASE WHEN t.amount_kopecks<0 THEN -t.amount_kopecks ELSE 0 END),0) FROM transactions t JOIN accounts a ON a.id=t.account_id WHERE a.user_id=$1 AND t.created_at >= date_trunc('month', now())`, userID).Scan(&income, &expense)
	if err != nil {
		return a, err
	}

	err = r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(principal_kopecks),0), COALESCE(SUM(annuity_payment_kopecks),0) FROM credits WHERE user_id=$1 AND status='active'`, userID).Scan(&debt, &pay)
	if err != nil {
		return a, err
	}

	a.MonthIncome = utils.ToRub(income.Int64)
	a.MonthExpense = utils.ToRub(expense.Int64)
	a.ActiveCreditDebt = utils.ToRub(debt.Int64)
	a.MonthlyCreditPayment = utils.ToRub(pay.Int64)

	if a.MonthIncome > 0 {
		a.CreditLoadPercent = a.MonthlyCreditPayment / a.MonthIncome * 100
	}

	return a, nil
}

func (r *Repository) Predict(ctx context.Context, userID, accountID int64, days int) (models.BalancePrediction, error) {
	a, err := r.GetAccount(ctx, accountID)
	if err != nil {
		return models.BalancePrediction{}, err
	}

	if a.UserID != userID {
		return models.BalancePrediction{}, ErrForbidden
	}

	var out sql.NullInt64

	err = r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(payment_kopecks + penalty_kopecks),0) FROM payment_schedules WHERE account_id=$1 AND status IN ('planned','overdue') AND due_date <= CURRENT_DATE + ($2 || ' days')::interval`, accountID, days).Scan(&out)
	if err != nil {
		return models.BalancePrediction{}, err
	}

	return models.BalancePrediction{AccountID: accountID, Days: days, CurrentBalance: a.Balance, ScheduledOutflow: utils.ToRub(out.Int64), PredictedBalance: utils.ToRub(a.BalanceKopecks - out.Int64)}, nil
}

func (r *Repository) ProcessDuePayments(ctx context.Context, now time.Time) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `SELECT id,credit_id,account_id,payment_kopecks,penalty_kopecks FROM payment_schedules WHERE status IN ('planned','overdue') AND due_date <= $1 ORDER BY due_date FOR UPDATE`, now.Format("2006-01-02"))
	if err != nil {
		return 0, err
	}

	type due struct{ id, creditID, accountID, payment, penalty int64 }

	var dues []due

	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.creditID, &d.accountID, &d.payment, &d.penalty); err != nil {
			rows.Close()
			return 0, err
		}
		dues = append(dues, d)
	}
	rows.Close()

	processed := 0
	for _, d := range dues {
		var balance int64

		if err := tx.QueryRowContext(ctx, `SELECT balance_kopecks FROM accounts WHERE id=$1 FOR UPDATE`, d.accountID).Scan(&balance); err != nil {
			return 0, err
		}

		total := d.payment + d.penalty

		if balance >= total {
			if _, err := tx.ExecContext(ctx, `UPDATE accounts SET balance_kopecks=balance_kopecks-$1 WHERE id=$2`, total, d.accountID); err != nil {
				return 0, err
			}

			if _, err := tx.ExecContext(ctx, `UPDATE payment_schedules SET status='paid', paid_at=now() WHERE id=$1`, d.id); err != nil {
				return 0, err
			}

			if _, err := tx.ExecContext(ctx, `INSERT INTO transactions(account_id,amount_kopecks,type,description) VALUES($1,$2,'credit_payment',$3)`, d.accountID, -total, fmt.Sprintf("Autopayment for credit #%d", d.creditID)); err != nil {
				return 0, err
			}

			processed++
		} else {
			penalty := (d.payment + d.penalty) / 10

			if penalty < 1 {
				penalty = 1
			}

			if _, err := tx.ExecContext(ctx, `UPDATE payment_schedules SET status='overdue', penalty_kopecks=penalty_kopecks+$1 WHERE id=$2`, penalty, d.id); err != nil {
				return 0, err
			}

			if _, err := tx.ExecContext(ctx, `UPDATE credits SET status='overdue' WHERE id=$1`, d.creditID); err != nil {
				return 0, err
			}
		}
	}

	return processed, tx.Commit()
}

func (r *Repository) GetUserByID(ctx context.Context, id int64) (models.User, error) {
	var u models.User

	err := r.db.QueryRowContext(ctx, `SELECT id,email,username,password_hash,created_at FROM users WHERE id=$1`, id).
		Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.CreatedAt)

	return u, convertPQ(err)
}

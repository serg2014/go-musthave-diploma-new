package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/postgres"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/serg2014/go-musthave-diploma/internal/app/models"
	"github.com/serg2014/go-musthave-diploma/internal/logger"
	"go.uber.org/zap"
)

var ErrUserExists = errors.New("user exists")
var ErrUserOrPassword = errors.New("bad user or password")
var ErrOrderAnotherUser = errors.New("order another user")
var ErrOrderExists = errors.New("order exists")
var ErrNotEnoughMoney = errors.New("not enough money")
var ErrOrderWithdrawnExists = errors.New("order withdrawn exists")

type User struct {
	ID    models.UserID
	Login string
	Hash  string
}

type storage struct {
	db *sql.DB
}

func NewStorage(ctx context.Context, dsn string) (Storager, error) {
	// dsn = "host=%s user=%s password=%s dbname=%s sslmode=disable"
	// dsn = "postgres://user:password@host:port/dbname?sslmode=disable"
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed connect to db: %w", err)
	}
	// проверяем подключение к бд
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %v", err)
	}
	logger.Log.Info("Connected to db")

	// миграции
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("can not set migrations: %w", err)
	}

	// TODO file://migrations путь задается относительно cwd
	// предполагается что запуск бинаря происходит в корне репозитория
	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		dsn,
		driver,
	)
	if err != nil {
		return nil, fmt.Errorf("can not find migrations: %w", err)
	}
	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Log.Error("migrations", zap.Error(err))
		// TODO в случае проблем с миграцией сообщение об ошибке говорит что не найдет файл
		// и все. путь к файлу не пишут
		return nil, fmt.Errorf("problem with Up migration: %w", err)
	}

	return &storage{db: db}, nil
}

// TODO посмотреть какой тип в postgres serial
func (s *storage) CreateUser(ctx context.Context, login, passwordHash string) (*models.UserID, error) {
	// начать транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed transaction in CreateUser: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO users (login, hash) VALUES($1, $2) RETURNING user_id`
	row := tx.QueryRowContext(ctx, query, login, passwordHash)
	var user User
	err = row.Scan(&user.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return nil, ErrUserExists
			}
		}
		return nil, fmt.Errorf("failed CreateUser. can not insert users: %w", err)
	}

	query = `INSERT INTO users_balance (user_id) VALUES($1)`
	_, err = s.db.ExecContext(ctx, query, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed CreateUser. can not insert users_balance: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed commit transaction: %w", err)
	}
	return &user.ID, nil
}

func (s *storage) GetUser(ctx context.Context, login, passwordHash string) (*models.UserID, error) {
	query := `SELECT user_id FROM users WHERE login=$1 AND hash=$2`
	row := s.db.QueryRowContext(ctx, query, login, passwordHash)
	var user User
	err := row.Scan(&user.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserOrPassword
		}
		return nil, fmt.Errorf("failed GetUser. can not select: %w", err)
	}
	return &user.ID, nil
}

func (s *storage) CreateOrder(ctx context.Context, orderID string, userID models.UserID) error {
	// начать транзакцию
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed SetBatch: %w", err)
	}
	defer tx.Rollback()

	// insert два раза одного и того же order_id, user_id - 23505
	query := `
	INSERT INTO orders (order_id, user_id, upload_time, status)
	VALUES($1, $2, current_timestamp, $3)
	ON CONFLICT (order_id) DO NOTHING`
	result, err := tx.ExecContext(ctx, query, orderID, userID, models.OrderNew)
	if err != nil {
		return fmt.Errorf("failed CreateOrder: %w", err)
	}
	ra, _ := result.RowsAffected()
	logger.Log.Info("rowsaffected", zap.Int64("ra", ra))
	if ra == 0 {
		// сразу отменяем транзакцию
		tx.Rollback()

		query = `
		SELECT order_id
		FROM orders
		WHERE order_id = $1 AND user_id = $2`
		result, err = s.db.ExecContext(ctx, query, orderID, userID)
		if err != nil {
			return fmt.Errorf("failed CreateOrder: %w", err)
		}
		ra, _ = result.RowsAffected()
		if ra == 0 {
			return ErrOrderAnotherUser
		}
		return ErrOrderExists
	}

	query = `
	INSERT INTO orders_for_process (order_id, user_id, update_time)
	VALUES($1, $2, current_timestamp)`
	_, err = tx.ExecContext(ctx, query, orderID, userID)
	if err != nil {
		return fmt.Errorf("failed CreateOrder: %w", err)
	}

	return tx.Commit()
}

func (s *storage) GetUserOrders(ctx context.Context, userID models.UserID) (models.Orders, error) {
	query := `
		SELECT order_id, upload_time, status, accrual
		FROM orders
		WHERE user_id = $1
		ORDER BY upload_time DESC`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed select in GetUserOrders: %w", err)
	}
	defer rows.Close()

	orders := make(models.Orders, 0)
	for rows.Next() {
		var order models.OrderItem
		err := rows.Scan(&order.OrderID, &order.UploadTime, &order.Status, &order.Accrual)
		if err != nil {
			return nil, fmt.Errorf("failed Scan in GetUserOrders: %w", err)
		}
		orders = append(orders, order)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("faild row: %w", err)
	}
	return orders, nil
}

func (s *storage) Balance(ctx context.Context, userID models.UserID) (*models.Balance, error) {
	query := `
		SELECT accrual-withdrawn, withdrawn
		FROM users_balance
		WHERE user_id = $1
	`
	row := s.db.QueryRowContext(ctx, query, userID)
	var balance models.Balance
	err := row.Scan(&balance.Current, &balance.Withdrawn)
	if err != nil {
		return nil, fmt.Errorf("failed Balance: %w", err)
	}
	return &balance, nil
}

func (s *storage) Withdraw(ctx context.Context, userID models.UserID, orderID string, sum uint32) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO credit (order_id, user_id, sum, create_time)
		VALUES($1, $2, $3, current_timestamp)
	`
	_, err = tx.ExecContext(ctx, query, orderID, userID, sum)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return ErrOrderWithdrawnExists
			}
		}
		return fmt.Errorf("failed insert credit: %w", err)
	}

	query = `
		UPDATE users_balance
		SET withdrawn=withdrawn+$1, accrual=accrual-$1
		WHERE user_id = $2 AND accrual>=$1
	`
	result, err := tx.ExecContext(ctx, query, sum, userID)
	if err != nil {
		return fmt.Errorf("failed update user_balance: %w", err)
	}
	ra, _ := result.RowsAffected()
	if ra == 0 {
		return ErrNotEnoughMoney
	}
	return tx.Commit()
}

func (s *storage) Withdrawals(ctx context.Context, userID models.UserID) (models.Withdrawals, error) {
	query := `
		SELECT order_id, sum, create_time
		FROM credit
		WHERE user_id = $1
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed Withdrawals: %w", err)
	}
	defer rows.Close()

	withdrawals := make(models.Withdrawals, 0)
	for rows.Next() {
		var withdrawal models.Withdrawal
		err := rows.Scan(&withdrawal.OrderID, &withdrawal.Sum, &withdrawal.CreateTime)
		if err != nil {
			return nil, fmt.Errorf("failed Scan in Withdrawals: %w", err)
		}
		withdrawals = append(withdrawals, withdrawal)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("failed Withdrawals: %w", err)
	}
	return withdrawals, nil
}

type Storager interface {
	CreateUser(ctx context.Context, login, passwordHash string) (*models.UserID, error)
	GetUser(ctx context.Context, login, passwordHash string) (*models.UserID, error)
	CreateOrder(ctx context.Context, orderID string, userID models.UserID) error
	GetUserOrders(ctx context.Context, userID models.UserID) (models.Orders, error)
	Balance(ctx context.Context, userID models.UserID) (*models.Balance, error)
	Withdraw(ctx context.Context, userID models.UserID, orderID string, sum uint32) error
	Withdrawals(ctx context.Context, userID models.UserID) (models.Withdrawals, error)
}

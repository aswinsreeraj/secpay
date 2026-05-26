package postgres

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	dialector := postgres.New(postgres.Config{
		Conn:       sqlDB,
		DriverName: "postgres",
	})

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open GORM db: %v", err)
	}

	return db, mock
}

func TestAccountRepository_TransferBalance(t *testing.T) {
	// Lexicographically, senderID < receiverID
	senderID := "11111111-1111-1111-1111-111111111111"
	receiverID := "22222222-2222-2222-2222-222222222222"

	t.Run("successful transfer - commits fully", func(t *testing.T) {
		db, mock := setupTestDB(t)
		repo := NewAccountRepository(db)

		// 1. Transaction starts
		mock.ExpectBegin()

		// 2. Fetch and Lock smaller ID (senderID) first
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "accounts" WHERE id = $1 ORDER BY "accounts"."id" LIMIT $2 FOR UPDATE`)).
			WithArgs(senderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "account_number", "account_type", "balance", "currency", "created_at", "updated_at"}).
				AddRow(senderID, "user-1", "ACC0000001", "savings", int64(10000), "USD", time.Now(), time.Now()))

		// 3. Fetch and Lock larger ID (receiverID) second
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "accounts" WHERE id = $1 ORDER BY "accounts"."id" LIMIT $2 FOR UPDATE`)).
			WithArgs(receiverID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "account_number", "account_type", "balance", "currency", "created_at", "updated_at"}).
				AddRow(receiverID, "user-2", "ACC0000002", "checking", int64(5000), "USD", time.Now(), time.Now()))

		// 4. Expect balance updates (GORM's Save updates both balance and updated_at)
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "accounts" SET "user_id"=$1,"account_number"=$2,"account_type"=$3,"balance"=$4,"currency"=$5,"created_at"=$6,"updated_at"=$7 WHERE "id" = $8`)).
			WithArgs("user-1", "ACC0000001", "savings", int64(8000), "USD", sqlmock.AnyArg(), sqlmock.AnyArg(), senderID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "accounts" SET "user_id"=$1,"account_number"=$2,"account_type"=$3,"balance"=$4,"currency"=$5,"created_at"=$6,"updated_at"=$7 WHERE "id" = $8`)).
			WithArgs("user-2", "ACC0000002", "checking", int64(7000), "USD", sqlmock.AnyArg(), sqlmock.AnyArg(), receiverID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// 5. Expect transaction audit log record insert
		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "transactions" ("id","from_account_id","to_account_id","amount","transaction_type","status","description","created_at") VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`)).
			WithArgs(sqlmock.AnyArg(), senderID, receiverID, int64(2000), "transfer", "success", "Rent payment", sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// 6. Transaction commits
		mock.ExpectCommit()

		err := repo.TransferBalance(context.Background(), senderID, receiverID, 2000, "Rent payment")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled mock expectations: %v", err)
		}
	})

	t.Run("insufficient balance - rolls back and triggers error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		repo := NewAccountRepository(db)

		// 1. Transaction starts
		mock.ExpectBegin()

		// 2. Fetch and Lock smaller ID (senderID) first
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "accounts" WHERE id = $1 ORDER BY "accounts"."id" LIMIT $2 FOR UPDATE`)).
			WithArgs(senderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "account_number", "account_type", "balance", "currency", "created_at", "updated_at"}).
				AddRow(senderID, "user-1", "ACC0000001", "savings", int64(1000), "USD", time.Now(), time.Now())) // balance: 1000

		// 3. Fetch and Lock larger ID (receiverID) second
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "accounts" WHERE id = $1 ORDER BY "accounts"."id" LIMIT $2 FOR UPDATE`)).
			WithArgs(receiverID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "account_number", "account_type", "balance", "currency", "created_at", "updated_at"}).
				AddRow(receiverID, "user-2", "ACC0000002", "checking", int64(5000), "USD", time.Now(), time.Now()))

		// 4. Transaction rolls back due to error (1000 balance < 2000 transfer)
		mock.ExpectRollback()

		err := repo.TransferBalance(context.Background(), senderID, receiverID, 2000, "Rent payment")
		if err == nil {
			t.Error("expected error for insufficient balance, got nil")
		} else if err.Error() != "insufficient balance" {
			t.Errorf("expected 'insufficient balance' error, got: %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled mock expectations: %v", err)
		}
	})

	t.Run("failure on writing audit log - rolls back entire balance change", func(t *testing.T) {
		db, mock := setupTestDB(t)
		repo := NewAccountRepository(db)

		// 1. Transaction starts
		mock.ExpectBegin()

		// 2. Fetch and Lock smaller ID (senderID) first
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "accounts" WHERE id = $1 ORDER BY "accounts"."id" LIMIT $2 FOR UPDATE`)).
			WithArgs(senderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "account_number", "account_type", "balance", "currency", "created_at", "updated_at"}).
				AddRow(senderID, "user-1", "ACC0000001", "savings", int64(10000), "USD", time.Now(), time.Now()))

		// 3. Fetch and Lock larger ID (receiverID) second
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "accounts" WHERE id = $1 ORDER BY "accounts"."id" LIMIT $2 FOR UPDATE`)).
			WithArgs(receiverID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "account_number", "account_type", "balance", "currency", "created_at", "updated_at"}).
				AddRow(receiverID, "user-2", "ACC0000002", "checking", int64(5000), "USD", time.Now(), time.Now()))

		// 4. Balances updates run successfully
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "accounts" SET "user_id"=$1,"account_number"=$2,"account_type"=$3,"balance"=$4,"currency"=$5,"created_at"=$6,"updated_at"=$7 WHERE "id" = $8`)).
			WithArgs("user-1", "ACC0000001", "savings", int64(8000), "USD", sqlmock.AnyArg(), sqlmock.AnyArg(), senderID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "accounts" SET "user_id"=$1,"account_number"=$2,"account_type"=$3,"balance"=$4,"currency"=$5,"created_at"=$6,"updated_at"=$7 WHERE "id" = $8`)).
			WithArgs("user-2", "ACC0000002", "checking", int64(7000), "USD", sqlmock.AnyArg(), sqlmock.AnyArg(), receiverID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// 5. Audit log insert fails (simulated DB error)
		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "transactions" ("id","from_account_id","to_account_id","amount","transaction_type","status","description","created_at") VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`)).
			WithArgs(sqlmock.AnyArg(), senderID, receiverID, int64(2000), "transfer", "success", "Rent payment", sqlmock.AnyArg()).
			WillReturnError(errors.New("db disk full"))

		// 6. Transaction rolls back completely (so that no partial balance changes are committed)
		mock.ExpectRollback()

		err := repo.TransferBalance(context.Background(), senderID, receiverID, 2000, "Rent payment")
		if err == nil {
			t.Error("expected error for failed insert, got nil")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled mock expectations: %v", err)
		}
	})
}

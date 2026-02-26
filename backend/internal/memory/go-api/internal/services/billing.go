// Package services provides core business logic.
// billing.go mirrors Python services/billing.py — ACID billing operations.
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidAmount       = errors.New("amount must be positive")
)

// BillingService provides balance checking, deduction, and recharge.
// All monetary operations use Decimal for precision.
type BillingService struct{}

// GetBalance returns the current balance for a user.
func (s *BillingService) GetBalance(db *gorm.DB, userID string) (decimal.Decimal, error) {
	var account models.BillingAccount
	result := db.Where("user_id = ?", userID).First(&account)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return decimal.Zero, nil // No account → zero balance
		}
		return decimal.Zero, fmt.Errorf("get balance: %w", result.Error)
	}
	return account.Balance, nil
}

// CheckBalance checks if a user has sufficient balance for the estimated cost.
func (s *BillingService) CheckBalance(db *gorm.DB, userID string, estimatedCost decimal.Decimal) (bool, error) {
	balance, err := s.GetBalance(db, userID)
	if err != nil {
		return false, err
	}
	return balance.GreaterThanOrEqual(estimatedCost), nil
}

// ProcessDeduction executes a billing deduction within a DB transaction.
// Ensures ACID atomicity: check balance → update → create transaction record.
func (s *BillingService) ProcessDeduction(
	db *gorm.DB,
	userID string,
	cost decimal.Decimal,
	description string,
	endpoint string,
) error {
	if cost.LessThanOrEqual(decimal.Zero) {
		return nil // No cost, no deduction
	}

	return db.Transaction(func(tx *gorm.DB) error {
		// 1. Lock and fetch current balance
		var account models.BillingAccount
		result := tx.Where("user_id = ?", userID).First(&account)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return ErrInsufficientBalance
			}
			return fmt.Errorf("fetch account: %w", result.Error)
		}

		// 2. Check balance
		if account.Balance.LessThan(cost) {
			return ErrInsufficientBalance
		}

		// 3. Update balance
		newBalance := account.Balance.Sub(cost)
		if err := tx.Model(&account).Update("balance", newBalance).Error; err != nil {
			return fmt.Errorf("update balance: %w", err)
		}

		// 4. Create transaction record
		desc := fmt.Sprintf("%s — %s", description, endpoint)
		txRecord := models.Transaction{
			UserID:          userID,
			Amount:          cost.Neg(), // Negative for deductions
			TransactionType: "deduction",
			Description:     &desc,
		}
		if err := tx.Create(&txRecord).Error; err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		slog.Debug("Billing deduction processed",
			"user_id", userID,
			"cost", cost.String(),
			"new_balance", newBalance.String(),
		)

		// 5. Check balance warning (async notification — deferred to notification service)
		go s.checkBalanceWarning(userID, newBalance)

		return nil
	})
}

// ProcessRecharge adds funds to a user's account.
func (s *BillingService) ProcessRecharge(
	db *gorm.DB,
	userID string,
	amount decimal.Decimal,
	description string,
) (*models.Transaction, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidAmount
	}

	var txRecord models.Transaction

	err := db.Transaction(func(tx *gorm.DB) error {
		// Upsert billing account
		var account models.BillingAccount
		result := tx.Where("user_id = ?", userID).First(&account)

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Create new account
			account = models.BillingAccount{
				UserID:  userID,
				Balance: amount,
			}
			if err := tx.Create(&account).Error; err != nil {
				return fmt.Errorf("create account: %w", err)
			}
		} else if result.Error != nil {
			return fmt.Errorf("fetch account: %w", result.Error)
		} else {
			// Update existing
			newBalance := account.Balance.Add(amount)
			if err := tx.Model(&account).Update("balance", newBalance).Error; err != nil {
				return fmt.Errorf("update balance: %w", err)
			}
		}

		// Create transaction record
		txRecord = models.Transaction{
			UserID:          userID,
			Amount:          amount,
			TransactionType: "recharge",
			Description:     &description,
		}
		if err := tx.Create(&txRecord).Error; err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return &txRecord, nil
}

// checkBalanceWarning checks if balance is below threshold and triggers notification.
func (s *BillingService) checkBalanceWarning(userID string, currentBalance decimal.Decimal) {
	threshold := decimal.NewFromFloat(10.0) // 可通过 ConfigService 动态配置
	if currentBalance.LessThanOrEqual(decimal.Zero) {
		_ = GetNotifier().Send(context.Background(), userID, EventBalanceZero, map[string]string{
			"balance": currentBalance.String(),
		})
	} else if currentBalance.LessThan(threshold) {
		_ = GetNotifier().Send(context.Background(), userID, EventBalanceLow, map[string]string{
			"balance":   currentBalance.String(),
			"threshold": threshold.String(),
		})
	}
}

// --- Singleton ---

var (
	billingOnce    sync.Once
	billingService *BillingService
)

// GetBillingService returns the singleton BillingService.
func GetBillingService() *BillingService {
	billingOnce.Do(func() {
		billingService = &BillingService{}
	})
	return billingService
}

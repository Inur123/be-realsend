package main

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type MidtransNotification struct {
	TransactionTime   string `json:"transaction_time"`
	TransactionStatus string `json:"transaction_status"`
	TransactionID     string `json:"transaction_id"`
	StatusMessage     string `json:"status_message"`
	StatusCode        string `json:"status_code"`
	SignatureKey      string `json:"signature_key"`
	PaymentType       string `json:"payment_type"`
	OrderID           string `json:"order_id"`
	GrossAmount       string `json:"gross_amount"`
	FraudStatus       string `json:"fraud_status"`
	Currency          string `json:"currency"`
}

func main() {
	// Load .env
	_ = godotenv.Load()

	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if serverKey == "" {
		log.Fatal("MIDTRANS_SERVER_KEY is not set in .env")
	}

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbSSLMode := os.Getenv("DB_SSLMODE")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", dbUser, dbPassword, dbHost, dbPort, dbName, dbSSLMode)
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer db.Close()

	// Find the latest pending payment
	var orderID string
	var amount int
	query := `
		SELECT external_id, amount_idr 
		FROM payments 
		WHERE status = 'pending' 
		ORDER BY created_at DESC 
		LIMIT 1
	`
	err = db.QueryRow(context.Background(), query).Scan(&orderID, &amount)
	if err != nil {
		log.Fatalf("No pending payments found in database. Please create a checkout first. Error: %v", err)
	}

	grossAmountStr := fmt.Sprintf("%d.00", amount)
	statusCode := "200"

	// Generate signature key: SHA512(order_id + status_code + gross_amount + ServerKey)
	payloadStr := orderID + statusCode + grossAmountStr + serverKey
	hasher := sha512.New()
	hasher.Write([]byte(payloadStr))
	signatureKey := hex.EncodeToString(hasher.Sum(nil))

	notif := MidtransNotification{
		TransactionTime:   "2026-05-28 15:30:00",
		TransactionStatus: "settlement",
		TransactionID:     "mock-trans-id-12345",
		StatusMessage:     "Success, payment settled",
		StatusCode:        statusCode,
		SignatureKey:      signatureKey,
		PaymentType:       "gopay",
		OrderID:           orderID,
		GrossAmount:       grossAmountStr,
		FraudStatus:       "accept",
		Currency:          "IDR",
	}

	jsonBytes, err := json.MarshalIndent(notif, "", "  ")
	if err != nil {
		log.Fatalf("Marshal error: %v", err)
	}

	fmt.Printf("Simulating success webhook for Order ID: %s (%s IDR)\n", orderID, grossAmountStr)
	fmt.Println("Payload:")
	fmt.Println(string(jsonBytes))

	resp, err := http.Post("http://localhost:3001/api/v1/billing/midtrans/notification", "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Fatalf("Failed to send webhook: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("\nResponse status: %d\n", resp.StatusCode)
	fmt.Printf("Response body: %s\n", string(respBody))
	if resp.StatusCode == 200 {
		fmt.Println("\n✅ Webhook simulated successfully! Please refresh your Billing page in the browser.")
	} else {
		fmt.Println("\n❌ Webhook simulation failed.")
	}
}

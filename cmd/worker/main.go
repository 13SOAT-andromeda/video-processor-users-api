package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/config"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database"
	usermodel "github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database/model/user"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database/repository"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
)

type snsEnvelope struct {
	Message string `json:"Message"`
}

type userSignedUpEvent struct {
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Document   string `json:"document"`
	OccurredAt string `json:"occurred_at"`
}

func main() {
	cfg, err := config.Init()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	queueURL := os.Getenv("SQS_QUEUE_URL")
	if queueURL == "" {
		log.Fatal("SQS_QUEUE_URL is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	db, err := database.Init(ctx, *cfg.Database)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err = db.AutoMigrate(&usermodel.Model{}); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	repo := repository.NewUserRepository(db.GetDB())

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}
	sqsClient := sqs.NewFromConfig(awsCfg)

	log.Println("worker started, polling", queueURL)

	for {
		select {
		case <-ctx.Done():
			log.Println("worker shutting down")
			return
		default:
		}

		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			log.Printf("receive error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, msg := range out.Messages {
			if err := processMessage(ctx, msg, repo); err != nil {
				log.Printf("process error (will not delete): %v", err)
				continue
			}
			if _, err := sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: msg.ReceiptHandle,
			}); err != nil {
				log.Printf("delete message error: %v", err)
			}
		}
	}
}

func processMessage(ctx context.Context, msg types.Message, repo interface {
	Upsert(context.Context, *domain.User) error
}) error {
	var envelope snsEnvelope
	if err := json.Unmarshal([]byte(aws.ToString(msg.Body)), &envelope); err != nil {
		log.Printf("malformed SNS envelope, skipping: %v", err)
		return nil
	}

	var event userSignedUpEvent
	if err := json.Unmarshal([]byte(envelope.Message), &event); err != nil {
		log.Printf("malformed UserSignedUp event, skipping: %v", err)
		return nil
	}

	id, err := uuid.Parse(event.UserID)
	if err != nil {
		log.Printf("invalid user_id %q, skipping: %v", event.UserID, err)
		return nil
	}

	return repo.Upsert(ctx, &domain.User{
		ID:       id,
		Name:     event.Name,
		Email:    event.Email,
		Document: event.Document,
	})
}

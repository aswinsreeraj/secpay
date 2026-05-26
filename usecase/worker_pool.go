package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"secpay/domain"

	"github.com/google/uuid"
)

// WorkerPool manages concurrent worker Goroutines listening on the payment job channel.
type WorkerPool interface {
	Start()
	Stop()
	Enqueue(job domain.PaymentJob) error
	GetMetrics() domain.WorkerMetrics
}

type workerPool struct {
	paymentUsecase  PaymentUsecase
	idempotencyRepo domain.IdempotencyRepository
	txRepo          domain.TransactionRepository
	numWorkers      int
	queue           chan domain.PaymentJob
	metrics         domain.WorkerMetrics
	wg              sync.WaitGroup
	stopChan        chan struct{}
	mu              sync.Mutex
	isStopped       bool
}

// NewWorkerPool initializes the WorkerPool manager.
func NewWorkerPool(
	paymentUsecase PaymentUsecase,
	idempotencyRepo domain.IdempotencyRepository,
	txRepo domain.TransactionRepository,
	numWorkers int,
	queueSize int,
) WorkerPool {
	return &workerPool{
		paymentUsecase:  paymentUsecase,
		idempotencyRepo: idempotencyRepo,
		txRepo:          txRepo,
		numWorkers:      numWorkers,
		queue:           make(chan domain.PaymentJob, queueSize),
		stopChan:        make(chan struct{}),
	}
}

func (p *workerPool) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.isStopped {
		return
	}

	log.Printf("Starting concurrent worker pool with %d workers...", p.numWorkers)
	for i := 0; i < p.numWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

func (p *workerPool) Stop() {
	p.mu.Lock()
	if p.isStopped {
		p.mu.Unlock()
		return
	}
	p.isStopped = true
	p.mu.Unlock()

	log.Println("Stopping concurrent worker pool...")
	close(p.stopChan)
	close(p.queue)
	p.wg.Wait()
	log.Println("Worker pool stopped successfully.")
}

func (p *workerPool) Enqueue(job domain.PaymentJob) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.isStopped {
		return fmt.Errorf("cannot enqueue to a stopped worker pool")
	}
	select {
	case p.queue <- job:
		return nil
	default:
		return fmt.Errorf("payment job queue is full")
	}
}

func (p *workerPool) GetMetrics() domain.WorkerMetrics {
	return p.metrics
}

func (p *workerPool) worker(id int) {
	p.metrics.IncrementActive()
	defer p.metrics.DecrementActive()
	defer p.wg.Done()

	for {
		select {
		case job, ok := <-p.queue:
			if !ok {
				return
			}
			p.processJob(job)
		case <-p.stopChan:
			return
		}
	}
}

func (p *workerPool) processJob(job domain.PaymentJob) {
	ctx := job.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	txRecord, err := p.paymentUsecase.ProcessPayment(ctx, job.FromAccountID, job.ToAccountID, job.Amount, job.Description)
	if err != nil {
		if err == ErrTransient {
			// 1. Temporary failure: check if we should retry
			if job.RetryCount < job.MaxRetries {
				p.metrics.IncrementRetried()
				job.RetryCount++
				
				log.Printf("[Worker] Temporary failure for job %s. Retrying in %v (Retry %d/%d)...",
					job.ID, job.Backoff, job.RetryCount, job.MaxRetries)

				// Asynchronous non-blocking retry using time.AfterFunc
				time.AfterFunc(job.Backoff, func() {
					job.Backoff *= 2 // Exponential backoff scaling
					p.mu.Lock()
					defer p.mu.Unlock()
					if !p.isStopped {
						p.queue <- job
					}
				})
				return
			}

			// Max retries exceeded for transient error
			err = fmt.Errorf("transaction failed after %d retries: database transient error", job.MaxRetries)
			
			// Generate and write failed transaction log to DB
			txRecord = &domain.Transaction{
				ID:              uuid.NewString(),
				FromAccountID:   job.FromAccountID,
				ToAccountID:     job.ToAccountID,
				Amount:          job.Amount,
				TransactionType: "transfer",
				Status:          domain.TransactionStateFailed,
				Description:     fmt.Sprintf("%s (Failed: %v)", job.Description, err),
				CreatedAt:       time.Now(),
			}
			_ = p.txRepo.Create(ctx, txRecord)
		}

		// 2. Terminal failure (or max retries exceeded): cache the failed payload
		p.metrics.IncrementFailed()
		p.metrics.IncrementProcessed()
		
		responseObj := map[string]interface{}{
			"error":       err.Error(),
			"transaction": txRecord,
		}
		p.cacheResponse(ctx, job.IdempotencyKey, http.StatusBadRequest, responseObj)
		return
	}

	// 3. Success path: cache success payload
	p.metrics.IncrementProcessed()
	responseObj := map[string]interface{}{
		"message":     "Payment processed successfully",
		"transaction": txRecord,
	}
	p.cacheResponse(ctx, job.IdempotencyKey, http.StatusOK, responseObj)
}

func (p *workerPool) cacheResponse(ctx context.Context, key string, statusCode int, responseObj interface{}) {
	responseBytes, _ := json.Marshal(responseObj)
	
	record, err := p.idempotencyRepo.Get(ctx, key)
	if err == nil {
		record.Status = "completed"
		record.ResponseCode = statusCode
		record.ResponseBody = string(responseBytes)
		record.UpdatedAt = time.Now()
		_ = p.idempotencyRepo.Update(ctx, record)
	}
}

// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"right-sizer/logger"
	"right-sizer/metrics"
)

// RetryableError represents an error that can be retried
type RetryableError struct {
	Err       error
	Retryable bool
}

func (r *RetryableError) Error() string {
	return r.Err.Error()
}

// IsRetryable returns true if the error can be retried
func (r *RetryableError) IsRetryable() bool {
	return r.Retryable
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error, retryable bool) *RetryableError {
	return &RetryableError{Err: err, Retryable: retryable}
}

// Config holds retry configuration
type Config struct {
	MaxRetries          int
	InitialDelay        time.Duration
	MaxDelay            time.Duration
	BackoffFactor       float64
	RandomizationFactor float64
	Timeout             time.Duration
}

// DefaultConfig returns a default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxRetries:          3,
		InitialDelay:        100 * time.Millisecond,
		MaxDelay:            10 * time.Second,
		BackoffFactor:       2.0,
		RandomizationFactor: 0.1,
		Timeout:             30 * time.Second,
	}
}

// RetryFunc is a function that can be retried
type RetryFunc func() error

// RetryFuncWithContext is a function that can be retried with context
type RetryFuncWithContext func(ctx context.Context) error

// Retryer handles retry logic with exponential backoff
type Retryer struct {
	config  Config
	metrics *metrics.OperatorMetrics
}

// New creates a new Retryer
func New(config Config, metrics *metrics.OperatorMetrics) *Retryer {
	return &Retryer{
		config:  config,
		metrics: metrics,
	}
}

// Do executes the function with retry logic
func (r *Retryer) Do(operation string, fn RetryFunc) error {
	return r.DoWithContext(context.Background(), operation, func(ctx context.Context) error {
		return fn()
	})
}

// DoWithContext executes the function with retry logic and context
func (r *Retryer) DoWithContext(ctx context.Context, operation string, fn RetryFuncWithContext) error {
	if r.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.config.Timeout)
		defer cancel()
	}

	delay := r.config.InitialDelay
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if r.metrics != nil {
			r.metrics.RecordRetryAttempt(operation, attempt+1)
		}

		// Execute the function
		err := fn(ctx)
		if err == nil {
			if attempt > 0 && r.metrics != nil {
				r.metrics.RecordRetrySuccess(operation)
				logger.Info("Operation %s succeeded after %d retries", operation, attempt)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if retryableErr, ok := err.(*RetryableError); ok && !retryableErr.IsRetryable() {
			logger.Warn("Operation %s failed with non-retryable error: %v", operation, err)
			return err
		}

		// Check if we've exhausted retries
		if attempt >= r.config.MaxRetries {
			logger.Error("Operation %s failed after %d attempts: %v", operation, attempt+1, err)
			break
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			logger.Warn("Operation %s canceled during retry attempt %d", operation, attempt+1)
			return ctx.Err()
		default:
		}

		// Calculate next delay with exponential backoff and jitter
		nextDelay := r.calculateDelay(delay, attempt)
		logger.Debug("Operation %s failed (attempt %d/%d), retrying in %v: %v",
			operation, attempt+1, r.config.MaxRetries+1, nextDelay, err)

		// Sleep before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(nextDelay):
		}

		delay = time.Duration(float64(delay) * r.config.BackoffFactor)
		if delay > r.config.MaxDelay {
			delay = r.config.MaxDelay
		}
	}

	return fmt.Errorf("operation %s failed after %d attempts: %w", operation, r.config.MaxRetries+1, lastErr)
}

// calculateDelay calculates the delay for the next retry with jitter
func (r *Retryer) calculateDelay(baseDelay time.Duration, attempt int) time.Duration {
	// Apply exponential backoff
	delay := time.Duration(float64(baseDelay) * math.Pow(r.config.BackoffFactor, float64(attempt)))

	// Cap at max delay
	if delay > r.config.MaxDelay {
		delay = r.config.MaxDelay
	}

	// Add randomization (jitter)
	if r.config.RandomizationFactor > 0 {
		jitter := float64(delay) * r.config.RandomizationFactor * (rand.Float64()*2 - 1)
		delay = time.Duration(float64(delay) + jitter)
	}

	// Ensure minimum delay
	if delay < time.Millisecond {
		delay = time.Millisecond
	}

	return delay
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold int
	RecoveryTimeout  time.Duration
	SuccessThreshold int
}

// DefaultCircuitBreakerConfig returns default circuit breaker configuration
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
		SuccessThreshold: 3,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config          CircuitBreakerConfig
	state           CircuitBreakerState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	mutex           sync.RWMutex
	metrics         *metrics.OperatorMetrics
	name            string
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, config CircuitBreakerConfig, metrics *metrics.OperatorMetrics) *CircuitBreaker {
	return &CircuitBreaker{
		config:  config,
		state:   StateClosed,
		metrics: metrics,
		name:    name,
	}
}

// Execute executes the function through the circuit breaker
func (cb *CircuitBreaker) Execute(fn RetryFunc) error {
	return cb.ExecuteWithContext(context.Background(), func(ctx context.Context) error {
		return fn()
	})
}

// ExecuteWithContext executes the function through the circuit breaker with context
func (cb *CircuitBreaker) ExecuteWithContext(ctx context.Context, fn RetryFuncWithContext) error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check if circuit should transition to half-open
	if cb.state == StateOpen && time.Since(cb.lastFailureTime) >= cb.config.RecoveryTimeout {
		cb.state = StateHalfOpen
		cb.successCount = 0
		logger.Info("Circuit breaker %s transitioned to HALF_OPEN", cb.name)
	}

	// If circuit is open, fail fast
	if cb.state == StateOpen {
		return NewRetryableError(fmt.Errorf("circuit breaker %s is OPEN", cb.name), false)
	}

	// Execute the function
	err := fn(ctx)
	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

// onSuccess handles successful execution
func (cb *CircuitBreaker) onSuccess() {
	cb.failureCount = 0

	if cb.state == StateHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.state = StateClosed
			cb.successCount = 0
			logger.Info("Circuit breaker %s transitioned to CLOSED", cb.name)
		}
	}
}

// onFailure handles failed execution
func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.state == StateClosed && cb.failureCount >= cb.config.FailureThreshold {
		cb.state = StateOpen
		logger.Warn("Circuit breaker %s transitioned to OPEN after %d failures", cb.name, cb.failureCount)
	} else if cb.state == StateHalfOpen {
		cb.state = StateOpen
		logger.Warn("Circuit breaker %s transitioned back to OPEN from HALF_OPEN", cb.name)
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() (state CircuitBreakerState, failures int, successes int) {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state, cb.failureCount, cb.successCount
}

// RetryWithCircuitBreaker combines retry logic with circuit breaker
type RetryWithCircuitBreaker struct {
	retryer        *Retryer
	circuitBreaker *CircuitBreaker
}

// NewRetryWithCircuitBreaker creates a new retry handler with circuit breaker
func NewRetryWithCircuitBreaker(name string, retryConfig Config, cbConfig CircuitBreakerConfig, metrics *metrics.OperatorMetrics) *RetryWithCircuitBreaker {
	return &RetryWithCircuitBreaker{
		retryer:        New(retryConfig, metrics),
		circuitBreaker: NewCircuitBreaker(name, cbConfig, metrics),
	}
}

// Execute executes the function with both retry and circuit breaker logic
func (r *RetryWithCircuitBreaker) Execute(operation string, fn RetryFunc) error {
	return r.ExecuteWithContext(context.Background(), operation, func(ctx context.Context) error {
		return fn()
	})
}

// ExecuteWithContext executes the function with both retry and circuit breaker logic and context
func (r *RetryWithCircuitBreaker) ExecuteWithContext(ctx context.Context, operation string, fn RetryFuncWithContext) error {
	return r.retryer.DoWithContext(ctx, operation, func(ctx context.Context) error {
		return r.circuitBreaker.ExecuteWithContext(ctx, fn)
	})
}

// GetCircuitBreakerState returns the current circuit breaker state
func (r *RetryWithCircuitBreaker) GetCircuitBreakerState() CircuitBreakerState {
	return r.circuitBreaker.GetState()
}

// IsRetryableKubernetesError determines if a Kubernetes error should be retried
func IsRetryableKubernetesError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Retryable error patterns
	retryablePatterns := []string{
		"connection refused",
		"timeout",
		"context deadline exceeded",
		"temporary failure",
		"server is currently unavailable",
		"too many requests",
		"service unavailable",
		"internal server error",
		"bad gateway",
		"gateway timeout",
		"connection reset",
		"EOF",
		"i/o timeout",
	}

	for _, pattern := range retryablePatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive) - used for error pattern matching
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func containsSubstring(s, substr string) bool {
	// This function is now redundant but kept for compatibility
	return contains(s, substr)
}

// WrapKubernetesError wraps a Kubernetes error as retryable or non-retryable
func WrapKubernetesError(err error) error {
	if err == nil {
		return nil
	}

	return NewRetryableError(err, IsRetryableKubernetesError(err))
}

// RetryManager provides a simple interface for retry operations
type RetryManager struct {
	retryer *Retryer
}

// NewRetryManager creates a new RetryManager with the given configuration
func NewRetryManager(config Config) *RetryManager {
	return &RetryManager{
		retryer: New(config, nil),
	}
}

// RetryWithBackoff performs an operation with exponential backoff retry
func (rm *RetryManager) RetryWithBackoff(fn func() error) error {
	return rm.retryer.Do("operation", fn)
}

// BackoffStrategy represents different backoff strategies
type BackoffStrategy int

const (
	ExponentialBackoff BackoffStrategy = iota
	LinearBackoff
	ConstantBackoff
)

// CustomRetryer allows for more customized retry behavior
type CustomRetryer struct {
	maxRetries  int
	strategy    BackoffStrategy
	baseDelay   time.Duration
	maxDelay    time.Duration
	factor      float64
	shouldRetry func(error, int) bool
	onRetry     func(error, int)
	metrics     *metrics.OperatorMetrics
}

// CustomRetryConfig holds configuration for CustomRetryer
type CustomRetryConfig struct {
	MaxRetries  int
	Strategy    BackoffStrategy
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Factor      float64
	ShouldRetry func(error, int) bool
	OnRetry     func(error, int)
}

// NewCustomRetryer creates a new custom retryer
func NewCustomRetryer(config CustomRetryConfig, metrics *metrics.OperatorMetrics) *CustomRetryer {
	if config.ShouldRetry == nil {
		config.ShouldRetry = func(err error, attempt int) bool {
			return IsRetryableKubernetesError(err)
		}
	}

	return &CustomRetryer{
		maxRetries:  config.MaxRetries,
		strategy:    config.Strategy,
		baseDelay:   config.BaseDelay,
		maxDelay:    config.MaxDelay,
		factor:      config.Factor,
		shouldRetry: config.ShouldRetry,
		onRetry:     config.OnRetry,
		metrics:     metrics,
	}
}

// Execute executes the function with custom retry logic
func (cr *CustomRetryer) Execute(operation string, fn RetryFunc) error {
	return cr.ExecuteWithContext(context.Background(), operation, func(ctx context.Context) error {
		return fn()
	})
}

// ExecuteWithContext executes the function with custom retry logic and context
func (cr *CustomRetryer) ExecuteWithContext(ctx context.Context, operation string, fn RetryFuncWithContext) error {
	var lastErr error

	for attempt := 0; attempt <= cr.maxRetries; attempt++ {
		// Check if context is already cancelled before attempting
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if cr.metrics != nil {
			cr.metrics.RecordRetryAttempt(operation, attempt+1)
		}

		err := fn(ctx)
		if err == nil {
			if attempt > 0 && cr.metrics != nil {
				cr.metrics.RecordRetrySuccess(operation)
			}
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !cr.shouldRetry(err, attempt) {
			return err
		}

		// Check if we've exhausted retries
		if attempt >= cr.maxRetries {
			break
		}

		// Call onRetry callback if provided
		if cr.onRetry != nil {
			cr.onRetry(err, attempt)
		}

		// Calculate delay based on strategy
		delay := cr.calculateDelayForStrategy(attempt)

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return fmt.Errorf("operation %s failed after %d attempts: %w", operation, cr.maxRetries+1, lastErr)
}

// calculateDelayForStrategy calculates delay based on the configured strategy
func (cr *CustomRetryer) calculateDelayForStrategy(attempt int) time.Duration {
	var delay time.Duration

	switch cr.strategy {
	case ExponentialBackoff:
		delay = time.Duration(float64(cr.baseDelay) * math.Pow(cr.factor, float64(attempt)))
	case LinearBackoff:
		delay = time.Duration(float64(cr.baseDelay) * (1 + cr.factor*float64(attempt)))
	case ConstantBackoff:
		delay = cr.baseDelay
	default:
		delay = cr.baseDelay
	}

	if delay > cr.maxDelay {
		delay = cr.maxDelay
	}

	return delay
}

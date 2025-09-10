package retry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"right-sizer/metrics"

	"github.com/stretchr/testify/assert"
)

func TestRetryableError(t *testing.T) {
	err := errors.New("test error")
	retryableErr := NewRetryableError(err, true)

	assert.NotNil(t, retryableErr)
	assert.Equal(t, "test error", retryableErr.Error())
	assert.True(t, retryableErr.IsRetryable())

	nonRetryableErr := NewRetryableError(err, false)
	assert.False(t, nonRetryableErr.IsRetryable())
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.InitialDelay)
	assert.Equal(t, 10*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.BackoffFactor)
	assert.Equal(t, 0.1, config.RandomizationFactor)
	assert.Equal(t, 30*time.Second, config.Timeout)
}

func TestNew(t *testing.T) {
	config := DefaultConfig()
	metrics := metrics.NewOperatorMetrics()
	retryer := New(config, metrics)

	assert.NotNil(t, retryer)
	assert.Equal(t, config, retryer.config)
	assert.Equal(t, metrics, retryer.metrics)
}

func TestRetryer_Do_Success(t *testing.T) {
	config := Config{MaxRetries: 1, InitialDelay: 1 * time.Millisecond}
	retryer := New(config, nil)

	callCount := 0
	err := retryer.Do("test", func() error {
		callCount++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestRetryer_Do_FailureThenSuccess(t *testing.T) {
	config := Config{MaxRetries: 2, InitialDelay: 1 * time.Millisecond}
	retryer := New(config, nil)

	callCount := 0
	err := retryer.Do("test", func() error {
		callCount++
		if callCount == 1 {
			return errors.New("temporary failure")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestRetryer_Do_ExhaustRetries(t *testing.T) {
	config := Config{MaxRetries: 2, InitialDelay: 1 * time.Millisecond}
	retryer := New(config, nil)

	callCount := 0
	err := retryer.Do("test", func() error {
		callCount++
		return errors.New("persistent failure")
	})

	assert.Error(t, err)
	assert.Equal(t, 3, callCount) // initial + 2 retries
	assert.Contains(t, err.Error(), "failed after 3 attempts")
}

func TestRetryer_DoWithContext_Cancellation(t *testing.T) {
	config := Config{MaxRetries: 5, InitialDelay: 10 * time.Millisecond}
	retryer := New(config, nil)

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	err := retryer.DoWithContext(ctx, "test", func(ctx context.Context) error {
		callCount++
		if callCount == 2 {
			cancel()
		}
		return errors.New("failure")
	})

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Equal(t, 2, callCount)
}

func TestRetryer_DoWithContext_Timeout(t *testing.T) {
	config := Config{
		MaxRetries:   5,
		InitialDelay: 10 * time.Millisecond,
		Timeout:      50 * time.Millisecond,
	}
	retryer := New(config, nil)

	callCount := 0
	start := time.Now()
	err := retryer.DoWithContext(context.Background(), "test", func(ctx context.Context) error {
		callCount++
		time.Sleep(20 * time.Millisecond)
		return errors.New("failure")
	})
	duration := time.Since(start)

	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "context deadline exceeded") ||
		strings.Contains(err.Error(), "timeout"))
	assert.True(t, duration < 200*time.Millisecond) // Should timeout quickly
}

func TestRetryer_CalculateDelay(t *testing.T) {
	config := Config{
		InitialDelay:        100 * time.Millisecond,
		MaxDelay:            1 * time.Second,
		BackoffFactor:       2.0,
		RandomizationFactor: 0.1,
	}
	retryer := New(config, nil)

	// Test first attempt
	delay1 := retryer.calculateDelay(config.InitialDelay, 0)
	assert.True(t, delay1 >= 90*time.Millisecond && delay1 <= 110*time.Millisecond)

	// Test second attempt with backoff
	delay2 := retryer.calculateDelay(config.InitialDelay, 1)
	assert.True(t, delay2 >= 180*time.Millisecond && delay2 <= 220*time.Millisecond)

	// Test max delay cap
	delay3 := retryer.calculateDelay(config.InitialDelay, 10) // Should be capped
	// Allow variance due to randomization factor (10% = 100ms for 1s max delay)
	assert.InDelta(t, float64(config.MaxDelay), float64(delay3), float64(config.MaxDelay)*config.RandomizationFactor)
}

func TestRetryer_CalculateDelay_NoRandomization(t *testing.T) {
	config := Config{
		InitialDelay:        100 * time.Millisecond,
		MaxDelay:            1 * time.Second,
		BackoffFactor:       2.0,
		RandomizationFactor: 0.0, // No randomization
	}
	retryer := New(config, nil)

	delay := retryer.calculateDelay(config.InitialDelay, 0)
	assert.Equal(t, 100*time.Millisecond, delay)
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config, nil)

	callCount := 0
	err := cb.Execute(func() error {
		callCount++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_Execute_Failure(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  100 * time.Millisecond,
	}
	cb := NewCircuitBreaker("test", config, nil)

	// First failure
	err1 := cb.Execute(func() error {
		return errors.New("failure")
	})
	assert.Error(t, err1)
	assert.Equal(t, StateClosed, cb.GetState())

	// Second failure - should open circuit
	err2 := cb.Execute(func() error {
		return errors.New("failure")
	})
	assert.Error(t, err2)
	assert.Equal(t, StateOpen, cb.GetState())

	// Third attempt - should fail fast
	err3 := cb.Execute(func() error {
		return nil // This shouldn't be called
	})
	assert.Error(t, err3)
	assert.Equal(t, StateOpen, cb.GetState())
	assert.Contains(t, err3.Error(), "circuit breaker test is OPEN")
}

func TestCircuitBreaker_Recovery(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  50 * time.Millisecond,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker("test", config, nil)

	// Fail circuit
	cb.Execute(func() error { return errors.New("failure") })
	cb.Execute(func() error { return errors.New("failure") })
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for recovery timeout
	time.Sleep(60 * time.Millisecond)

	// First success in half-open state
	err := cb.Execute(func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Second success should close circuit
	err = cb.Execute(func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_ExecuteWithContext(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := cb.ExecuteWithContext(ctx, func(ctx context.Context) error {
		return nil
	})

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestCircuitBreaker_GetStats(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config, nil)

	// Initial state
	state, failures, successes := cb.GetStats()
	assert.Equal(t, StateClosed, state)
	assert.Equal(t, 0, failures)
	assert.Equal(t, 0, successes)

	// After failure
	cb.Execute(func() error { return errors.New("failure") })
	state, failures, successes = cb.GetStats()
	assert.Equal(t, StateClosed, state)
	assert.Equal(t, 1, failures)
	assert.Equal(t, 0, successes)
}

func TestCircuitBreakerState_String(t *testing.T) {
	assert.Equal(t, "CLOSED", StateClosed.String())
	assert.Equal(t, "OPEN", StateOpen.String())
	assert.Equal(t, "HALF_OPEN", StateHalfOpen.String())
	assert.Equal(t, "UNKNOWN", CircuitBreakerState(999).String())
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()

	assert.Equal(t, 5, config.FailureThreshold)
	assert.Equal(t, 30*time.Second, config.RecoveryTimeout)
	assert.Equal(t, 3, config.SuccessThreshold)
}

func TestRetryWithCircuitBreaker_Execute(t *testing.T) {
	retryConfig := Config{MaxRetries: 1, InitialDelay: 1 * time.Millisecond}
	cbConfig := CircuitBreakerConfig{FailureThreshold: 3, RecoveryTimeout: 100 * time.Millisecond}
	rcb := NewRetryWithCircuitBreaker("test", retryConfig, cbConfig, nil)

	callCount := 0
	err := rcb.Execute("test-op", func() error {
		callCount++
		if callCount <= 2 {
			return errors.New("failure")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount) // 2 failures + 1 success
}

func TestRetryWithCircuitBreaker_GetCircuitBreakerState(t *testing.T) {
	retryConfig := DefaultConfig()
	cbConfig := DefaultCircuitBreakerConfig()
	rcb := NewRetryWithCircuitBreaker("test", retryConfig, cbConfig, nil)

	assert.Equal(t, StateClosed, rcb.GetCircuitBreakerState())
}

func TestIsRetryableKubernetesError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"connection refused", errors.New("dial tcp: connection refused"), true},
		{"timeout", errors.New("context deadline exceeded"), true},
		{"temporary failure", errors.New("temporary failure in name resolution"), true},
		{"server unavailable", errors.New("server is currently unavailable"), true},
		{"too many requests", errors.New("too many requests"), true},
		{"service unavailable", errors.New("503 Service Unavailable"), true},
		{"internal server error", errors.New("500 Internal Server Error"), true},
		{"bad gateway", errors.New("502 Bad Gateway"), true},
		{"gateway timeout", errors.New("504 Gateway Timeout"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"EOF", errors.New("unexpected EOF"), true},
		{"i/o timeout", errors.New("i/o timeout"), true},
		{"non-retryable error", errors.New("not found"), false},
		{"validation error", errors.New("validation failed"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableKubernetesError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWrapKubernetesError(t *testing.T) {
	originalErr := errors.New("connection refused")
	wrappedErr := WrapKubernetesError(originalErr)

	assert.NotNil(t, wrappedErr)
	retryableErr, ok := wrappedErr.(*RetryableError)
	assert.True(t, ok)
	assert.True(t, retryableErr.IsRetryable())
	assert.Equal(t, "connection refused", retryableErr.Error())

	// Test with nil error
	assert.Nil(t, WrapKubernetesError(nil))

	// Test with non-retryable error
	nonRetryable := errors.New("not found")
	wrappedNonRetryable := WrapKubernetesError(nonRetryable)
	retryableNonRetryable, ok := wrappedNonRetryable.(*RetryableError)
	assert.True(t, ok)
	assert.False(t, retryableNonRetryable.IsRetryable())
}

func TestRetryManager(t *testing.T) {
	config := Config{MaxRetries: 1, InitialDelay: 1 * time.Millisecond}
	manager := NewRetryManager(config)

	callCount := 0
	err := manager.RetryWithBackoff(func() error {
		callCount++
		if callCount == 1 {
			return errors.New("failure")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestCustomRetryer_Execute_Success(t *testing.T) {
	config := CustomRetryConfig{
		MaxRetries: 2,
		Strategy:   ExponentialBackoff,
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Factor:     2.0,
	}
	retryer := NewCustomRetryer(config, nil)

	callCount := 0
	err := retryer.Execute("test", func() error {
		callCount++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestCustomRetryer_Execute_ExponentialBackoff(t *testing.T) {
	config := CustomRetryConfig{
		MaxRetries: 2,
		Strategy:   ExponentialBackoff,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Factor:     2.0,
	}
	retryer := NewCustomRetryer(config, nil)

	callCount := 0
	start := time.Now()
	err := retryer.Execute("test", func() error {
		callCount++
		if callCount <= 2 {
			return errors.New("failure")
		}
		return nil
	})
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
	assert.True(t, duration >= 30*time.Millisecond) // 10ms + 20ms delays
}

func TestCustomRetryer_Execute_LinearBackoff(t *testing.T) {
	config := CustomRetryConfig{
		MaxRetries: 2,
		Strategy:   LinearBackoff,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Factor:     1.5,
	}
	retryer := NewCustomRetryer(config, nil)

	callCount := 0
	start := time.Now()
	err := retryer.Execute("test", func() error {
		callCount++
		if callCount <= 2 {
			return errors.New("failure")
		}
		return nil
	})
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
	assert.True(t, duration >= 35*time.Millisecond) // 10ms + 25ms delays
}

func TestCustomRetryer_Execute_ConstantBackoff(t *testing.T) {
	config := CustomRetryConfig{
		MaxRetries: 2,
		Strategy:   ConstantBackoff,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Factor:     2.0,
	}
	retryer := NewCustomRetryer(config, nil)

	callCount := 0
	start := time.Now()
	err := retryer.Execute("test", func() error {
		callCount++
		if callCount <= 2 {
			return errors.New("failure")
		}
		return nil
	})
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
	assert.True(t, duration >= 20*time.Millisecond) // 10ms + 10ms delays
}

func TestCustomRetryer_Execute_CustomShouldRetry(t *testing.T) {
	callCount := 0
	config := CustomRetryConfig{
		MaxRetries: 3,
		Strategy:   ConstantBackoff,
		BaseDelay:  1 * time.Millisecond,
		ShouldRetry: func(err error, attempt int) bool {
			callCount++
			return attempt < 2 // Only retry twice
		},
	}
	retryer := NewCustomRetryer(config, nil)

	err := retryer.Execute("test", func() error {
		return errors.New("persistent failure")
	})

	assert.Error(t, err)
	assert.Equal(t, 3, callCount) // Should retry twice, then stop
}

func TestCustomRetryer_Execute_OnRetryCallback(t *testing.T) {
	retryCount := 0
	config := CustomRetryConfig{
		MaxRetries: 2,
		Strategy:   ConstantBackoff,
		BaseDelay:  1 * time.Millisecond,
		OnRetry: func(err error, attempt int) {
			retryCount++
		},
	}
	retryer := NewCustomRetryer(config, nil)

	err := retryer.Execute("test", func() error {
		return errors.New("failure")
	})

	assert.Error(t, err)
	assert.Equal(t, 2, retryCount)
}

func TestCustomRetryer_ExecuteWithContext_Cancellation(t *testing.T) {
	config := CustomRetryConfig{
		MaxRetries: 5,
		Strategy:   ConstantBackoff,
		BaseDelay:  10 * time.Millisecond,
	}
	retryer := NewCustomRetryer(config, nil)

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	err := retryer.ExecuteWithContext(ctx, "test", func(ctx context.Context) error {
		callCount++
		if callCount == 2 {
			cancel()
		}
		return errors.New("failure")
	})

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Equal(t, 2, callCount)
}

func TestCustomRetryer_CalculateDelayForStrategy_Exponential(t *testing.T) {
	config := CustomRetryConfig{
		Strategy:  ExponentialBackoff,
		BaseDelay: 10 * time.Millisecond,
		MaxDelay:  100 * time.Millisecond,
		Factor:    2.0,
	}
	retryer := NewCustomRetryer(config, nil)

	// Test various attempts
	assert.Equal(t, 10*time.Millisecond, retryer.calculateDelayForStrategy(0))
	assert.Equal(t, 20*time.Millisecond, retryer.calculateDelayForStrategy(1))
	assert.Equal(t, 40*time.Millisecond, retryer.calculateDelayForStrategy(2))
	assert.Equal(t, 80*time.Millisecond, retryer.calculateDelayForStrategy(3))
	assert.Equal(t, 100*time.Millisecond, retryer.calculateDelayForStrategy(4)) // Capped at MaxDelay
}

func TestCustomRetryer_CalculateDelayForStrategy_Linear(t *testing.T) {
	config := CustomRetryConfig{
		Strategy:  LinearBackoff,
		BaseDelay: 10 * time.Millisecond,
		MaxDelay:  100 * time.Millisecond,
		Factor:    1.5,
	}
	retryer := NewCustomRetryer(config, nil)

	assert.Equal(t, 10*time.Millisecond, retryer.calculateDelayForStrategy(0))
	assert.Equal(t, 25*time.Millisecond, retryer.calculateDelayForStrategy(1)) // 10 * (1 + 1.5*1)
	assert.Equal(t, 40*time.Millisecond, retryer.calculateDelayForStrategy(2)) // 10 * (1 + 1.5*2)
}

func TestCustomRetryer_CalculateDelayForStrategy_Constant(t *testing.T) {
	config := CustomRetryConfig{
		Strategy:  ConstantBackoff,
		BaseDelay: 10 * time.Millisecond,
		MaxDelay:  100 * time.Millisecond,
		Factor:    2.0,
	}
	retryer := NewCustomRetryer(config, nil)

	assert.Equal(t, 10*time.Millisecond, retryer.calculateDelayForStrategy(0))
	assert.Equal(t, 10*time.Millisecond, retryer.calculateDelayForStrategy(1))
	assert.Equal(t, 10*time.Millisecond, retryer.calculateDelayForStrategy(2))
}

func TestBackoffStrategy_Values(t *testing.T) {
	assert.Equal(t, BackoffStrategy(0), ExponentialBackoff)
	assert.Equal(t, BackoffStrategy(1), LinearBackoff)
	assert.Equal(t, BackoffStrategy(2), ConstantBackoff)
}

func TestContainsSubstring(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "goodbye", false},
		{"test", "test", true},
		{"test", "testing", false},
		{"", "", true},
		{"a", "", true},
		{"", "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := containsSubstring(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "WORLD", false}, // case sensitive
		{"test", "test", true},
		{"connection refused", "connection", true},
		{"timeout", "timeout", true},
		{"", "", true},
		{"a", "", true},
		{"", "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

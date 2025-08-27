# Logging Fix for Controller-Runtime Warning

## Issue Description

The right-sizer pod logs were showing the following warning message:

```
[controller-runtime] log.SetLogger(...) was never called; logs will not be displayed.
Detected at:
	>  goroutine 63 [running]:
	>  runtime/debug.Stack()
	>  	/usr/local/go/src/runtime/debug/stack.go:26 +0x64
	>  sigs.k8s.io/controller-runtime/pkg/log.eventuallyFulfillRoot()
...
```

This warning occurred because the controller-runtime library requires its logger to be explicitly initialized before use, but the right-sizer application wasn't setting up the controller-runtime logger.

## Root Cause

The right-sizer application has its own custom logger implementation in the `logger` package but didn't initialize the controller-runtime's logger. When controller-runtime components (like the manager and controllers) tried to log messages, they detected that no logger had been set and printed the warning stack trace.

## Solution

### Changes Made

1. **Added Dependencies** (in `go/go.mod`):
   - `github.com/go-logr/zapr` - Adapter to use zap with go-logr
   - `go.uber.org/zap` - High-performance logging library

2. **Modified main.go**:
   ```go
   import (
       // ... existing imports ...
       "github.com/go-logr/zapr"
       "go.uber.org/zap"
       ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
   )

   func main() {
       // ... existing code ...
       
       // Initialize logger with configured level
       logger.Init(cfg.LogLevel)

       // Initialize controller-runtime logger to prevent warnings
       zapLog, err := zap.NewProduction()
       if err != nil {
           // Fall back to development logger if production logger fails
           zapLog, _ = zap.NewDevelopment()
       }
       ctrllog.SetLogger(zapr.NewLogger(zapLog))
       
       // ... rest of the code ...
   }
   ```

### How the Fix Works

1. **Zap Logger Creation**: We create a zap logger instance (production mode for better performance, with fallback to development mode if needed)

2. **Logger Adapter**: The `zapr.NewLogger()` wraps the zap logger to make it compatible with the go-logr interface that controller-runtime expects

3. **Set Global Logger**: `ctrllog.SetLogger()` sets this as the global logger for all controller-runtime components

4. **Timing**: This initialization happens early in the main() function, right after the custom logger initialization, ensuring all controller-runtime components have access to a proper logger before they start

## Building and Deploying

### Build the Updated Image

```bash
# From the project root directory
docker build -t right-sizer:fixed .

# Or if you're using a registry
docker build -t your-registry/right-sizer:v1.0.1 .
docker push your-registry/right-sizer:v1.0.1
```

### Update Kubernetes Deployment

Update your right-sizer deployment to use the new image:

```bash
kubectl set image deployment/right-sizer right-sizer=your-registry/right-sizer:v1.0.1 -n right-sizer-system
```

Or edit the deployment directly:

```bash
kubectl edit deployment right-sizer -n right-sizer-system
```

### Verify the Fix

After deploying the updated version, check the logs:

```bash
kubectl logs -n right-sizer-system deployment/right-sizer -f
```

You should see:
- Clean log output without the controller-runtime warning
- Normal operational messages like "🔍 Analyzing X pods for right-sizing..."
- No stack traces related to logger initialization

## Benefits

1. **Cleaner Logs**: No more warning stack traces cluttering the logs
2. **Better Debugging**: Controller-runtime components can now properly log their messages
3. **Production Ready**: Uses zap's production configuration for optimal performance
4. **Graceful Fallback**: Falls back to development logger if production logger fails to initialize

## Testing

Run the test suite to ensure everything still works:

```bash
cd go
go test ./... -v
```

All tests should pass without any logger-related warnings.

## Notes

- The fix doesn't change any functional behavior of the right-sizer
- The application's custom logger remains unchanged and continues to work as before
- Both loggers (custom and controller-runtime) coexist without conflicts
- The zap logger is configured for production use with JSON output and sampling for performance
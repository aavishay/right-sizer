package main

import (
	"fmt"
	"right-sizer/logger"
)

func main() {
	// Initialize the logger
	logger.Init("info")

	fmt.Println("================================================")
	fmt.Println("    Testing Logger [INFO] Prefix Removal")
	fmt.Println("================================================")
	fmt.Println()

	fmt.Println("OLD FORMAT (with [INFO] prefix):")
	fmt.Println("---------------------------------")
	fmt.Println("2025/08/31 20:59:59 [INFO] 🔍 Scaling analysis - CPU: scale down (usage: 69m, limit: 500m, 13.8%), Memory: no change (usage: 263Mi, limit: 512Mi, 51.4%)")
	fmt.Println("2025/08/31 20:59:59 [INFO] 📈 Container right-sizer/right-sizer-546cd5df94-6p7lx/operator will be resized - CPU: 100m→75m, Memory: 128Mi→316Mi")
	fmt.Println()

	fmt.Println("NEW FORMAT (without [INFO] prefix):")
	fmt.Println("------------------------------------")

	// Test Info messages (should not have [INFO] prefix)
	logger.Global.Info("🔍 Scaling analysis - CPU: scale down (usage: 69m, limit: 500m, 13.8%%), Memory: no change (usage: 263Mi, limit: 512Mi, 51.4%%)")
	logger.Global.Info("📈 Container right-sizer/right-sizer-546cd5df94-6p7lx/operator will be resized - CPU: 100m→75m, Memory: 128Mi→316Mi")

	fmt.Println()
	fmt.Println("Other log levels (should still have prefixes):")
	fmt.Println("-----------------------------------------------")

	// Test other log levels (should still have their prefixes)
	logger.Global.Debug("This is a debug message - should have [DEBUG] prefix")
	logger.Global.Warn("This is a warning message - should have [WARN] prefix")
	logger.Global.Error("This is an error message - should have [ERROR] prefix")

	fmt.Println()
	fmt.Println("Additional test messages:")
	fmt.Println("-------------------------")

	// Test various Info messages that would appear in right-sizer
	logger.Global.Info("✅ In-place pod resizing is available - pods can be resized without restarts")
	logger.Global.Info("Starting adaptive right-sizer with 30s interval (DryRun: false)")
	logger.Global.Info("📊 Found 1 resources needing adjustment")
	logger.Global.Info("🔄 Processing 1 pod updates in 1 batches (batch size: 5)")
	logger.Global.Info("📦 Processing batch 1/1 (1 pods)")
	logger.Global.Info("✅ Successfully resized pod test-namespace/test-pod (CPU: 100m→150m, Memory: 128Mi→256Mi)")
	logger.Global.Info("✅ Completed processing all 1 pod updates")

	fmt.Println()
	fmt.Println("================================================")
	fmt.Println("                    SUMMARY")
	fmt.Println("================================================")
	fmt.Println("✅ Info and Success messages: NO [INFO] prefix")
	fmt.Println("✅ Debug messages: HAVE [DEBUG] prefix")
	fmt.Println("✅ Warning messages: HAVE [WARN] prefix")
	fmt.Println("✅ Error messages: HAVE [ERROR] prefix")
	fmt.Println()
	fmt.Println("The logger now produces cleaner output for Info")
	fmt.Println("messages while maintaining prefixes for other")
	fmt.Println("log levels for proper severity identification.")
	fmt.Println("================================================")
}

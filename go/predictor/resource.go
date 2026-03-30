package predictor

// Resource type constants for type safety
const (
	ResourceTypeCPU    = "cpu"
	ResourceTypeMemory = "memory"
)

// IsValidResourceType checks if the resource type is recognized
func IsValidResourceType(rt string) bool {
	return rt == ResourceTypeCPU || rt == ResourceTypeMemory
}

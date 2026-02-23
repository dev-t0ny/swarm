package port

// Allocator manages port assignment for dev servers.
type Allocator struct {
	BasePort int
	used     map[int]string
}

// NewAllocator creates a new port allocator starting at basePort.
func NewAllocator(basePort int) *Allocator {
	return &Allocator{
		BasePort: basePort,
		used:     make(map[int]string),
	}
}

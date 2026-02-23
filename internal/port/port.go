package port

import (
	"fmt"
	"net"
	"sync"
)

// Allocator manages port assignment for dev servers.
type Allocator struct {
	BasePort int
	mu       sync.Mutex
	used     map[int]string // port -> agent name
}

// NewAllocator creates a new port allocator starting at basePort.
func NewAllocator(basePort int) *Allocator {
	return &Allocator{
		BasePort: basePort,
		used:     make(map[int]string),
	}
}

// Allocate assigns the next available port for the given agent.
// It starts from BasePort and increments until it finds a free port.
func (a *Allocator) Allocate(agentName string) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for port := a.BasePort; port < a.BasePort+100; port++ {
		// Check if already allocated by us
		if _, taken := a.used[port]; taken {
			continue
		}
		// Check if actually available on the system
		if isPortAvailable(port) {
			a.used[port] = agentName
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", a.BasePort, a.BasePort+100)
}

// Release frees a port allocation.
func (a *Allocator) Release(port int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.used, port)
}

// ReleaseByAgent frees all ports allocated to an agent.
func (a *Allocator) ReleaseByAgent(agentName string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for port, name := range a.used {
		if name == agentName {
			delete(a.used, port)
		}
	}
}

// ReleaseAll frees all port allocations.
func (a *Allocator) ReleaseAll() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.used = make(map[int]string)
}

// GetPort returns the port allocated to an agent, or 0 if none.
func (a *Allocator) GetPort(agentName string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	for port, name := range a.used {
		if name == agentName {
			return port
		}
	}
	return 0
}

// isPortAvailable checks if a TCP port is free.
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

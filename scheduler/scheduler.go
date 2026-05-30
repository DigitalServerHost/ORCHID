/**
 * @file scheduler.go
 * @brief High-performance concurrent memory bank scheduler for Project ORCHID.
 * 
 * This module implements a thread-safe, banked queue scheduler designed to
 * coordinate memory operations across multiple physical channels or hardware memory banks.
 * It manages concurrent reads and writes, enforcing bank serialization fences while
 * enabling maximum parallel throughput across separate banks.
 * 
 * Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
 * Project Lead & Maintainer: Kevin West (@westkevin12)
 * License: GNU GPLv3
 */

package scheduler

import (
	"errors"
	"sync"
	"sync/atomic"
)

/**
 * @struct AccessEvent
 * @brief Holds detail logging metrics for a scheduled memory request.
 */
type AccessEvent struct {
	Role     string ///< Memory role ('A', 'B', or 'C')
	Kind     string ///< Access type ('READ' or 'WRITE')
	Index    int    ///< Index inside the stream vector
	Bank     int    ///< Target physical memory bank
	Earliest uint64 ///< The earliest cycle this request was logically ready
	Start    uint64 ///< Scheduled start cycle
	End      uint64 ///< Scheduled completion cycle
}

/**
 * @struct MemoryScheduler
 * @brief Manages thread-safe banked memory queue schedules.
 */
type MemoryScheduler struct {
	bankCount     int           ///< Number of simulated hardware memory banks
	serviceCycles uint64        ///< Service latency in CPU cycles per request
	freeAt        []uint64      ///< Slice of cycle clocks when each bank will become free
	requests      []uint64      ///< Request counters per bank
	bankLocks     []sync.Mutex  ///< Mutex fences protecting each physical memory bank
	trace         []AccessEvent ///< Log trace of scheduled events
	traceLimit    int           ///< Maximum event log tracing threshold
	traceMu       sync.Mutex    ///< Mutex protecting logging trace slices
}

/**
 * @brief Initializes a new high-performance MemoryScheduler.
 * 
 * @param bankCount Number of hardware memory banks.
 * @param serviceCycles Processing cycle overhead per request.
 * @param traceLimit Upper limit of events to trace in the trace logger.
 * @return Pointer to the initialized MemoryScheduler or an error.
 */
func NewMemoryScheduler(bankCount int, serviceCycles uint64, traceLimit int) (*MemoryScheduler, error) {
	if bankCount < 1 || serviceCycles < 1 {
		return nil, errors.New("bankCount and serviceCycles must be greater than or equal to 1")
	}

	return &MemoryScheduler{
		bankCount:     bankCount,
		serviceCycles: serviceCycles,
		freeAt:        make([]uint64, bankCount),
		requests:      make([]uint64, bankCount),
		bankLocks:     make([]sync.Mutex, bankCount),
		traceLimit:    traceLimit,
		trace:         make([]AccessEvent, 0, traceLimit),
	}, nil
}

/**
 * @brief Request thread-safe access to a specific memory bank.
 * 
 * Enforces bank-level serialization using active bankLocks, updates the bank's
 * availability register, and increments hardware request counters using atomics.
 * 
 * @param role The memory stream role identifier ('A', 'B', or 'C').
 * @param kind The operation type ('READ' or 'WRITE').
 * @param index The vector address index being accessed.
 * @param bank The physical bank ID to allocate.
 * @param earliest The logical ready cycle constraint (cannot start before this).
 * @return The final completion cycle of the memory operation.
 */
func (ms *MemoryScheduler) Access(role string, kind string, index int, bank int, earliest uint64) (uint64, error) {
	if bank < 0 || bank >= ms.bankCount {
		return 0, errors.New("requested memory bank index out of bounds")
	}

	// Acquire lock fence for the targeted physical memory bank
	ms.bankLocks[bank].Lock()
	defer ms.bankLocks[bank].Unlock()

	// Calculate scheduling start and end cycles
	currentFree := ms.freeAt[bank]
	startCycle := earliest
	if currentFree > startCycle {
		startCycle = currentFree
	}
	endCycle := startCycle + ms.serviceCycles

	// Update bank availability register
	ms.freeAt[bank] = endCycle

	// Atomically increment access metrics
	atomic.AddUint64(&ms.requests[bank], 1)

	// Log event to trace buffer if within limit
	ms.traceMu.Lock()
	if len(ms.trace) < ms.traceLimit {
		ms.trace = append(ms.trace, AccessEvent{
			Role:     role,
			Kind:     kind,
			Index:    index,
			Bank:     bank,
			Earliest: earliest,
			Start:    startCycle,
			End:      endCycle,
		})
	}
	ms.traceMu.Unlock()

	return endCycle, nil
}

/**
 * @brief Retrieves the total elapsed scheduling cycle count.
 * 
 * Finds the maximum cycle across all individual memory banks.
 * 
 * @return The overall completion cycle count.
 */
func (ms *MemoryScheduler) TotalCycles() uint64 {
	var maxCycles uint64
	for i := 0; i < ms.bankCount; i++ {
		ms.bankLocks[i].Lock()
		val := ms.freeAt[i]
		if val > maxCycles {
			maxCycles = val
		}
		ms.bankLocks[i].Unlock()
	}
	return maxCycles
}

/**
 * @brief Returns the total request count for a given bank.
 * 
 * @param bank The targeted physical bank ID.
 * @return Atomic request counter value.
 */
func (ms *MemoryScheduler) GetRequests(bank int) uint64 {
	if bank < 0 || bank >= ms.bankCount {
		return 0
	}
	return atomic.LoadUint64(&ms.requests[bank])
}

/**
 * @brief Retrieves a copy of the event trace buffer.
 * 
 * @return A slice containing recorded AccessEvents.
 */
func (ms *MemoryScheduler) GetTrace() []AccessEvent {
	ms.traceMu.Lock()
	defer ms.traceMu.Unlock()
	
	cpy := make([]AccessEvent, len(ms.trace))
	copy(cpy, ms.trace)
	return cpy
}

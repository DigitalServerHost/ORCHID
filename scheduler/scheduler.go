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
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

/**
 * @struct QueueItem
 * @brief Represents an item in the memory bank's lock-free ring buffer.
 */
type QueueItem struct {
	State    uint32        ///< 0: Idle, 1: Enqueued/Waiting, 2: Processing
	Role     string        ///< Memory role ('A', 'B', or 'C')
	Kind     string        ///< Access type ('READ' or 'WRITE')
	Index    int           ///< Index inside the stream vector
	Earliest uint64        ///< The earliest cycle this request was logically ready
	EndCycle uint64        ///< The scheduled completion cycle computed by the scheduler
	sem      chan struct{} ///< Semaphore channel to park the waiting goroutine
}

/**
 * @struct BankQueue
 * @brief Bounded, lock-free ring-buffer queue for a memory bank.
 * 
 * Arranges fields to guarantee 8-byte alignment for atomic head and tail cursors.
 */
type BankQueue struct {
	head     uint64      ///< Tail cursor index (write offset)
	tail     uint64      ///< Head cursor index (read offset)
	mask     uint64      ///< Bitmask for circular wrapping
	ring     []QueueItem ///< The slice backing the ring buffer
}

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
	bankQueues    []BankQueue   ///< Lock-free ring buffer queues protecting each bank
	trace         []AccessEvent ///< Log trace of scheduled events
	traceLimit    int           ///< Maximum event log tracing threshold
	traceMu       sync.Mutex    ///< Mutex protecting logging trace slices
	numaEnabled   bool          ///< Flag indicating if NUMA allocation is active
	numaBankMap   map[int]int   ///< Map linking each bank ID to its target physical NUMA node
	numaBuffers   map[int][]byte ///< Map holding the allocated mmap'ed buffers for each bank
	numaMu        sync.RWMutex  ///< Mutex protecting NUMA states and allocated bank buffers
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

	// Bounded ring-buffer queue size per bank (must be a power of two)
	// Set to 65536 to guarantee ticket slots never wrap around for active concurrent requests.
	const queueSize = 65536
	const queueMask = queueSize - 1

	bankQueues := make([]BankQueue, bankCount)
	for i := 0; i < bankCount; i++ {
		ring := make([]QueueItem, queueSize)
		for j := 0; j < queueSize; j++ {
			ring[j] = QueueItem{
				State: 0,
				sem:   make(chan struct{}, 1),
			}
		}
		bankQueues[i] = BankQueue{
			head: 0,
			tail: 0,
			mask: queueMask,
			ring: ring,
		}
	}

	return &MemoryScheduler{
		bankCount:     bankCount,
		serviceCycles: serviceCycles,
		freeAt:        make([]uint64, bankCount),
		requests:      make([]uint64, bankCount),
		bankQueues:    bankQueues,
		traceLimit:    traceLimit,
		trace:         make([]AccessEvent, 0, traceLimit),
		numaBankMap:   make(map[int]int),
		numaBuffers:   make(map[int][]byte),
	}, nil
}

/**
 * @brief Wait for the queue ticket's turn using channel handoff or bypass.
 */
func (q *BankQueue) waitTurn(ticket uint64, idx uint64) {
	h := atomic.LoadUint64(&q.head)
	if h == ticket {
		// Try to claim the bypassed state transition.
		if !atomic.CompareAndSwapUint32(&q.ring[idx].State, 1, 2) {
			// Previous thread already claimed and signaled us, consume the token.
			<-q.ring[idx].sem
		}
	} else {
		// Park the goroutine until the previous holder signals us.
		<-q.ring[idx].sem
	}
}

/**
 * @brief Performs state transition checks and signals the next ticket holder if enqueued.
 * 
 * @param nextTicket The next ticket sequence number in the queue.
 * @param nextIdx The next ticket index mapping into the circular queue ring buffer.
 * @return True if signaling is done (either completed, signaled, or bypassed), false to retry.
 */
func (q *BankQueue) checkAndSignal(nextTicket, nextIdx uint64) bool {
	// If head has already advanced past nextTicket, the next thread has completed.
	if atomic.LoadUint64(&q.head) > nextTicket {
		return true
	}
	st := atomic.LoadUint32(&q.ring[nextIdx].State)
	if st == 1 {
		if atomic.CompareAndSwapUint32(&q.ring[nextIdx].State, 1, 2) {
			q.ring[nextIdx].sem <- struct{}{}
		}
		return true
	}
	if st == 2 {
		// The next thread already bypassed and is running.
		return true
	}
	return false
}

/**
 * @brief Signal the next ticket holder in the queue if they are ready.
 */
func (q *BankQueue) signalNext(ticket uint64) {
	nextTicket := ticket + 1
	if nextTicket < atomic.LoadUint64(&q.tail) {
		nextIdx := nextTicket & q.mask
		// Wait briefly until the next thread is enqueued (State becomes 1 or 2).
		for {
			if q.checkAndSignal(nextTicket, nextIdx) {
				break
			}
			runtime.Gosched()
		}
	}
}

/**
 * @brief Thread-safely records scheduled access parameters into the trace buffer.
 */
func (ms *MemoryScheduler) logTrace(role, kind string, index, bank int, earliest, start, end uint64) {
	ms.traceMu.Lock()
	defer ms.traceMu.Unlock()
	if len(ms.trace) < ms.traceLimit {
		ms.trace = append(ms.trace, AccessEvent{
			Role:     role,
			Kind:     kind,
			Index:    index,
			Bank:     bank,
			Earliest: earliest,
			Start:    start,
			End:      end,
		})
	}
}

/**
 * @brief Request thread-safe access to a specific memory bank.
 * 
 * Enforces bank-level serialization using active lock-free queues, updates the bank's
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

	q := &ms.bankQueues[bank]

	// 1. Claim a ticket in the ring buffer.
	ticket := atomic.AddUint64(&q.tail, 1) - 1

	// 2. Write the request data into the acquired slot.
	idx := ticket & q.mask
	q.ring[idx].Role = role
	q.ring[idx].Kind = kind
	q.ring[idx].Index = index
	q.ring[idx].Earliest = earliest

	// Mark slot as enqueued/waiting.
	atomic.StoreUint32(&q.ring[idx].State, 1)

	// 3. Wait until head pointer reaches our ticket (our turn).
	q.waitTurn(ticket, idx)

	// 4. Execute the bank allocation cycle update (user-space serialized section).
	currentFree := atomic.LoadUint64(&ms.freeAt[bank])
	startCycle := earliest
	if currentFree > startCycle {
		startCycle = currentFree
	}
	endCycle := startCycle + ms.serviceCycles

	// Update bank availability register clock.
	atomic.StoreUint64(&ms.freeAt[bank], endCycle)

	// Increment requests metric atomically.
	atomic.AddUint64(&ms.requests[bank], 1)

	// Log event to trace buffer.
	ms.logTrace(role, kind, index, bank, earliest, startCycle, endCycle)

	// 5. Release our ticket and signal the next holder if present.
	atomic.StoreUint32(&q.ring[idx].State, 0)
	atomic.StoreUint64(&q.head, ticket+1)

	q.signalNext(ticket)

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
		val := atomic.LoadUint64(&ms.freeAt[i])
		if val > maxCycles {
			maxCycles = val
		}
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

/**
 * @brief Allocates a single memory buffer and binds it to a target physical NUMA node.
 * 
 * @param bank The physical memory bank index.
 * @param node The physical NUMA node ID to bind the memory to.
 * @param bankSize The size in bytes of the buffer to allocate.
 * @return The allocated byte slice buffer, or an error on failure.
 */
func (ms *MemoryScheduler) allocateAndBindBank(bank, node, bankSize int) ([]byte, error) {
	if bank < 0 || bank >= ms.bankCount {
		return nil, errors.New("bank index out of range for scheduler configurations")
	}

	// Allocate memory using mmap with MAP_ANONYMOUS | MAP_PRIVATE | MAP_POPULATE (0x8000)
	// MAP_POPULATE prefaults the page tables, ensuring zero page-fault scheduling latency.
	flags := syscall.MAP_ANONYMOUS | syscall.MAP_PRIVATE | 0x8000
	data, err := syscall.Mmap(-1, 0, bankSize, syscall.PROT_READ|syscall.PROT_WRITE, flags)
	if err != nil {
		return nil, err
	}

	// Set memory bitmask for mbind
	var nodemask uint64
	if node >= 0 && node < 64 {
		nodemask = 1 << uint(node)
	}

	// Invoke Linux SYS_MBIND (syscall 237 on x86_64) to bind memory pages to physical NUMA nodes
	// MPOL_BIND = 1, MPOL_MF_STRICT = 1, MPOL_MF_MOVE = 2
	addr := uintptr(unsafe.Pointer(&data[0]))
	length := uintptr(len(data))
	
	_, _, errno := syscall.Syscall6(
		237, // SYS_MBIND
		addr,
		length,
		uintptr(1), // MPOL_BIND
		uintptr(unsafe.Pointer(&nodemask)),
		uintptr(64),
		uintptr(3), // MPOL_MF_STRICT | MPOL_MF_MOVE
	)

	if errno != 0 && errno != syscall.EINVAL && errno != syscall.EPERM && errno != syscall.ENOSYS {
		_ = syscall.Munmap(data)
		return nil, errno
	}

	return data, nil
}

/**
 * @brief Configures and allocates physical NUMA-bound memory buffers for each bank.
 * 
 * Leverages explicit memory-mapped file/anonymous nodes (mmap with MAP_POPULATE)
 * and the Linux mbind(2) system call to bind virtual memory ranges to host physical sockets.
 * This directly demonstrates physical CADENCE memory role isolation.
 * 
 * @param bankToNode A map linking each bank ID to its target physical NUMA node.
 * @param bankSize The size in bytes of the buffer to allocate per bank.
 * @return An error if allocations fail, or nil on success.
 */
func (ms *MemoryScheduler) EnablePhysicalNUMA(bankToNode map[int]int, bankSize int) error {
	ms.numaMu.Lock()
	defer ms.numaMu.Unlock()

	ms.numaBankMap = make(map[int]int)
	ms.numaBuffers = make(map[int][]byte)
	ms.numaEnabled = true

	for bank, node := range bankToNode {
		data, err := ms.allocateAndBindBank(bank, node, bankSize)
		if err != nil {
			// Rollback previously mapped banks in this call on failure
			_ = ms.Close()
			return err
		}
		ms.numaBankMap[bank] = node
		ms.numaBuffers[bank] = data
	}

	return nil
}

/**
 * @brief Returns the physical NUMA buffer allocated for a specific bank.
 * 
 * @param bank The targeted physical memory bank.
 * @return The byte slice buffer, or nil if not allocated/enabled.
 */
func (ms *MemoryScheduler) GetNUMABuffer(bank int) []byte {
	ms.numaMu.RLock()
	defer ms.numaMu.RUnlock()
	return ms.numaBuffers[bank]
}

/**
 * @brief Returns whether NUMA binding is active.
 */
func (ms *MemoryScheduler) IsNUMAEnabled() bool {
	ms.numaMu.RLock()
	defer ms.numaMu.RUnlock()
	return ms.numaEnabled
}

/**
 * @brief Releases and unmaps all allocated NUMA memory buffers.
 */
func (ms *MemoryScheduler) Close() error {
	ms.numaMu.Lock()
	defer ms.numaMu.Unlock()

	var errs []error
	for bank, data := range ms.numaBuffers {
		if err := syscall.Munmap(data); err != nil {
			errs = append(errs, err)
		}
		delete(ms.numaBuffers, bank)
	}
	ms.numaEnabled = false
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

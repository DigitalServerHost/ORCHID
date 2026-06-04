/**
 * @file main.go
 * @brief Entry point and TCP daemon for Project ORCHID's zero-dependency simulation node.
 * 
 * Provides CLI commands, schedules bank-split workloads sequentially, manages client connections,
 * processes JSON plan payloads, and executes timing diagnostics sweeps.
 * 
 * Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
 * Project Lead & Maintainer: Kevin West (@westkevin12)
 * License: GNU GPLv3
 */

package main

import (
	"ORCHID/scheduler"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	separatorLine      = "======================================================================"
	evidenceCurrent    = "evidence/current"
	evidenceReproduced = "evidence/reproduced"
	errorFormat        = "Error: %v\n"
)

/**
 * @struct SimCase
 * @brief Represents a parallel memory bank mapping configuration test case.
 */
type SimCase struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	BankCount   int            `json:"bank_count"`
	Mapping     map[string]int `json:"mapping"`
}

/**
 * @struct Event
 * @brief Standardized trace event payload format compatible with JSON clients.
 */
type Event struct {
	Role     string `json:"role"`
	Kind     string `json:"kind"`
	Index    int    `json:"index"`
	Bank     int    `json:"bank"`
	Earliest int    `json:"earliest"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
}

/**
 * @struct Result
 * @brief Holds full intermediate simulation results for validation correctness checks.
 */
type Result struct {
	Name           string
	Description    string
	BanksAvailable int
	Mapping        map[string]int
	Cycles         uint64
	Checksum       int64
	Requests       []uint64
	Trace          []Event
	Output         []int32
}

/**
 * @struct Row
 * @brief Holds tabular results to write to CSV and JSON files.
 */
type Row struct {
	Case               string  `json:"case"`
	BanksAvailable     int     `json:"banks_available"`
	Mapping            string  `json:"mapping"`
	Cycles             uint64  `json:"cycles"`
	SpeedupVsSerial    float64 `json:"speedup_vs_serial"`
	RequestsPerBank    string  `json:"requests_per_bank"`
	UtilizationPerBank string  `json:"utilization_per_bank"`
	Checksum           int64   `json:"checksum"`
}

/**
 * @struct SimOutputsConfig
 * @brief Bundles all parameters and output variables to pass into writing routines.
 */
type SimOutputsConfig struct {
	OutDir        string
	N             int
	Scalar        int
	ServiceCycles int
	ComputeCycles int
	Baseline      Result
	Results       []Result
	Rows          []Row
}

/**
 * @struct ConnectionPayload
 * @brief Outer client TCP command structure holding plan configs.
 */
type ConnectionPayload struct {
	Action     string             `json:"action"`
	Simulation *SimulationPayload `json:"simulation"`
	Locality   *LocalityPayload   `json:"locality"`
}

/**
 * @struct SimulationPayload
 * @brief Inner simulation payload containing size parameters and service cycles.
 */
type SimulationPayload struct {
	N             int    `json:"n"`
	Scalar        int    `json:"scalar"`
	ServiceCycles int    `json:"service_cycles"`
	ComputeCycles int    `json:"compute_cycles"`
	TraceLimit    int    `json:"trace_limit"`
	OutDir        string `json:"out_dir"`
}

/**
 * @struct LocalityPayload
 * @brief Inner matrix cache locality timing benchmark parameters.
 */
type LocalityPayload struct {
	Repeats int    `json:"repeats"`
	OutDir  string `json:"out_dir"`
}

/**
 * @brief Generates deterministic inputs B and C vectors for math parity validation.
 * 
 * @param n Length of vectors to allocate.
 * @return B slice and C slice of int32 elements.
 */
func generateInputVectors(n int) ([]int32, []int32) {
	b := make([]int32, n)
	c := make([]int32, n)
	for i := 0; i < n; i++ {
		b[i] = int32(((i*17 + 3) % 97) - 48)
		c[i] = int32(((i*29 + 11) % 89) - 44)
	}
	return b, c
}

/**
 * @brief Runs sequential scheduler loops mimicking DRAM simulation steps.
 * 
 * Runs mathematical Triad formula: A[i] = B[i] + scalar * C[i], scheduling accesses.
 * 
 * @param n Element size length of the buffers.
 * @param scalar Scalar multiplier coefficient.
 * @param bankCount Number of hardware banks.
 * @param mapping Key-value mapping designating which bank gets A, B, and C.
 * @param serviceCycles Processing cycle latency cost per access.
 * @param computeCycles Additional cycle delay for execution.
 * @param traceLimit Upper limit of events recorded in the trace log.
 * @return Total cycles, output slice, correctness checksum, requests per bank, trace array, error.
 */
func runTriadSimulationSequential(
	n int,
	scalar int32,
	bankCount int,
	mapping map[string]int,
	serviceCycles uint64,
	computeCycles uint64,
	traceLimit int,
) (uint64, []int32, int64, []uint64, []Event, error) {
	b, c := generateInputVectors(n)
	a := make([]int32, n)

	ms, err := scheduler.NewMemoryScheduler(bankCount, serviceCycles, traceLimit)
	if err != nil {
		return 0, nil, 0, nil, nil, err
	}

	for i := 0; i < n; i++ {
		bDone, err := ms.Access("B", "READ", i, mapping["B"], 0)
		if err != nil {
			return 0, nil, 0, nil, nil, err
		}
		cDone, err := ms.Access("C", "READ", i, mapping["C"], 0)
		if err != nil {
			return 0, nil, 0, nil, nil, err
		}

		readyCycle := bDone
		if cDone > readyCycle {
			readyCycle = cDone
		}
		computedCycle := readyCycle + computeCycles

		a[i] = b[i] + scalar*c[i]

		_, err = ms.Access("A", "WRITE", i, mapping["A"], computedCycle)
		if err != nil {
			return 0, nil, 0, nil, nil, err
		}
	}

	var checksum int64
	for i, v := range a {
		checksum += int64(i+1) * int64(v)
	}

	requests := make([]uint64, bankCount)
	for i := 0; i < bankCount; i++ {
		requests[i] = ms.GetRequests(i)
	}

	schTrace := ms.GetTrace()
	traceEvents := make([]Event, len(schTrace))
	for i, ev := range schTrace {
		traceEvents[i] = Event{
			Role:     ev.Role,
			Kind:     ev.Kind,
			Index:    ev.Index,
			Bank:     ev.Bank,
			Earliest: int(ev.Earliest),
			Start:    int(ev.Start),
			End:      int(ev.End),
		}
	}

	return ms.TotalCycles(), a, checksum, requests, traceEvents, nil
}

/**
 * @brief Asserts logical result correctness across parallel configuration tests.
 * 
 * @param results Slice of simulation results.
 * @param baseline The reference serial result configuration.
 * @return error if any check value or checksum does not match baseline.
 */
func validateSimResults(results []Result, baseline Result) error {
	for _, res := range results[1:] {
		if len(res.Output) != len(baseline.Output) {
			return fmt.Errorf("mismatch in output size in case %s", res.Name)
		}
		for idx, v := range res.Output {
			if v != baseline.Output[idx] {
				return fmt.Errorf("logical calculation mismatch in case %s at index %d", res.Name, idx)
			}
		}
		if res.Checksum != baseline.Checksum {
			return fmt.Errorf("checksum mismatch in case %s", res.Name)
		}
	}
	return nil
}

/**
 * @brief Formats raw metrics results into rounded rows for table display.
 * 
 * @param results Slice of simulation results.
 * @param baseline Reference serial simulation result.
 * @param serviceCycles Processing cycle cost per memory bank access.
 * @return Slice of Row models formatted for printing/writing.
 */
func buildSimRows(results []Result, baseline Result, serviceCycles int) []Row {
	rows := make([]Row, len(results))
	for i, res := range results {
		speedup := float64(baseline.Cycles) / float64(res.Cycles)
		utilization := make([]float64, len(res.Requests))
		for bankIdx, req := range res.Requests {
			utilization[bankIdx] = float64(req) * float64(serviceCycles) / float64(res.Cycles)
		}

		keys := make([]string, 0, len(res.Mapping))
		for k := range res.Mapping {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sortedMappingParts := make([]string, len(keys))
		for kIdx, k := range keys {
			sortedMappingParts[kIdx] = fmt.Sprintf(`"%s": %d`, k, res.Mapping[k])
		}
		sortedMappingStr := "{" + strings.Join(sortedMappingParts, ", ") + "}"

		reqsJSON, _ := json.Marshal(res.Requests)
		utilsRounded := make([]float64, len(utilization))
		for idx, val := range utilization {
			utilsRounded[idx] = float64(int(val*1000000)) / 1000000.0
		}
		utilsJSON, _ := json.Marshal(utilsRounded)

		rows[i] = Row{
			Case:               res.Name,
			BanksAvailable:     res.BanksAvailable,
			Mapping:            sortedMappingStr,
			Cycles:             res.Cycles,
			SpeedupVsSerial:    float64(int(speedup*1000000)) / 1000000.0,
			RequestsPerBank:    string(reqsJSON),
			UtilizationPerBank: string(utilsJSON),
			Checksum:           res.Checksum,
		}
	}
	return rows
}

/**
 * @brief Encodes the simulation config and results into results.json.
 * 
 * @param cfg Pointer to SimOutputsConfig.
 * @return error if writing or encoding fails.
 */
func writeResultsJSON(cfg *SimOutputsConfig) error {
	resultsJSONMap := map[string]interface{}{
		"workload": map[string]interface{}{
			"formula": "A[i] = B[i] + scalar * C[i]",
			"n":       cfg.N,
			"scalar":  cfg.Scalar,
			"memory_operations_per_element": []string{"READ B", "READ C", "WRITE A"},
			"service_cycles_per_memory_operation": cfg.ServiceCycles,
			"compute_cycles_after_reads": cfg.ComputeCycles,
		},
		"baseline": cfg.Baseline.Name,
		"results":  cfg.Rows,
		"verification": map[string]interface{}{
			"same_logical_output": true,
			"checksum":            cfg.Baseline.Checksum,
		},
	}
	resJSONBytes, err := json.MarshalIndent(resultsJSONMap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cfg.OutDir, "results.json"), append(resJSONBytes, '\n'), 0644)
}

/**
 * @brief Formats and writes tabular simulation results into results.csv.
 * 
 * @param cfg Pointer to SimOutputsConfig.
 * @return error if CSV creation or writes fail.
 */
func writeResultsCSV(cfg *SimOutputsConfig) error {
	csvFile, err := os.Create(filepath.Join(cfg.OutDir, "results.csv"))
	if err != nil {
		return err
	}
	defer csvFile.Close()

	csvWriter := csv.NewWriter(csvFile)
	defer csvWriter.Flush()

	headers := []string{"case", "banks_available", "mapping", "cycles", "speedup_vs_serial", "requests_per_bank", "utilization_per_bank", "checksum"}
	if err := csvWriter.Write(headers); err != nil {
		return err
	}
	for _, row := range cfg.Rows {
		record := []string{
			row.Case,
			strconv.Itoa(row.BanksAvailable),
			row.Mapping,
			strconv.FormatUint(row.Cycles, 10),
			fmt.Sprintf("%.6f", row.SpeedupVsSerial),
			row.RequestsPerBank,
			row.UtilizationPerBank,
			strconv.FormatInt(row.Checksum, 10),
		}
		if err := csvWriter.Write(record); err != nil {
			return err
		}
	}
	return nil
}

/**
 * @brief Outputs trace log events into trace_first_events.csv.
 * 
 * @param cfg Pointer to SimOutputsConfig.
 * @return error if file creation or CSV writes fail.
 */
func writeTraceCSV(cfg *SimOutputsConfig) error {
	traceFile, err := os.Create(filepath.Join(cfg.OutDir, "trace_first_events.csv"))
	if err != nil {
		return err
	}
	defer traceFile.Close()

	traceWriter := csv.NewWriter(traceFile)
	defer traceWriter.Flush()

	traceHeaders := []string{"case", "role", "kind", "index", "bank", "earliest", "start", "end"}
	if err := traceWriter.Write(traceHeaders); err != nil {
		return err
	}
	for _, res := range cfg.Results {
		for _, ev := range res.Trace {
			record := []string{
				res.Name,
				ev.Role,
				ev.Kind,
				strconv.Itoa(ev.Index),
				strconv.Itoa(ev.Bank),
				strconv.Itoa(ev.Earliest),
				strconv.Itoa(ev.Start),
				strconv.Itoa(ev.End),
			}
			if err := traceWriter.Write(record); err != nil {
				return err
			}
		}
	}
	return nil
}

/**
 * @brief Prints and writes a human-readable performance summary to summary.txt.
 * 
 * @param cfg Pointer to SimOutputsConfig.
 * @return error if writing file fails.
 */
func writeSummaryTXT(cfg *SimOutputsConfig) error {
	summaryLines := []string{
		separatorLine,
		"           PROJECT ORCHID: PARALLEL MEMORY MINIMAL POC",
		"           Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵",
		"           Maintainer: Kevin West (@westkevin12)",
		separatorLine,
		fmt.Sprintf("workload: A[i] = B[i] + %d * C[i] | total_elements=%d", cfg.Scalar, cfg.N),
		fmt.Sprintf("latency_cycles: service=%d | compute=%d", cfg.ServiceCycles, cfg.ComputeCycles),
		fmt.Sprintf("verification: identical outputs validated | checksum=%d", cfg.Baseline.Checksum),
		"",
		fmt.Sprintf("%-42s %14s %12s %20s", "case", "cycles", "speedup", "requests/bank"),
	}
	for _, row := range cfg.Rows {
		summaryLines = append(summaryLines,
			fmt.Sprintf("%-42s %14d %11.3fx %20s",
				row.Case, row.Cycles, row.SpeedupVsSerial, row.RequestsPerBank,
			),
		)
	}
	summaryLines = append(summaryLines,
		"",
		"INTERPRETATION",
		"- serial_single_memory is the undifferentiated one-service baseline.",
		"- parallel_two_memory_role_split is the conservative proposed minimum: independent source access is parallelized while output still shares one bank.",
		"- parallel_three_memory_role_split shows the upper reference when input and output roles have independent services.",
		"- parallel_two_memory_conflicted_control proves that merely having multiple banks gives no benefit unless roles/requests are separated correctly.",
		"",
		"BOUNDARY",
		"- This is a deterministic architectural scheduling proof, not a physical DRAM benchmark.",
		"- Physical validation requires an implementation substrate exposing independent memory service paths or hardware/FPGA/simulator support.",
		separatorLine,
		"",
	)
	summaryText := strings.Join(summaryLines, "\n")
	fmt.Print(summaryText)
	return os.WriteFile(filepath.Join(cfg.OutDir, "summary.txt"), []byte(summaryText), 0644)
}

/**
 * @brief Coordinates outputs creation (JSON, CSV, summary) inside output directory.
 * 
 * @param cfg Pointer to SimOutputsConfig containing simulation context.
 * @return error if directories creation or file writes fail.
 */
func writeSimulationOutputs(cfg *SimOutputsConfig) error {
	if cfg.OutDir == "" {
		return nil
	}

	if err := os.MkdirAll(cfg.OutDir, 0755); err != nil {
		return err
	}

	if err := writeResultsJSON(cfg); err != nil {
		return err
	}

	if err := writeResultsCSV(cfg); err != nil {
		return err
	}

	if err := writeTraceCSV(cfg); err != nil {
		return err
	}

	return writeSummaryTXT(cfg)
}

/**
 * @brief Entry point for executing the parallel memory bank scheduler simulation.
 * 
 * Allocates scheduler structures, runs sequential case combinations, verifies parity,
 * formats speedups tables, and records logs.
 * 
 * @param n Element size length of the buffers.
 * @param scalar Scalar multiplier coefficient.
 * @param serviceCycles Processing cycle latency cost per access.
 * @param computeCycles Additional cycle delay for execution.
 * @param traceLimit Upper limit of events recorded in the trace log.
 * @param outDir Target directory to save results files.
 * @return Result summary parameters or an error.
 */
func RunSimulationBenchmark(n int, scalar int, serviceCycles, computeCycles, traceLimit int, outDir string) (map[string]interface{}, error) {
	if n < 1 || computeCycles < 0 {
		return nil, fmt.Errorf("n must be >= 1 and compute_cycles must be >= 0")
	}

	cases := []SimCase{
		{"serial_single_memory", "One logical memory service; B read, C read and A write serialize.", 1, map[string]int{"B": 0, "C": 0, "A": 0}},
		{"parallel_two_memory_role_split", "Two independent services; B read and A write share bank 0, C read uses bank 1.", 2, map[string]int{"B": 0, "C": 1, "A": 0}},
		{"parallel_three_memory_role_split", "Three independent services; B read, C read and A write have distinct banks.", 3, map[string]int{"B": 0, "C": 1, "A": 2}},
		{"parallel_two_memory_conflicted_control", "Two banks exist but all data roles are placed on bank 0; negative control.", 2, map[string]int{"B": 0, "C": 0, "A": 0}},
	}

	results := make([]Result, len(cases))
	for i, c := range cases {
		cycles, out, checksum, requests, trace, err := runTriadSimulationSequential(
			n, int32(scalar), c.BankCount, c.Mapping, uint64(serviceCycles), uint64(computeCycles), traceLimit,
		)
		if err != nil {
			return nil, err
		}
		results[i] = Result{
			Name:           c.Name,
			Description:    c.Description,
			BanksAvailable: c.BankCount,
			Mapping:        c.Mapping,
			Cycles:         cycles,
			Checksum:       checksum,
			Requests:       requests,
			Trace:          trace,
			Output:         out,
		}
	}

	baseline := results[0]
	if err := validateSimResults(results, baseline); err != nil {
		return nil, err
	}

	rows := buildSimRows(results, baseline, serviceCycles)

	cfg := &SimOutputsConfig{
		OutDir:        outDir,
		N:             n,
		Scalar:        scalar,
		ServiceCycles: serviceCycles,
		ComputeCycles: computeCycles,
		Baseline:      baseline,
		Results:       results,
		Rows:          rows,
	}

	if err := writeSimulationOutputs(cfg); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"cycles":   baseline.Cycles,
		"checksum": baseline.Checksum,
	}, nil
}

/**
 * @brief Parses incoming client simulation configuration variables or loads defaults.
 * 
 * @param p Pointer to SimulationPayload.
 * @return parsed n, scalar, serviceCycles, computeCycles, traceLimit, and outDir values.
 */
func parseSimPayload(p *SimulationPayload) (int, int, int, int, int, string) {
	n := 16384
	scalar := 3
	serviceCycles := 100
	computeCycles := 1
	traceLimit := 18
	outDir := evidenceCurrent

	if p != nil {
		if p.N > 0 {
			n = p.N
		}
		if p.Scalar != 0 {
			scalar = p.Scalar
		}
		if p.ServiceCycles > 0 {
			serviceCycles = p.ServiceCycles
		}
		if p.ComputeCycles >= 0 {
			computeCycles = p.ComputeCycles
		}
		if p.TraceLimit > 0 {
			traceLimit = p.TraceLimit
		}
		if p.OutDir != "" {
			outDir = p.OutDir
		}
	}
	return n, scalar, serviceCycles, computeCycles, traceLimit, outDir
}

/**
 * @brief Parses client locality benchmark repeat parameters or loads defaults.
 * 
 * @param p Pointer to LocalityPayload.
 * @return parsed repeats count and output directory path.
 */
func parseLocalityPayload(p *LocalityPayload) (int, string) {
	repeats := 8
	outDir := evidenceReproduced

	if p != nil {
		if p.Repeats > 0 {
			repeats = p.Repeats
		}
		if p.OutDir != "" {
			outDir = p.OutDir
		}
	}
	return repeats, outDir
}

/**
 * @brief Reads, decodes, executes and responds to client TCP requests in daemon mode.
 * 
 * @param conn Net Connection interface.
 */
func handleConnection(conn net.Conn) {
	defer conn.Close()

	var payload ConnectionPayload
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&payload); err != nil {
		writeErrorResponse(conn, fmt.Sprintf("invalid json payload: %v", err))
		return
	}

	response := map[string]interface{}{"status": "success"}
	action := strings.ToLower(payload.Action)
	if action == "" {
		action = "all"
	}

	if action == "simulation" || action == "all" {
		n, scalar, service, compute, trace, out := parseSimPayload(payload.Simulation)
		res, err := RunSimulationBenchmark(n, scalar, service, compute, trace, out)
		if err != nil {
			writeErrorResponse(conn, fmt.Sprintf("simulation failed: %v", err))
			return
		}
		response["simulation"] = res
	}

	if action == "locality" || action == "all" {
		repeats, out := parseLocalityPayload(payload.Locality)
		res, err := RunLocalityBenchmark(repeats, out)
		if err != nil {
			writeErrorResponse(conn, fmt.Sprintf("locality benchmark failed: %v", err))
			return
		}
		response["locality"] = res
	}

	responseBytes, _ := json.Marshal(response)
	conn.Write(append(responseBytes, '\n'))
}

/**
 * @brief Encodes and writes standard error outputs to TCP clients.
 * 
 * @param conn Net Connection interface.
 * @param errMsg Human-readable error message.
 */
func writeErrorResponse(conn net.Conn, errMsg string) {
	res := map[string]interface{}{
		"status": "error",
		"error":  errMsg,
	}
	resBytes, _ := json.Marshal(res)
	conn.Write(append(resBytes, '\n'))
}

/**
 * @brief Loads, reads, and decodes planning instruction documents.
 * 
 * @param planPath File path string location, or '-' to read from stdin.
 * @return ConnectionPayload configuration pointer or an error.
 */
func decodePlanFile(planPath string) (*ConnectionPayload, error) {
	var input io.Reader
	if planPath == "-" {
		input = os.Stdin
	} else {
		file, err := os.Open(planPath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		input = file
	}

	var plan ConnectionPayload
	if err := json.NewDecoder(input).Decode(&plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

/**
 * @brief Executes local simulation and locality workloads defined in JSON plan files.
 * 
 * @param planPath File path containing simulation details.
 * @param customOutDir Custom output directory to write logs (optional).
 */
func executePlan(planPath string, customOutDir string) {
	plan, err := decodePlanFile(planPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load plan: %v\n", err)
		os.Exit(1)
	}

	action := strings.ToLower(plan.Action)
	if action == "" {
		action = "all"
	}

	if action == "simulation" || action == "all" {
		n, scalar, service, compute, trace, outDir := parseSimPayload(plan.Simulation)
		if customOutDir != "" {
			outDir = customOutDir
		}

		fmt.Printf("Executing simulation (N=%d, scalar=%d, service_cycles=%d, compute_cycles=%d, out_dir=%s)...\n",
			n, scalar, service, compute, outDir)
		_, err := RunSimulationBenchmark(n, scalar, service, compute, trace, outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Simulation execution failed: %v\n", err)
			os.Exit(1)
		}
	}

	if action == "locality" || action == "all" {
		repeats, outDir := parseLocalityPayload(plan.Locality)
		if customOutDir != "" {
			outDir = customOutDir
		}

		fmt.Printf("Executing locality benchmark (repeats=%d, out_dir=%s)...\n", repeats, outDir)
		_, err := RunLocalityBenchmark(repeats, outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Locality execution failed: %v\n", err)
			os.Exit(1)
		}
	}
}

/**
 * @brief Listens on TCP sockets to serve incoming planning payload requests.
 * 
 * @param addr Address string to bind to (e.g. ":9000").
 */
func runDaemon(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to listen on %s: %v\n", addr, err)
		os.Exit(1)
	}
	defer ln.Close()
	fmt.Printf("Go daemon listening on TCP %s...\n", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn)
	}
}

/**
 * @brief Command line helper to run the simulation benchmark.
 * 
 * @param outDir Target directory to write outputs.
 */
func runSimulationCLI(outDir string) {
	fmt.Println("Running simulation benchmark...")
	_, err := RunSimulationBenchmark(16384, 3, 100, 1, 18, outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, errorFormat, err)
		os.Exit(1)
	}
}

/**
 * @brief Command line helper to execute locality timing sweeps.
 * 
 * @param outDir Target directory to write outputs.
 */
func runLocalityCLI(outDir string) {
	fmt.Println("Running locality cache benchmark...")
	_, err := RunLocalityBenchmark(8, outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, errorFormat, err)
		os.Exit(1)
	}
}

/**
 * @brief Command line helper to run both simulation and locality sweeps.
 * 
 * @param simDir Target simulation output directory.
 * @param locDir Target locality output directory.
 */
func runAllCLI(simDir, locDir string) {
	fmt.Println("==========================================")
	fmt.Println("Running parallel bank simulation...")
	_, err := RunSimulationBenchmark(16384, 3, 100, 1, 18, simDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, errorFormat, err)
		os.Exit(1)
	}

	fmt.Println("\n==========================================")
	fmt.Println("Running CPU locality matrix benchmark...")
	_, err = RunLocalityBenchmark(8, locDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, errorFormat, err)
		os.Exit(1)
	}
}

/**
 * @brief main executable entry point routing execution flags.
 */
func main() {
	modeFlag := flag.String("mode", "daemon", "Execution mode: daemon, simulation, locality, all")
	planFlag := flag.String("plan", "", "Path to local JSON plan file to execute (or '-' for stdin)")
	addrFlag := flag.String("addr", ":9000", "TCP address to listen on in daemon mode")
	outDirFlag := flag.String("out-dir", "", "Custom output directory for files (overrides defaults)")
	flag.Parse()

	if *planFlag != "" {
		executePlan(*planFlag, *outDirFlag)
		return
	}

	switch *modeFlag {
	case "daemon":
		runDaemon(*addrFlag)
	case "simulation":
		outDir := *outDirFlag
		if outDir == "" {
			outDir = evidenceCurrent
		}
		runSimulationCLI(outDir)
	case "locality":
		outDir := *outDirFlag
		if outDir == "" {
			outDir = evidenceReproduced
		}
		runLocalityCLI(outDir)
	case "all":
		simDir := *outDirFlag
		if simDir == "" {
			simDir = evidenceCurrent
		}
		locDir := *outDirFlag
		if locDir == "" {
			locDir = evidenceReproduced
		}
		runAllCLI(simDir, locDir)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", *modeFlag)
		os.Exit(1)
	}
}

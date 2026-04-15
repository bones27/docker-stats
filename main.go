package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const (
	exporterPort = ":9111"
	metricsPath  = "/metrics"
)

type ContainerMetric struct {
	ContainerID   string
	ContainerName string
	CPUUsage      string
	MemoryUsage   string
	NetworkRx     string
	NetworkTx     string
	BlockRead     string
	BlockWrite    string
	PIDs          string
}

func main() {
	http.HandleFunc("/metrics", metricsHandler)
	fmt.Printf("Starting Docker metrics exporter on %s...\n", exporterPort)
	if err := http.ListenAndServe(exporterPort, nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != metricsPath {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Header().Set("Cache-Control", "no-cache")

	metrics, err := collectDockerStats()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	if err := renderPrometheusMetrics(w, metrics); err != nil {
		http.Error(w, fmt.Sprintf("Error rendering metrics: %v", err), http.StatusInternalServerError)
	}
}

func collectDockerStats() ([]ContainerMetric, error) {
	// Format: ID, Name, CPU%, MemUsage, NetIO, BlockIO, PIDs
	cmd := exec.Command("docker", "stats", "--no-stream", "--format",
		`{{.ID}}\t{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDs}}`)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker stats failed: %v", err)
	}

	return parseDockerStatsOutput(output), nil
}

func parseDockerStatsOutput(output []byte) []ContainerMetric {
	var metrics []ContainerMetric
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}

		cpu := fields[2]
		mem := fields[3]
		netIO := fields[4]
		blockIO := fields[5]
		pids := fields[6]

		// Split network I/O (rx / tx)
		networkRx, networkTx := parseIO(netIO)

		// Split block I/O (read / write)
		blockRead, blockWrite := parseIO(blockIO)

		metric := ContainerMetric{
			ContainerID:   fields[0],
			ContainerName: fields[1],
			CPUUsage:      cpu,
			MemoryUsage:   mem,
			NetworkRx:     networkRx,
			NetworkTx:     networkTx,
			BlockRead:     blockRead,
			BlockWrite:    blockWrite,
			PIDs:          pids,
		}

		metrics = append(metrics, metric)
	}

	return metrics
}

func parseIO(ioStr string) (string, string) {
	parts := strings.Split(ioStr, "/")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(ioStr), "0"
}

func renderPrometheusMetrics(w http.ResponseWriter, metrics []ContainerMetric) error {
	// CPU metrics
	fmt.Fprintln(w, "# HELP container_cpu_usage_percent CPU usage percent")
	fmt.Fprintln(w, "# TYPE container_cpu_usage_percent gauge")
	for _, m := range metrics {
		fmt.Fprintf(w, "container_cpu_usage_percent{container_id=\"%s\",container_name=\"%s\"} %s\n",
			m.ContainerID, m.ContainerName, strings.Trim(m.CPUUsage, "%"))
	}

	// Memory metrics
	fmt.Fprintln(w, "\n# HELP container_memory_usage_bytes Memory usage bytes")
	fmt.Fprintln(w, "# TYPE container_memory_usage_bytes gauge")
	for _, m := range metrics {
		fmt.Fprintf(w, "container_memory_usage_bytes{container_id=\"%s\",container_name=\"%s\"} %s\n",
			m.ContainerID, m.ContainerName, cleanMetricValue(m.MemoryUsage))
	}

	// Network RX
	fmt.Fprintln(w, "\n# HELP container_network_rx_bytes Network receive bytes")
	fmt.Fprintln(w, "# TYPE container_network_rx_bytes gauge")
	for _, m := range metrics {
		fmt.Fprintf(w, "container_network_rx_bytes{container_id=\"%s\",container_name=\"%s\"} %s\n",
			m.ContainerID, m.ContainerName, cleanMetricValue(m.NetworkRx))
	}

	// Network TX
	fmt.Fprintln(w, "\n# HELP container_network_tx_bytes Network transmit bytes")
	fmt.Fprintln(w, "# TYPE container_network_tx_bytes gauge")
	for _, m := range metrics {
		fmt.Fprintf(w, "container_network_tx_bytes{container_id=\"%s\",container_name=\"%s\"} %s\n",
			m.ContainerID, m.ContainerName, cleanMetricValue(m.NetworkTx))
	}

	// Block I/O Read
	fmt.Fprintln(w, "\n# HELP container_block_io_read_bytes Block I/O read bytes")
	fmt.Fprintln(w, "# TYPE container_block_io_read_bytes gauge")
	for _, m := range metrics {
		fmt.Fprintf(w, "container_block_io_read_bytes{container_id=\"%s\",container_name=\"%s\"} %s\n",
			m.ContainerID, m.ContainerName, cleanMetricValue(m.BlockRead))
	}

	// Block I/O Write
	fmt.Fprintln(w, "\n# HELP container_block_io_write_bytes Block I/O write bytes")
	fmt.Fprintln(w, "# TYPE container_block_io_write_bytes gauge")
	for _, m := range metrics {
		fmt.Fprintf(w, "container_block_io_write_bytes{container_id=\"%s\",container_name=\"%s\"} %s\n",
			m.ContainerID, m.ContainerName, cleanMetricValue(m.BlockWrite))
	}

	// PIDs
	fmt.Fprintln(w, "\n# HELP container_pids Number of processes")
	fmt.Fprintln(w, "# TYPE container_pids gauge")
	for _, m := range metrics {
		fmt.Fprintf(w, "container_pids{container_id=\"%s\",container_name=\"%s\"} %s\n",
			m.ContainerID, m.ContainerName, m.PIDs)
	}

	return nil
}

// Convert string value with units to bytes
func cleanMetricValue(value string) string {
	if value == "" {
		return "0"
	}

	// Remove % from CPU
	value = regexp.MustCompile(`\s*%\s*`).ReplaceAllString(value, "")

	// Parse memory units and convert to bytes
	bytesValue, err := parseAndConvertToBytes(value)
	if err != nil {
		return "0"
	}

	return fmt.Sprintf("%d", bytesValue)
}

// Parse and convert memory units to bytes
func parseAndConvertToBytes(value string) (int64, error) {
	// Regex to extract number and unit
	re := regexp.MustCompile(`(\d+\.?\d*)\s*([A-Za-z]*)`)
	matches := re.FindStringSubmatch(value)

	if len(matches) < 3 {
		return 0, fmt.Errorf("invalid format: %s", value)
	}

	// Extract number
	numStr := matches[1]
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, err
	}

	// Extract unit and convert
	unit := strings.ToUpper(matches[2])
	var multiplier float64

	switch unit {
	case "B", "BYTE", "BYTES":
		multiplier = 1
	case "K", "KB", "KIB":
		if strings.Contains(strings.ToLower(unit), "i") {
			multiplier = 1024 // KiB
		} else {
			multiplier = 1000 // KB
		}
	case "M", "MB", "MIB":
		if strings.Contains(strings.ToLower(unit), "i") {
			multiplier = 1024 * 1024 // MiB
		} else {
			multiplier = 1000 * 1000 // MB
		}
	case "G", "GB", "GIB":
		if strings.Contains(strings.ToLower(unit), "i") {
			multiplier = 1024 * 1024 * 1024 // GiB
		} else {
			multiplier = 1000 * 1000 * 1000 // GB
		}
	case "T", "TB", "TIB":
		if strings.Contains(strings.ToLower(unit), "i") {
			multiplier = 1024 * 1024 * 1024 * 1024 // TiB
		} else {
			multiplier = 1000 * 1000 * 1000 * 1000 // TB
		}
	default:
		multiplier = 1 // Assume bytes
	}

	return int64(num * multiplier), nil
}

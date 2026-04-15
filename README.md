# Docker Stats Prometheus Exporter

A lightweight, high-performance Go service that scrapes real-time container statistics using the `docker stats` command and exposes them in a Prometheus-compatible format.

## 🚀 Features

- **Automatic Unit Conversion**: Automatically converts human-readable Docker output (e.g., `MiB`, `GiB`, `kB`, `MB`) into raw **Bytes**, ensuring compatibility with Prometheus mathematical functions.
- **Clean Metric Naming**: Uses the `container_` prefix for all exported metrics for better organization.
- **Comprehensive Metrics**:
    - 🧠 **CPU**: Percentage usage (stripped of `%` symbol).
    - 💾 **Memory**: Current usage in bytes.
    - 🌐 **Network**: Receive (RX) and Transmit (TX) bytes.
    - 💿 **Block I/O**: Read and Write bytes.
    - 🔢 **PIDs**: Current process count.
- **Zero Dependencies**: Compiled into a single static binary.
- **Low Overhead**: Uses `--no-stream` to prevent long-running process accumulation.

## 📋 Prerequisites

- [Go](https://golang.org/doc/install) (if building from source)
- [Docker](https://docs.docker.com/get-docker/) installed and running
- Access to `/var/run/docker.sock` (required to read container stats)

## 🛠 Installation & Building

### From Source
1. Clone this repository or copy the `main.go` file.
2. Build the binary:
   ```bash
   go build -o docker-exporter main.go
   ```

### Running via Docker
To run the exporter as a container, ensure you mount the Docker socket:

```bash
docker run -d \
  --name docker-metrics-exporter \
  -p 9111:9111 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --restart unless-stopped \
  [your-image-name]
```

## 📊 Metrics Reference

All size-based metrics are exported in **Bytes**.

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `container_cpu_usage_percent` | Gauge | CPU usage percentage (0-100) |
| `container_memory_usage_bytes` | Gauge | Memory usage in bytes |
| `container_network_rx_bytes` | Gauge | Network receive traffic in bytes |
| `container_network_tx_bytes` | Gauge | Network transmit traffic in bytes |
| `container_block_io_read_bytes` | Gauge | Block I/O read operations in bytes |
| `container_block_io_write_bytes` | Gauge | Block I/O write operations in bytes |
| `container_pids` | Gauge | Number of processes running in the container |

**Labels provided for every metric:**
- `container_id`: The short Docker Container ID.
- `container_name`: The name of the container.

## 📈 Prometheus Configuration

Add the following job to your `prometheus.yml` file to begin scraping:

```yaml
scrape_configs:
  - job_name: 'docker-stats'
    static_configs:
      - targets: ['<YOUR_HOST_IP>:9111']
    scrape_interval: 60s
```

## 🔍 Verification

Once the service is running, you can verify the output manually using `curl`:

```bash
curl http://localhost:9111/metrics
```

**Example Output:**
```text
# HELP container_cpu_usage_percent CPU usage percent
# TYPE container_cpu_usage_percent gauge
container_cpu_usage_percent{container_id="3e59ae52e9c5",container_name="caddy-caddy-1"} 0.00

# HELP container_memory_usage_bytes Memory usage bytes
# TYPE container_memory_usage_bytes gauge
container_memory_usage_bytes{container_id="3e59ae52e9c5",container_name="caddy-caddy-1"} 70782464

# HELP container_network_rx_bytes Network receive bytes
# TYPE container_network_rx_bytes gauge
container_network_rx_bytes{container_id="3e59ae52e9c5",container_name="caddy-caddy-1"} 4180
```

## ⚠️ Important Notes

- **Permissions**: If running the binary directly on Linux, ensure the user running the service has permissions to access `/var/run/docker.sock` (usually by being in the `docker` group).
- **Unit Logic**: The converter handles both **Binary** (KiB, MiB, GiB using 1024) and **Decimal** (KB, MB, GB using 1000) units to ensure accurate data regardless of how Docker chooses to display it.
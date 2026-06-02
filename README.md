# 🌸 Project ORCHID

### Operation-Role Coordination & Hedging Interface Daemon

[![License: GPLv3](https://img.shields.io/badge/License-GPLv3-blue.svg)](#)
[![Tech: Go](https://img.shields.io/badge/Tech-Go_1.20%2B-00ADD8.svg)](#)
[![Tech: Python](https://img.shields.io/badge/Tech-Python_3.10%2B-blue.svg)](#)
[![Tech: C](https://img.shields.io/badge/Tech-C11-blue.svg)](#)
[![Tech: Assembly](https://img.shields.io/badge/Tech-x86--64_Assembly-orange.svg)](#)
[![GitHub Release](https://img.shields.io/github/v/release/DigitalServerHost/ORCHID?include_prereleases&sort=semver&color=FF69B4)](https://github.com/DigitalServerHost/ORCHID/releases/latest)
[![GHCR Container](https://img.shields.io/badge/GHCR-Package_Registry-blueviolet.svg?logo=docker&logoColor=white)](https://github.com/DigitalServerHost/ORCHID/pkgs/container/orchid)
[![Downloads](https://img.shields.io/github/downloads/DigitalServerHost/ORCHID/total?color=blue)](https://github.com/DigitalServerHost/ORCHID/releases)

Project **ORCHID** is the low-level micro-architectural execution core of the RAMNET protocol. It provides the mathematical proof-of-concepts, dynamic assembly generators, and scheduling blueprints required to bypass the digital memory wall and run bare-metal computation at zero-stall efficiency.

> [!NOTE]  
> **Standalone Architecture:** While ORCHID was intentionally designed and optimized as the foundational low-level execution engine for the decentralized compute mesh of the **RAMNET Protocol**, it is engineered as a completely decoupled, standalone layer. Its core scheduler, cache-line saturation modules, and micro-kernel code emitters can be utilized independently across the industry for high-concurrency systems and bare-metal orchestration.

---

## 🏛️ Project Roles

- **Concept originator:** **Teppei Oohira / 大平鉄兵 (@gatchimuchio)**
  - _Designed the initial CPU cache line locality proofs, assembly code generation matrices, and parallel multi-memory bank role-scheduling modules._
- **Core Architecture & Maintainer:** **Kevin West / @westkevin12**
  - _Directs overall system integration, maintains the execution environments, and manages the architectural roadmap for deployment within the RAMNET distributed compute mesh._

### 📜 Historical Foundations

The absolute base foundation, research primitives, and original codebase layout can be found preserved on the legacy archive branch:
👉 **[View the Baseline Concept Code (`tree/gatchimuchio-original`)](https://github.com/DigitalServerHost/ORCHID/tree/gatchimuchio-original)**

---

## 📊 Reproduced Locality Performance

Under identical, mathematically verified logical execution constraints (512x512 matrix size, double-triplicate verification, and total 64 MiB L1-L3 cache flushes between timing runs), the locality-aligned (I-K-J) memory mapping sweeps demonstrate exceptionally high performance improvements. Badges below are dynamically parsed from current timing sweeps:

| Metric              | Speedup                                                                                                                                                                                                                                                        |
| :------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Minimum Speedup** | ![Speedup Min](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fraw.githubusercontent.com%2Fwestkevin12%2FRAMNET%2Ffeat%2FSIMD_Vector%2FORCHID%2Fevidence%2Freproduced%2Fspeedups.json&query=%24.min&label=Speedup%20Min&color=blue)                |
| **Median Speedup**  | ![Speedup Median](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fraw.githubusercontent.com%2Fwestkevin12%2FRAMNET%2Ffeat%2FSIMD_Vector%2FORCHID%2Fevidence%2Freproduced%2Fspeedups.json&query=%24.median&label=Speedup%20Median&color=blueviolet) |
| **Maximum Speedup** | ![Speedup Max](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fraw.githubusercontent.com%2Fwestkevin12%2FRAMNET%2Ffeat%2FSIMD_Vector%2FORCHID%2Fevidence%2Freproduced%2Fspeedups.json&query=%24.max&label=Speedup%20Max&color=brightgreen)         |
| **Mean Speedup**    | ![Speedup Mean](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fraw.githubusercontent.com%2Fwestkevin12%2FRAMNET%2Ffeat%2FSIMD_Vector%2FORCHID%2Fevidence%2Freproduced%2Fspeedups.json&query=%24.mean&label=Speedup%20Mean&color=orange)           |

---

## 🏛️ Centralized Architectural Design & Blueprint

To ensure professional documentation standards and maintain a clean, readable quickstart guide, Project ORCHID's deep technical designs, mathematical formulations, and nested folder blueprints have been centralized:

👉 **[Read the Master Architecture Blueprint (`docs/ARCHITECTURE.md`)](docs/ARCHITECTURE.md)**

### What You Will Find Inside the Architecture Blueprint:

- **The Go/Python Hybrid Split:** Understanding how the Python client SDK prepares/decomposes graphs and the native Go daemon schedules execution payloads.
- **Mathematical Formulations:** Technical detail on why loop striding swap-layouts (`I-K-J` vs `I-J-K`) saturate CPU caches, alongside the CADENCE parallel banking role-routing models.
- **Repository File Blueprint:** A detailed responsibility description of every single directory, file, and utility script.
- **Continuous Quality Orchestration:** How Docker Compose, Astral `uv` virtual environments, and SonarQube static analyzer suites interact to verify system integrity.

## 🚀 Universal Command Dashboard: The `Makefile`

Project ORCHID features a top-level [**`Makefile`**](Makefile) acting as the central developer control panel. Instead of navigating subfolders and invoking standalone shell scripts, use these standardized commands:

### 1. Bootstrapping Your System (`make setup`)

Automatically provisions the sandboxed Python 3.10 virtual environment, installs the modular `orchid` Python SDK in editable development mode (`uv pip install -e .`), and runs first-run diagnostic verification checks.

```bash
make setup
```

### 2. Native Multi-Language Sweeps (`make test`)

Executes concurrent Go scheduling unit tests, compiles x86-64 assembly locality cache-line saturation benchmarks, and generates parallel banked STREAM-Triad simulation logs.

```bash
make test
```

### 3. Native Daemon Binary Build (`make build`)

Compiles the high-concurrency Go node scheduler daemon into a standalone, bare-metal native binary at `build/orchid-daemon`.

```bash
make build
```

### 4. Zero-Dependency Containerized Sandbox (`make docker-up`)

Builds, spins up, and executes the entire multi-language ORCHID stack in isolated Docker containers, volume-syncing generated benchmarks back to your local host filesystem.

```bash
make docker-up
```

> [!TIP]
> To run the container network in the background (detached mode), use the `-d` flag:
>
> ```bash
> docker compose up -d --build
> ```
>
> You can follow and stream the logs live by executing:
>
> ```bash
> docker compose logs -f
> ```
>
> Or isolate output to a single service (e.g., the cache locality timings):
>
> ```bash
> docker compose logs -f orchid-locality-benchmark
> ```

### 5. Cleaning Workspace Artifacts (`make clean`)

Instantly purges temporary compile targets (`locality/build/`), telemetry traces (`evidence/`), and Python `__pycache__` artifacts.

```bash
make clean
```

## 📦 Dual-Container Architecture

Project ORCHID publishes two distinct, optimized container flavors to the GitHub Container Registry under a single repository space to meet different operational environments:

### 1. Hardened Production Image (`ghcr.io/digitalserverhost/orchid:latest`)

- **Target Stage:** `release-hardened`
- **Compiled Control Plane:** Compiles the `orchid` Python SDK plane into optimized C/C++ extension modules (`.so`) using **Nuitka**.
- **Source Protection:** Purges raw `.py` scripts inside the package namespace to prevent code extraction.
- **High Performance:** Execution loops for micro-kernels and role-scheduling simulators execute at native C speeds.

### 2. Developer Sandbox Image (`ghcr.io/digitalserverhost/orchid:dev`)

- **Target Stage:** `developer`
- **Raw Python SDK:** Features standard, raw Python code inside the package structure.
- **Developer Toolset:** Includes the full Astral `uv` package manager, volume mount options, and system diagnostic sweeps for active engineering.

---

## 🛠️ Integrated Developer Onboarding & Tooling

To ensure a deterministic, high-performance workspace out-of-the-box, Project ORCHID coordinates the following enterprise-grade tooling layers:

### 1. Packaged Python SDK (`orchid/`)

The Python control plane is structured as a modular, distributable Python package using the `hatchling` build-backend. You can build it into wheels (`uv build`) or import modules programmatically:

- `from orchid.assembler import Spec, emit_locality` - x86-64 micro-kernel code emitter.
- `from orchid.simulator import BankedMemoryScheduler` - Stream-Triad memory bank role simulator.
- `from orchid.aggregator import parse_and_summarize` - Statistical result parser.

### 2. Astral `uv` Python Version Management

We use [**Astral `uv`**](https://astral.sh/uv/) for lightning-fast Python version lock-in and virtual environment sandboxing. It guarantees that the correct minimum Python version (`>= 3.10`) is isolated and executed in `.venv/` without polluting your global system.

### 3. Integrated IDE Workspace Setup

- **VS Code Settings:** Opening this folder in VS Code automatically reads the pre-configured [**`.vscode/settings.json`**](.vscode/settings.json), instantly targeting the `.venv/bin/python` interpreter.
- **Multi-Language Quality Gates (SonarQube):** We use **SonarQube** for enterprise-grade quality gates and security audits across all of ORCHID's modules (Python, Go, C, and Bash). Standard configuration properties are loaded from [**`sonar-project.properties`**](sonar-project.properties). Developers are highly encouraged to install the **SonarLint** extension in their IDE for live real-time analysis logs.

---

_"Intelligence requires every available joule."_

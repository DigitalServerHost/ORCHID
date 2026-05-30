# 🌸 Project ORCHID

### Operation-Role Coordination & Hedging Interface Daemon

[![License: GPLv3](https://img.shields.io/badge/License-GPLv3-blue.svg)](#)
[![Tech: Python](https://img.shields.io/badge/Tech-Python_3.10%2B-blue.svg)](#)
[![Tech: C](https://img.shields.io/badge/Tech-C11-blue.svg)](#)
[![Tech: Assembly](https://img.shields.io/badge/Tech-x86--64_Assembly-orange.svg)](#)

Project **ORCHID** is the low-level micro-architectural execution core of the RAMNET protocol. It provides the mathematical proof-of-concepts, dynamic assembly generators, and scheduling blueprints required to bypass the digital memory wall and run bare-metal computation at zero-stall efficiency.

---

## 🏛️ Project Roles & Leadership

*   **Originator:** **Teppei Oohira / 大平鉄兵 (@gatchimuchio)**
    *   *Designed the initial CPU cache line locality proofs, assembly code generation matrices, and parallel multi-memory bank role-scheduling modules.*
*   **Project Lead & Maintainer:** **Kevin West / @westkevin12**
    *   *Directs overall system integration, maintains the execution environments, and manages the architectural roadmap for deployment within the RAMNET distributed compute mesh.*

---

## 🏛️ Centralized Architectural Design & Blueprint

To ensure professional documentation standards and maintain a clean, readable quickstart guide, Project ORCHID's deep technical designs, mathematical formulations, and nested folder blueprints have been centralized:

👉 **[Read the Master Architecture Blueprint (`docs/ARCHITECTURE.md`)](docs/ARCHITECTURE.md)**

### What You Will Find Inside the Architecture Blueprint:
*   **The Go/Python Hybrid Split:** Understanding how the Python client SDK prepares/decomposes graphs and the native Go daemon schedules execution payloads.
*   **Mathematical Formulations:** Technical detail on why loop striding swap-layouts (`I-K-J` vs `I-J-K`) saturate CPU caches, alongside the CADENCE parallel banking role-routing models.
*   **Repository File Blueprint:** A detailed responsibility description of every single directory, file, and utility script.
*   **Continuous Quality Orchestration:** How Docker Compose, Astral `uv` virtual environments, and SonarQube static analyzer suites interact to verify system integrity.

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
> ```bash
> docker compose up -d --build
> ```
> You can follow and stream the logs live by executing:
> ```bash
> docker compose logs -f
> ```
> Or isolate output to a single service (e.g., the cache locality timings):
> ```bash
> docker compose logs -f orchid-locality-benchmark
> ```

### 5. Cleaning Workspace Artifacts (`make clean`)
Instantly purges temporary compile targets (`locality/build/`), telemetry traces (`evidence/`), and Python `__pycache__` artifacts.
```bash
make clean
```

---

## 🛠️ Integrated Developer Onboarding & Tooling

To ensure a deterministic, high-performance workspace out-of-the-box, Project ORCHID coordinates the following enterprise-grade tooling layers:

### 1. Packaged Python SDK (`orchid/`)
The Python control plane is structured as a modular, distributable Python package using the `hatchling` build-backend. You can build it into wheels (`uv build`) or import modules programmatically:
*   `from orchid.assembler import Spec, emit_locality` - x86-64 micro-kernel code emitter.
*   `from orchid.simulator import BankedMemoryScheduler` - Stream-Triad memory bank role simulator.
*   `from orchid.aggregator import parse_and_summarize` - Statistical result parser.

### 2. Astral `uv` Python Version Management
We use [**Astral `uv`**](https://astral.sh/uv/) for lightning-fast Python version lock-in and virtual environment sandboxing. It guarantees that the correct minimum Python version (`>= 3.10`) is isolated and executed in `.venv/` without polluting your global system.

### 3. Integrated IDE Workspace Setup
*   **VS Code Settings:** Opening this folder in VS Code automatically reads the pre-configured [**`.vscode/settings.json`**](.vscode/settings.json), instantly targeting the `.venv/bin/python` interpreter.
*   **Multi-Language Quality Gates (SonarQube):** We use **SonarQube** for enterprise-grade quality gates and security audits across all of ORCHID's modules (Python, Go, C, and Bash). Standard configuration properties are loaded from [**`sonar-project.properties`**](sonar-project.properties). Developers are highly encouraged to install the **SonarLint** extension in their IDE for live real-time analysis logs.

---
_"Intelligence requires every available joule."_

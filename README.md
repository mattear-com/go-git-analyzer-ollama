<p align="center">
  <h1 align="center">🔍 CodeLens AI</h1>
  <p align="center">
    <strong>AI-powered Git repository analyzer with RAG capabilities</strong>
  </p>
  <p align="center">
    <a href="README.es.md">🇪🇸 Español</a> •
    <a href="LICENSE">MIT License</a>
  </p>
</p>

---

CodeLens AI is an open-source platform that connects your Git repositories to a local [Ollama](https://ollama.com) instance, enabling **AI-driven code analysis**, **semantic search (RAG)** over your codebase, and **automated quality reports** — all without sending code to third-party clouds.

## ✨ Features

| Feature | Description |
|---|---|
| **Multi-strategy Analysis** | Architecture, code quality, functionality, and DevOps — each evaluated independently by AI |
| **RAG (Retrieval-Augmented Generation)** | Ask natural-language questions about your code; answers are grounded in your actual source files via pgvector embeddings |
| **Streaming Responses** | Real-time, token-by-token AI responses via Server-Sent Events |
| **MCP Server** | Expose analysis and RAG capabilities to external AI agents through the Model Context Protocol |
| **OAuth2 Authentication** | Sign in with Google or GitHub; JWT-protected API |
| **Audit Logging** | Every API request is recorded for compliance and traceability |
| **Snapshot-based Versioning** | Each analysis is tied to a specific commit, enabling historical comparison |

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────┐
│                  Next.js Frontend                    │
│          (React 19 · TypeScript · App Router)        │
└───────────────────────┬─────────────────────────────┘
                        │ REST API
┌───────────────────────▼─────────────────────────────┐
│                Go Backend (Fiber v3)                 │
│                                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │  Auth     │  │  Repos   │  │ Analysis │          │
│  │  Handler  │  │  Handler │  │  Handler │          │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘          │
│       │              │              │                │
│  ┌────▼──────────────▼──────────────▼────┐          │
│  │           Service Layer               │          │
│  │  AuthSvc · RepoSvc · AnalysisSvc · RAG│          │
│  └────────────────┬──────────────────────┘          │
│                   │                                  │
│  ┌────────────────▼──────────────────────┐          │
│  │            Port / Adapter Layer       │          │
│  │  AI (Ollama) · VCS (Git) · Store (PG) │          │
│  └───────────────────────────────────────┘          │
└──────────┬───────────────────┬──────────────────────┘
           │                   │
     ┌─────▼─────┐      ┌─────▼──────┐
     │ PostgreSQL │      │   Ollama   │
     │ + pgvector │      │  (Local)   │
     └───────────┘      └────────────┘
```

The backend follows a **hexagonal (ports & adapters) architecture**, making it straightforward to swap AI providers, VCS backends, or databases.

### Analysis Strategies

The analysis engine uses the **Strategy Pattern** with four independent evaluators:

- **Architecture** — project structure, separation of concerns, dependency management
- **Code Quality** — readability, maintainability, test coverage, best practices
- **Functionality** — feature completeness, API design, error handling
- **DevOps** — CI/CD, containerization, monitoring, deployment readiness

## 🛠️ Tech Stack

| Layer | Technology |
|---|---|
| **Backend** | Go 1.25 · [Fiber v3](https://gofiber.io) |
| **Frontend** | [Next.js 16](https://nextjs.org) · React 19 · TypeScript |
| **Database** | PostgreSQL 16 · [pgvector](https://github.com/pgvector/pgvector) |
| **AI** | [Ollama](https://ollama.com) (embeddings + chat) |
| **Auth** | OAuth2 (Google, GitHub) · JWT |
| **Infra** | Docker Compose |

## 🚀 Getting Started

### Prerequisites

- **Go** ≥ 1.25
- **Node.js** ≥ 18
- **Docker** & Docker Compose
- **Ollama** running locally with the desired models pulled

```bash
# Pull the default models
ollama pull bge-m3      # embeddings
ollama pull qwen3       # chat
```

### 1. Clone the repository

```bash
git clone https://github.com/arturoeanton/go-git-analyzer-ollama.git
cd go-git-analyzer-ollama
```

### 2. Configure environment

```bash
cp .env.example .env
# Edit .env with your OAuth credentials and preferences
```

### 3. Start the database

```bash
docker compose up -d
```

### 4. Run the backend

```bash
go run ./cmd/server
```

The API will be available at `http://localhost:3001`.

### 5. Run the frontend

```bash
cd web
npm install
npm run dev
```

The UI will be available at `http://localhost:3000`.

## 📡 API Overview

All endpoints (except auth and health) require a valid JWT in the `Authorization` header.

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/health` | Health check |
| `GET/POST` | `/api/v1/auth/{provider}/*` | OAuth2 authentication flow |
| `GET/POST` | `/api/v1/repos` | List / add repositories |
| `POST` | `/api/v1/analysis/run` | Trigger a full analysis |
| `GET` | `/api/v1/reports` | List analysis reports |
| `POST` | `/api/v1/rag/query` | Ask a question about a repository (RAG) |
| `POST` | `/api/v1/rag/stream` | Streaming RAG query (SSE) |
| `GET` | `/api/v1/audit` | Retrieve audit logs |

## 🤖 MCP Integration

When `MCP_ENABLED=true`, a separate [Model Context Protocol](https://modelcontextprotocol.io) server starts on `MCP_PORT` (default `3002`), exposing the RAG and analysis capabilities to external AI agents and IDEs.

## 📁 Project Structure

```
.
├── cmd/server/          # Application entrypoint
├── internal/
│   ├── adapter/         # Infrastructure implementations
│   │   ├── ai/          #   Ollama provider
│   │   ├── analysis/    #   Strategy implementations
│   │   ├── auth/        #   Google & GitHub OAuth
│   │   ├── store/       #   PostgreSQL + pgvector
│   │   └── vcs/         #   Git operations
│   ├── domain/          # Core domain models
│   ├── handler/         # HTTP handlers (Fiber)
│   ├── mcp/             # MCP server
│   ├── middleware/       # JWT auth & audit middleware
│   ├── port/            # Interfaces (ports)
│   └── service/         # Business logic
├── migrations/          # SQL migration scripts
├── pkg/config/          # Configuration loader
├── web/                 # Next.js frontend
│   └── src/app/
│       ├── dashboard/   # Main dashboard, repos, reports, audit
│       ├── login/       # Login page
│       └── auth/        # OAuth callback handler
└── docker-compose.yml   # PostgreSQL + pgvector setup
```

## 🗄️ Database Schema

The schema is managed via SQL migrations in `migrations/`:

- **users** — OAuth2 user profiles
- **repos** — registered Git repositories
- **snapshots** — immutable commit-level snapshots
- **embeddings** — pgvector code chunk embeddings
- **analysis_results** — per-strategy analysis output (with scores and suggestions)
- **audit_logs** — full request audit trail

## 📄 License

This project is licensed under the [MIT License](LICENSE).

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/arturoeanton">Arturo Elias</a>
</p>

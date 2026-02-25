<p align="center">
  <h1 align="center">🔍 CodeLens AI</h1>
  <p align="center">
    <strong>Analizador de repositorios Git potenciado por IA con capacidades RAG</strong>
  </p>
  <p align="center">
    <a href="README.md">🇬🇧 English</a> •
    <a href="LICENSE">Licencia MIT</a>
  </p>
</p>

---

CodeLens AI es una plataforma open-source que conecta tus repositorios Git con una instancia local de [Ollama](https://ollama.com), habilitando **análisis de código con IA**, **búsqueda semántica (RAG)** sobre tu código fuente y **reportes de calidad automatizados** — todo sin enviar código a nubes de terceros.

## ✨ Características

| Característica | Descripción |
|---|---|
| **Análisis Multi-estrategia** | Arquitectura, calidad de código, funcionalidad y DevOps — cada uno evaluado de forma independiente por IA |
| **RAG (Generación Aumentada por Recuperación)** | Haz preguntas en lenguaje natural sobre tu código; las respuestas se basan en tus archivos fuente reales mediante embeddings de pgvector |
| **Respuestas en Streaming** | Respuestas de IA en tiempo real, token por token, vía Server-Sent Events |
| **Servidor MCP** | Expone las capacidades de análisis y RAG a agentes de IA externos a través del Model Context Protocol |
| **Autenticación OAuth2** | Inicia sesión con Google o GitHub; API protegida con JWT |
| **Registro de Auditoría** | Cada petición a la API queda registrada para cumplimiento y trazabilidad |
| **Versionado por Snapshots** | Cada análisis se vincula a un commit específico, permitiendo comparaciones históricas |

## 🏗️ Arquitectura

```
┌─────────────────────────────────────────────────────┐
│                  Frontend Next.js                    │
│          (React 19 · TypeScript · App Router)        │
└───────────────────────┬─────────────────────────────┘
                        │ API REST
┌───────────────────────▼─────────────────────────────┐
│               Backend Go (Fiber v3)                  │
│                                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │   Auth    │  │  Repos   │  │ Análisis │          │
│  │  Handler  │  │  Handler │  │  Handler │          │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘          │
│       │              │              │                │
│  ┌────▼──────────────▼──────────────▼────┐          │
│  │          Capa de Servicios            │          │
│  │ AuthSvc · RepoSvc · AnalysisSvc · RAG │          │
│  └────────────────┬──────────────────────┘          │
│                   │                                  │
│  ┌────────────────▼──────────────────────┐          │
│  │       Capa de Puertos / Adaptadores   │          │
│  │  IA (Ollama) · VCS (Git) · Store (PG) │          │
│  └───────────────────────────────────────┘          │
└──────────┬───────────────────┬──────────────────────┘
           │                   │
     ┌─────▼─────┐      ┌─────▼──────┐
     │ PostgreSQL │      │   Ollama   │
     │ + pgvector │      │  (Local)   │
     └───────────┘      └────────────┘
```

El backend sigue una **arquitectura hexagonal (puertos y adaptadores)**, lo que facilita intercambiar proveedores de IA, backends de VCS o bases de datos.

### Estrategias de Análisis

El motor de análisis utiliza el **patrón Strategy** con cuatro evaluadores independientes:

- **Arquitectura** — estructura del proyecto, separación de responsabilidades, gestión de dependencias
- **Calidad de Código** — legibilidad, mantenibilidad, cobertura de tests, mejores prácticas
- **Funcionalidad** — completitud de features, diseño de API, manejo de errores
- **DevOps** — CI/CD, containerización, monitoreo, preparación para despliegue

## 🛠️ Stack Tecnológico

| Capa | Tecnología |
|---|---|
| **Backend** | Go 1.25 · [Fiber v3](https://gofiber.io) |
| **Frontend** | [Next.js 16](https://nextjs.org) · React 19 · TypeScript |
| **Base de Datos** | PostgreSQL 16 · [pgvector](https://github.com/pgvector/pgvector) |
| **IA** | [Ollama](https://ollama.com) (embeddings + chat) |
| **Autenticación** | OAuth2 (Google, GitHub) · JWT |
| **Infraestructura** | Docker Compose |

## 🚀 Inicio Rápido

### Prerrequisitos

- **Go** ≥ 1.25
- **Node.js** ≥ 18
- **Docker** y Docker Compose
- **Ollama** ejecutándose localmente con los modelos descargados

```bash
# Descargar los modelos por defecto
ollama pull bge-m3      # embeddings
ollama pull qwen3       # chat
```

### 1. Clonar el repositorio

```bash
git clone https://github.com/arturoeanton/go-git-analyzer-ollama.git
cd go-git-analyzer-ollama
```

### 2. Configurar el entorno

```bash
cp .env.example .env
# Editar .env con tus credenciales OAuth y preferencias
```

### 3. Iniciar la base de datos

```bash
docker compose up -d
```

### 4. Ejecutar el backend

```bash
go run ./cmd/server
```

La API estará disponible en `http://localhost:3001`.

### 5. Ejecutar el frontend

```bash
cd web
npm install
npm run dev
```

La interfaz estará disponible en `http://localhost:3000`.

## 📡 Resumen de la API

Todos los endpoints (excepto auth y health) requieren un JWT válido en el header `Authorization`.

| Método | Endpoint | Descripción |
|---|---|---|
| `GET` | `/api/v1/health` | Verificación de salud |
| `GET/POST` | `/api/v1/auth/{provider}/*` | Flujo de autenticación OAuth2 |
| `GET/POST` | `/api/v1/repos` | Listar / agregar repositorios |
| `POST` | `/api/v1/analysis/run` | Ejecutar un análisis completo |
| `GET` | `/api/v1/reports` | Listar reportes de análisis |
| `POST` | `/api/v1/rag/query` | Hacer una pregunta sobre un repositorio (RAG) |
| `POST` | `/api/v1/rag/stream` | Consulta RAG con streaming (SSE) |
| `GET` | `/api/v1/audit` | Obtener registros de auditoría |

## 🤖 Integración MCP

Cuando `MCP_ENABLED=true`, un servidor [Model Context Protocol](https://modelcontextprotocol.io) separado se inicia en `MCP_PORT` (por defecto `3002`), exponiendo las capacidades de RAG y análisis a agentes de IA externos e IDEs.

## 📁 Estructura del Proyecto

```
.
├── cmd/server/          # Punto de entrada de la aplicación
├── internal/
│   ├── adapter/         # Implementaciones de infraestructura
│   │   ├── ai/          #   Proveedor Ollama
│   │   ├── analysis/    #   Implementaciones de estrategias
│   │   ├── auth/        #   OAuth de Google y GitHub
│   │   ├── store/       #   PostgreSQL + pgvector
│   │   └── vcs/         #   Operaciones Git
│   ├── domain/          # Modelos de dominio
│   ├── handler/         # Handlers HTTP (Fiber)
│   ├── mcp/             # Servidor MCP
│   ├── middleware/       # Middleware de JWT y auditoría
│   ├── port/            # Interfaces (puertos)
│   └── service/         # Lógica de negocio
├── migrations/          # Scripts de migración SQL
├── pkg/config/          # Cargador de configuración
├── web/                 # Frontend Next.js
│   └── src/app/
│       ├── dashboard/   # Dashboard principal, repos, reportes, auditoría
│       ├── login/       # Página de login
│       └── auth/        # Callback de OAuth
└── docker-compose.yml   # Setup de PostgreSQL + pgvector
```

## 🗄️ Esquema de Base de Datos

El esquema se gestiona mediante migraciones SQL en `migrations/`:

- **users** — perfiles de usuario OAuth2
- **repos** — repositorios Git registrados
- **snapshots** — snapshots inmutables a nivel de commit
- **embeddings** — embeddings de fragmentos de código con pgvector
- **analysis_results** — resultados de análisis por estrategia (con puntuaciones y sugerencias)
- **audit_logs** — registro completo de auditoría de peticiones

## 📄 Licencia

Este proyecto está licenciado bajo la [Licencia MIT](LICENSE).

---

<p align="center">
  Hecho con ❤️ por <a href="https://github.com/arturoeanton">Arturo Elias</a>
</p>

# Prompt — Construção da `video-processor-users-api`

## Contexto Geral

Você vai construir do zero o microsserviço **`video-processor-users-api`** em **Go 1.25**.

O repositório de referência de qualidade, padrões arquiteturais, bibliotecas e convenções é o **`tech-challenge-s1`** (oficina mecânica). Toda decisão de design — estrutura de pastas, nomenclatura de arquivos, injeção de dependência, tratamento de erro, padrões de resposta HTTP — deve ser idêntica à adotada nele.

O contrato da API está definido em **`openapi-users.yaml`** e é a fonte de verdade para rotas, schemas, validações e respostas de erro.

---

## Spec do Contrato (`openapi-users.yaml`) — resumo das regras de negócio

| Operação | Rota | Auth |
|---|---|---|
| Listar usuários (paginado) | `GET /users/` | Bearer JWT, role `administrator` |
| Criar usuário | `POST /users/` | Bearer JWT, role `administrator` |
| Buscar por ID | `GET /users/{id}` | Bearer JWT, role `administrator` |
| Atualizar usuário | `PUT /users/{id}` | Bearer JWT, role `administrator` |
| Deletar usuário | `DELETE /users/{id}` | Bearer JWT, role `administrator` |

**Regras críticas:**
- Toda rota exige role `administrator` (validada pelo JWT do authorizer **e** reconfirmada no handler).
- `POST /users/` grava o perfil no **RDS (PostgreSQL)** e, em seguida, chama o serviço externo `video-processor-authentication-api` para criar a credencial de autenticação; se a chamada ao serviço externo falhar, desfaz a criação no RDS (rollback lógico).
- `PUT /users/{id}` atualiza **apenas** o perfil no RDS — gestão de senha/credencial é responsabilidade da `authentication-api`, fora do escopo desta API.
- `DELETE /users/{id}` remove **apenas** o perfil do RDS — remoção de credencial é responsabilidade da `authentication-api`.
- Resposta nunca expõe `password_hash`.
- Erro de email já cadastrado retorna `400` com `code: EMAIL_ALREADY_EXISTS`.
- Erro de role insuficiente retorna `403` com `code: FORBIDDEN`.

---

## Estrutura de Pastas

Replique exatamente a estrutura hexagonal do `tech-challenge-s1`, adaptando para este domínio:

```
video-processor-users-api/
├── cmd/
│   └── api/
│       └── main.go                         # entry point, DI e wire-up
├── internal/
│   ├── domain/
│   │   ├── user.go                         # entidade User
│   │   └── vo/
│   │       ├── vo_document.go              # value object CPF (validação dígito verificador)
│   │       ├── vo_email.go                 # value object Email
│   │       └── vo_password.go              # value object Password (regras de complexidade)
│   ├── application/
│   │   ├── ports/
│   │   │   ├── user_service.go             # driving port (interface do service)
│   │   │   ├── user_repository.go          # driven port (RDS)
│   │   │   └── auth_service_client.go      # driven port (HTTP client → authentication-api)
│   │   └── services/
│   │       └── user_service.go             # casos de uso
│   └── adapter/
│       ├── config/
│       │   └── config.go                   # carrega .env via godotenv
│       ├── http/
│       │   ├── router.go                   # gin router + middlewares
│       │   ├── middleware/
│       │   │   ├── auth.go                 # valida JWT e injeta claims no contexto
│       │   │   └── role.go                 # verifica role administrator
│       │   ├── handlers/
│       │   │   └── user_handler.go         # HTTP handlers
│       │   └── response/
│       │       └── base_response.go        # estruturas de resposta padronizadas
│       ├── database/
│       │   ├── postgres.go                 # conexão GORM + PostgreSQL
│       │   ├── model/
│       │   │   └── user/
│       │   │       └── model.go            # GORM model com ToDomain() e FromDomain()
│       │   ├── repository/
│       │   │   └── user_repository.go      # implementação da driven port RDS
│       │   └── seeder.go                   # seed do admin inicial (opcional)
│       └── authclient/
│           └── auth_service_client.go      # HTTP client para video-processor-authentication-api
├── pkg/
│   ├── encryption/
│   │   └── hasher.go                       # bcrypt wrapper (interface + impl)
│   └── jwt/
│       └── jwt.go                          # parse e validação de JWT HS256
├── test/
│   └── e2e/
│       └── users_test.go                   # testes E2E das rotas
├── swagger/
│   └── swagger.yaml                        # cópia do openapi-users.yaml
├── Dockerfile                              # multi-stage (production_builder / development / production)
├── docker-compose.yml                      # postgres + app
├── air.toml                                # hot reload apontando para ./cmd/api
├── .golangci.yml                           # gosec + govet + errcheck + staticcheck + unused + gofmt + goimports
├── sonar-project.properties
├── go.mod                                  # module github.com/<org>/video-processor-users-api
├── go.sum
├── Makefile
└── .env.example
```

---

## Stack Tecnológica (espelhar `tech-challenge-s1`)

| Componente | Biblioteca |
|---|---|
| HTTP framework | `github.com/gin-gonic/gin v1.12.0` |
| Validação | `github.com/go-playground/validator/v10 v10.30.1` |
| ORM / RDS | `gorm.io/gorm` + `gorm.io/driver/postgres` |
| HTTP client externo | `net/http` stdlib + `encoding/json` (chamada à `authentication-api`) |
| JWT | `github.com/golang-jwt/jwt/v5 v5.3.0` |
| Crypto | `golang.org/x/crypto/bcrypt` |
| Env | `github.com/joho/godotenv v1.5.1` |
| Logging | `go.uber.org/zap v1.27.1` + `github.com/gin-contrib/zap` |
| CORS | `github.com/gin-contrib/cors v1.7.6` |
| Tracing | `github.com/DataDog/dd-trace-go/v2` + contrib gin + gorm |
| Testes | `github.com/stretchr/testify` + `github.com/DATA-DOG/go-sqlmock` |
| Swagger UI | `github.com/swaggo/gin-swagger` + `github.com/swaggo/files` |
| UUID | `github.com/google/uuid` |

---

## Padrão de Model GORM — `ToDomain` / `FromDomain`

**Não existe camada de converter separada.** A conversão entre model de persistência e entidade de domínio é responsabilidade do próprio model, via dois métodos obrigatórios — padrão idêntico ao `tech-challenge-s1`.

Cada model em `internal/adapter/database/model/` deve implementar:

```go
// ToDomain converte o model GORM para a entidade de domínio.
func (m *Model) ToDomain() *domain.User { ... }

// FromDomain popula o model a partir da entidade de domínio.
func (m *Model) FromDomain(d *domain.User) { ... }
```

**Exemplo de referência** (extraído do `tech-challenge-s1`, adaptado para User):

```go
package user

import (
    "time"
    "github.com/<org>/video-processor-users-api/internal/domain"
    "gorm.io/gorm"
)

type Model struct {
    gorm.Model
    Name         string `gorm:"not null"`
    Email        string `gorm:"uniqueIndex;not null"`
    Document     string `gorm:"not null"`
    Role         string `gorm:"not null"`
    PasswordHash string `gorm:"not null"`
}

func (*Model) TableName() string {
    return "users"
}

func (m *Model) ToDomain() *domain.User {
    var deletedAt *time.Time
    if m.DeletedAt.Valid {
        deletedAt = &m.DeletedAt.Time
    }
    return &domain.User{
        ID:           m.ID,
        Name:         m.Name,
        Email:        m.Email,
        Document:     m.Document,
        Role:         domain.Role(m.Role),
        PasswordHash: m.PasswordHash,
        CreatedAt:    m.CreatedAt,
        UpdatedAt:    m.UpdatedAt,
        DeletedAt:    deletedAt,
    }
}

func (m *Model) FromDomain(d *domain.User) {
    if d == nil {
        return
    }
    m.ID = d.ID
    m.Name = d.Name
    m.Email = d.Email
    m.Document = d.Document
    m.Role = string(d.Role)
    m.PasswordHash = d.PasswordHash
}
```

**Regras:**
- O model **nunca** é importado pelo `domain` nem pelo `application/services` — apenas pelo `adapter/database/repository`.
- O repository chama `model.ToDomain()` antes de retornar ao service e `model.FromDomain(entity)` antes de persistir.
- DTOs de request/response HTTP são definidos no `ports/user_service.go` e mapeados diretamente nos handlers — **não** via model.

---

## Variáveis de Ambiente (`.env.example`)

```env
# PostgreSQL (RDS)
DB_HOST=
DB_USER=
DB_PASSWORD=
DB_NAME=
DB_PORT=5432
DB_SSLMODE=disable
DB_TIMEZONE=UTC

# Serviço externo — authentication-api
AUTH_SERVICE_URL=         # ex: http://authentication-api:8081

# App
ENV=development
API_VERSION=1.0.0
GIN_MODE=debug
HTTP_PORT=8080
HTTP_ALLOWED_ORIGINS=*

# JWT
JWT_SECRET=your-secret-here
JWT_ACCESS_TOKEN_EXPIRY=15m

# DataDog
DD_API_KEY=
DD_SITE=
DD_AGENT_HOST=
DD_SERVICE=video-processor-users-api
DD_VERSION=1.0.0
```

---

## Contratos de Código — Ports (Interfaces)

### `ports/user_repository.go` (Driven Port — RDS)
```go
type UserRepository interface {
    Create(ctx context.Context, user *domain.User) error
    FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
    Update(ctx context.Context, user *domain.User) error
    Delete(ctx context.Context, id uuid.UUID) error
    List(ctx context.Context, limit, offset int) ([]*domain.User, error)
}
```

### `ports/auth_service_client.go` (Driven Port — HTTP client para `video-processor-authentication-api`)
```go
type AuthServiceClient interface {
    CreateCredential(ctx context.Context, email, passwordHash string, userID uuid.UUID, role domain.Role) error
}
```

> **Importante:** a gestão de credenciais de autenticação (atualização de senha, revogação) é responsabilidade exclusiva do serviço [`video-processor-authentication-api`](https://github.com/13SOAT-andromeda/video-processor-authentication-api). A `users-api` **não** chama DynamoDB diretamente — ela delega apenas a criação de credencial via chamada HTTP para aquele serviço no momento do `POST /users/`. Update e Delete de credencial **não** são responsabilidade desta API.
```

### `ports/user_service.go` (Driving Port)
```go
type UserService interface {
    Create(ctx context.Context, req dto.CreateUserRequest) (*dto.CreateUserResponse, error)
    GetByID(ctx context.Context, id uuid.UUID) (*dto.UserResponse, error)
    Update(ctx context.Context, id uuid.UUID, req dto.UpdateUserRequest) (*dto.UpdateUserResponse, error)
    Delete(ctx context.Context, id uuid.UUID) (*dto.DeleteUserResponse, error)
    List(ctx context.Context, limit, offset int) (*dto.UserListResponse, error)
}
```

---

## Padrão de Resposta HTTP (espelhar `base_response.go` do tech-challenge-s1)

**Sucesso:**
```json
{ "status": "success", "message": "...", "id": "<uuid>" }
```

**Erro:**
```json
{ "status": "error", "code": "INVALID_PAYLOAD", "message": "email: invalid format" }
```

Códigos de erro obrigatórios (enum da spec): `INVALID_PAYLOAD`, `EMAIL_ALREADY_EXISTS`, `UNAUTHORIZED`, `FORBIDDEN`, `USER_NOT_FOUND`.

---

## Regras de Implementação

1. **Arquitetura Hexagonal** — dependências sempre apontam para o centro. O `domain` não importa nada de `adapter`. O `service` não importa nada de `adapter/http`.

2. **DI no `main.go`** — instancie todas as dependências manualmente (sem frameworks de DI): `config → db → authclient → repositories → services → handlers → router`.

3. **Middleware de autenticação** — parse e valida o JWT (`HS256`, secret do env `JWT_SECRET`). Injeta `userID`, `email` e `role` no `gin.Context`.

4. **Middleware de autorização** — lê `role` do contexto; rejeita com `403 FORBIDDEN` se não for `administrator`.

5. **Integração com `authentication-api`** — no service, o fluxo de `Create` é: persistir no RDS → chamar `AuthServiceClient.CreateCredential`; se a chamada HTTP falhar, desfazer a inserção no RDS (rollback lógico via `UserRepository.Delete`). `Update` e `Delete` **não** interagem com a `authentication-api` — são operações exclusivas do RDS nesta API.

6. **Value Objects** — `vo_document.go` valida CPF com algoritmo de dígito verificador real. `vo_password.go` valida mínimo 8 chars, 1 maiúscula, 1 número, 1 símbolo.

7. **GORM model** — definido em `internal/adapter/database/model/user/model.go`, embute `gorm.Model` (fornece `ID uint`, `CreatedAt`, `UpdatedAt`, `DeletedAt` automaticamente), usa `gorm:"uniqueIndex;not null"` em `Email` e implementa obrigatoriamente `ToDomain() *domain.User` e `FromDomain(d *domain.User)`. O repository é o único pacote que importa o model.

8. **`authclient` HTTP adapter** — `internal/adapter/authclient/auth_service_client.go` implementa `ports.AuthServiceClient`; faz `POST` para `AUTH_SERVICE_URL/credentials` com body `{email, password_hash, user_id, role}`; trata erros HTTP (4xx/5xx) convertendo para erros de domínio reconhecíveis pelo service.

9. **Linter** — `.golangci.yml` idêntico ao do `tech-challenge-s1` (gosec obrigatório + govet + errcheck + staticcheck + unused + gofmt + goimports).

10. **Dockerfile** — multi-stage idêntico ao do `tech-challenge-s1` (`production_builder` → `development` com air → `production` com alpine:3.22.2 e usuário `nonroot`). Entry point: `./cmd/api`.

11. **Testes** — ao menos um teste E2E por endpoint em `test/e2e/users_test.go`. Unit tests nos services usando mocks gerados via `testify/mock`.

12. **Swagger** — servir o `swagger.yaml` em `/swagger/swagger.yaml`; UI em `/docs/index.html`; Redoc em `/redoc/`.

13. **DataDog** — instrumentar o roteador Gin e o GORM com os contrib packages do `dd-trace-go/v2`, idêntico ao `tech-challenge-s1`.

14. **Health check** — `GET /api/health` retorna `200 OK` com `{"status":"ok"}`.

---

## Planos de Execução

### Plano 1 — Scaffold e Configuração

**Objetivo:** repo funcional sem lógica de negócio.

1. `git init video-processor-users-api`
2. Criar `go.mod` com `module github.com/<org>/video-processor-users-api` e `go 1.25.0`
3. Criar toda a árvore de pastas acima (arquivos `.gitkeep` onde necessário)
4. Copiar e adaptar `.golangci.yml`, `air.toml`, `sonar-project.properties`, `.gitignore`
5. Criar `.env.example` com todas as variáveis listadas
6. Criar `internal/adapter/config/config.go` — struct `Config` com todos os campos, carregamento via `godotenv` e `os.Getenv`
7. Criar `cmd/api/main.go` esqueleto: carrega config, loga e retorna
8. Criar `Dockerfile` multi-stage (copiar do tech-challenge-s1, ajustar paths)
9. Criar `docker-compose.yml` com serviços: `postgres` e `app` (a `authentication-api` roda em repositório separado; usar `AUTH_SERVICE_URL` apontando para ela quando disponível)
10. Criar `Makefile` com targets: `run`, `build`, `test`, `lint`, `docker-up`, `docker-down`
11. `go mod tidy` — validar que compila

**Critério de aceite:** `go build ./cmd/api` sem erros; `docker-compose up` sobe postgres e app.

---

### Plano 2 — Domain e Ports

**Objetivo:** núcleo da aplicação sem dependências externas.

1. `internal/domain/user.go` — struct `User` com campos: `ID uuid.UUID`, `Name string`, `Email string`, `Document string`, `Role Role`, `PasswordHash string`, `CreatedAt time.Time`, `UpdatedAt time.Time`; type `Role string` com constantes `RoleAdministrator`, `RoleUser`
2. `internal/domain/vo/vo_document.go` — type `Document string`; func `NewDocument(raw string) (Document, error)` com validação CPF (algoritmo completo)
3. `internal/domain/vo/vo_email.go` — type `Email string`; func `NewEmail(raw string) (Email, error)` com regex RFC 5322 simplificado
4. `internal/domain/vo/vo_password.go` — type `Password string`; func `NewPassword(raw string) (Password, error)` com regras de complexidade
5. `internal/application/ports/user_repository.go` — interface `UserRepository` (conforme seção "Contratos de Código")
6. `internal/application/ports/auth_service_client.go` — interface `AuthServiceClient`
7. `internal/application/ports/user_service.go` — interface `UserService` + DTOs de request/response
8. `pkg/encryption/hasher.go` — interface `Hasher` com `Hash(plain string) (string, error)` e `Compare(hash, plain string) bool`; implementação bcrypt; mock para testes
9. Unit tests para todos os value objects

**Critério de aceite:** `go test ./internal/domain/...` 100 % passing.

---

### Plano 3 — Adapters Secundários

**Objetivo:** persistência e cliente HTTP externo funcionando.

1. `internal/adapter/database/postgres.go` — abre conexão GORM com instrumentação DataDog; auto-migrate do `model.Model` (pacote `user`)
2. `internal/adapter/database/repository/user_repository.go` — implementa `ports.UserRepository` com GORM; usa `model.FromDomain()` antes de persistir e `model.ToDomain()` antes de retornar; tratar `ErrRecordNotFound` → erro de domínio `ErrUserNotFound`; tratar violação de unique constraint de email → `ErrEmailAlreadyExists`
3. `internal/adapter/authclient/auth_service_client.go` — implementa `ports.AuthServiceClient`; `http.Client` com timeout configurável; lê `AUTH_SERVICE_URL` do env; método `CreateCredential` faz `POST /credentials` na `authentication-api` e mapeia erros HTTP para erros de domínio
4. `internal/adapter/database/seeder.go` — seed opcional do primeiro usuário admin (lê `ADMIN_EMAIL`/`ADMIN_PASSWORD`/`ADMIN_DOCUMENT` do env); chama `AuthServiceClient.CreateCredential` após inserção no RDS
5. Testes de repositório com `go-sqlmock`; mock de `AuthServiceClient` via `testify/mock`

**Critério de aceite:** `go test ./internal/adapter/...` passing; `docker-compose up` sobe postgres e app sem erros.

---

### Plano 4 — Application Service

**Objetivo:** casos de uso completos e testados.

1. `internal/application/services/user_service.go` — implementa `ports.UserService`
    - **Create**: validar VOs → `hasher.Hash(password)` → `UserRepository.Create` → `AuthServiceClient.CreateCredential`; se `CreateCredential` falhar: `UserRepository.Delete` (rollback lógico) e retornar erro
    - **GetByID**: `UserRepository.FindByID` → mapear campos do `domain.User` para DTO de resposta diretamente no service (sem password_hash)
    - **Update**: `UserRepository.FindByID` → atualizar campos da entidade → `UserRepository.Update` (sem interação com `authentication-api`)
    - **Delete**: `UserRepository.Delete` (sem interação com `authentication-api`)
    - **List**: `UserRepository.List` com limit/offset → mapear slice de `domain.User` para `UserListResponse` diretamente no service
2. Unit tests do service com mocks de `UserRepository`, `AuthServiceClient` e `Hasher`

**Critério de aceite:** `go test ./internal/application/...` passing com cobertura ≥ 80 %.

---

### Plano 5 — Adapter HTTP

**Objetivo:** API HTTP funcional e documentada.

1. `internal/adapter/http/response/base_response.go` — structs `SuccessResponse`, `ErrorResponse`, funções helper `RespondSuccess`, `RespondError`, `RespondCreated`
2. `internal/adapter/http/middleware/auth.go` — extrai Bearer token do header `Authorization`, valida JWT com `JWT_SECRET`, injeta claims no `gin.Context` (chaves: `"userID"`, `"email"`, `"role"`)
3. `internal/adapter/http/middleware/role.go` — middleware factory `RequireRole(role domain.Role)` que lê `"role"` do contexto e aborta com `403` se não bater
4. `internal/adapter/http/handlers/user_handler.go` — `UserHandler` struct com `userService ports.UserService`; métodos `Create`, `GetByID`, `Update`, `Delete`, `List`; bind JSON → chamar service → responder
5. `internal/adapter/http/router.go` — configura gin, registra CORS, logging zap, tracing DataDog, health check, grupo `/users` com middlewares `auth` + `RequireRole(RoleAdministrator)`, registra handlers, serve Swagger UI e Redoc
6. `pkg/jwt/jwt.go` — `ParseToken(tokenStr, secret string) (*Claims, error)` com validação de expiração e algoritmo HS256
7. Atualizar `cmd/api/main.go` com DI completo

**Critério de aceite:** `curl localhost:8080/api/health` retorna `200`; todas as rotas respondem `401` sem token.

---

### Plano 6 — Testes E2E e Qualidade

**Objetivo:** cobertura E2E, linter limpo, documentação.

1. `test/e2e/users_test.go` — testes para: criar usuário, listar, buscar por ID, atualizar, deletar; cenários de erro (401, 403, 404, 400 email duplicado)
2. `test/e2e/helper.go` — setup de cliente HTTP, geração de token JWT de teste
3. Rodar `golangci-lint run` — corrigir todos os warnings do gosec e demais linters
4. Rodar `go test ./... -coverprofile=coverage.out` — atingir ≥ 80 % de cobertura geral
5. Atualizar `sonar-project.properties` com `sonar.projectKey` correto
6. Revisar `README.md` com instruções de setup, execução e teste

**Critério de aceite:** `golangci-lint run` sem erros; `go test ./...` 100 % passing; cobertura ≥ 80 %.

---

## Checklist Final

- [ ] `go build ./cmd/api` sem warnings
- [ ] `golangci-lint run` limpo (gosec incluso)
- [ ] `go test ./... -count=1` passing
- [ ] `coverage.out` gerado com ≥ 80 %
- [ ] `docker-compose up --build` sobe sem erros
- [ ] `GET /api/health` retorna `200 {"status":"ok"}`
- [ ] `POST /users/` sem token retorna `401`
- [ ] `POST /users/` com token `role: user` retorna `403`
- [ ] `POST /users/` com token `role: administrator` cria usuário no RDS e chama `authentication-api` para criar credencial
- [ ] `DELETE /users/{id}` remove perfil do RDS
- [ ] Swagger UI acessível em `/docs/index.html`
- [ ] Nenhum campo `password_hash` exposto em nenhuma resposta
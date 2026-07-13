# Spec — video-processor-authentication-api

**Data:** 2026-07-12
**Status:** Draft — pronto para virar plano de implementação
**Repo antigo de referência (esqueleto estrutural apenas, não domínio):** `tech-challenge-user-authentication`
**Spec relacionada:** `docs/superpowers/specs/2026-07-11-users-api-design.md` (dono de `auth-credentials`)
**Contrato HTTP:** `openapi-authentication.yaml` (raiz deste repo)

> Este serviço ainda não tem repositório próprio na workspace atual; este documento formaliza o modelo de dados e as regras de negócio necessárias para o contrato já definido em `openapi-authentication.yaml`, em particular a dependência de uma tabela DynamoDB de sessão que motivou este spec.

---

## 1. Responsabilidade

Login, renovação (`refresh`) e logout de sessão. **Não cria nem altera** dados de usuário ou credencial — acesso de leitura apenas a `auth-credentials` (tabela de dono `video-processor-users-api`, ver `2026-07-11-users-api-design.md`, seção 2). É o único serviço com acesso de escrita à tabela `auth-sessions`, definida abaixo.

## 2. Modelo de dados

### 2.1 `auth-credentials` (DynamoDB) — leitura apenas

Item existente, escrito por `video-processor-users-api`: chave de partição `email`, atributos `userId`, `password_hash`, `role`. Este serviço faz apenas `GetItem` por `email` no login; nunca escreve neste item.

### 2.2 `auth-sessions` (DynamoDB) — nova, dono exclusivo deste serviço

| Atributo | Tipo | Descrição |
|---|---|---|
| `session_id` (PK) | string (UUID) | Igual ao claim `jti` emitido no access token e no refresh token do mesmo login — permite localizar a sessão a partir de qualquer um dos dois tokens. |
| `user_id` | string | Copiado de `auth-credentials.userId` no momento do login. |
| `email` | string | Copiado de `auth-credentials.email`. |
| `role` | string | Copiado de `auth-credentials.role` (`administrator` \| `user`). |
| `refresh_token_hash` | string | SHA-256 do refresh token JWT — nunca o token em texto puro, para que um leak da tabela não implique posse direta de um refresh token válido. |
| `created_at` | string (RFC3339) | Momento do login. |
| `expires_at` | number (epoch seconds) | Atributo de **TTL do DynamoDB** — igual à expiração do refresh token (ex.: login + 7 dias). Limpeza automática independe do fluxo de logout. |

```go
// authentication-api/internal/domain/session/session.go
type Session struct {
    SessionID         string
    UserID             string
    Email              string
    Role               string
    RefreshTokenHash   string
    CreatedAt          time.Time
    ExpiresAt          int64 // epoch seconds, atributo TTL
}

type Repository interface {
    Create(ctx context.Context, s *Session) error
    FindByID(ctx context.Context, sessionID string) (*Session, error)
    Delete(ctx context.Context, sessionID string) error
}
```

Não há GSI: todos os acessos (login cria, refresh e logout leem/removem) são por `session_id`, obtido do claim `jti` do token apresentado — não é necessário buscar sessões por `user_id` neste escopo (sem endpoint de "logout em todos os dispositivos").

## 3. Contrato

Ver `openapi-authentication.yaml` para o contrato completo (schemas, exemplos, códigos de erro). Resumo:

```
POST   /sessions            (público)  -> login, cria item em auth-sessions
POST   /sessions/refresh    (público)  -> renova access token, lê auth-sessions
DELETE /sessions/logout     [bearer]   -> remove item de auth-sessions
```

## 4. Regras de negócio

1. **Login**: `GetItem` em `auth-credentials` por `email`. Se não existir OU `bcrypt.CompareHashAndPassword` falhar, responder `401 INVALID_CREDENTIALS` com a mesma mensagem genérica nos dois casos (evitar enumeração de e-mails cadastrados — ver `openapi-authentication.yaml`, resposta `InvalidCredentials`).
2. **Emissão de tokens**: gerar `session_id` (UUID) novo por login; assinar access token (claims: `sub`, `email`, `role`, `jti=session_id`, `iss`, `exp` curto — ex. 15 min) e refresh token (claims: `sub`, `jti=session_id`, TTL longo — ex. 7 dias) com segredos distintos (`JWT_SECRET` / `JWT_REFRESH_SECRET`). Gravar `Session` em `auth-sessions` (com `refresh_token_hash`) **antes** de responder `200`.
3. **Refresh**: decodificar o refresh token recebido (verificar assinatura e `exp`); buscar `auth-sessions` por `session_id=jti`. Se não encontrado, ou `refresh_token_hash` não bater com o hash do token recebido, ou `expires_at` no passado → `401 INVALID_REFRESH_TOKEN` (mensagem genérica — não distinguir "sessão não encontrada" de "expirada" de "revogada", mesmo racional do item 1). Em caso de sucesso, emitir **novo access token** reaproveitando o `session_id` existente; **não** gera novo refresh token (sem rotação neste escopo — consistente com o esqueleto antigo).
4. **Logout**: extrair `session_id` do claim `jti` do access token validado (`Authorization: Bearer`); `DeleteItem` em `auth-sessions` por esse `session_id`. Idempotente — sessão já removida/inexistente também responde `204`.
5. Este serviço nunca escreve em `auth-credentials` nem no RDS `users` — qualquer necessidade de alterar credencial ou perfil é responsabilidade exclusiva de `video-processor-users-api` (ADR-010, ver spec de users).

## 5. Dependências

- DynamoDB `auth-credentials` — `GetItem` (leitura apenas; ARN da tabela de `video-processor-users-api`).
- DynamoDB `auth-sessions` — `PutItem`, `GetItem`, `DeleteItem` (tabela própria deste serviço; TTL habilitado no atributo `expires_at`).
- Bibliotecas: `github.com/golang-jwt/jwt/v5`, `github.com/aws/aws-sdk-go-v2/service/dynamodb`, `golang.org/x/crypto/bcrypt`, `github.com/google/uuid`, `github.com/awslabs/aws-lambda-go-api-proxy` (framework HTTP a definir — ver esqueleto antigo para padrão de handlers sem framework, se optar por manter simplicidade).

## 6. Config Lambda / Terraform

- `memory 256MB`, `timeout 10s`, `arch arm64`.
- Sem VPC — acesso apenas a DynamoDB (diferente de `video-processor-users-api`, que precisa de VPC para o RDS).
- IAM: `dynamodb:GetItem` no ARN de `auth-credentials` (tabela de outro serviço — permissão cross-service); `dynamodb:PutItem`/`GetItem`/`DeleteItem` no ARN de `auth-sessions` (tabela própria); TTL habilitado via `aws_dynamodb_table.ttl` no atributo `expires_at`.
- Segredos `JWT_SECRET` / `JWT_REFRESH_SECRET` via variável de ambiente Lambda (validar se devem migrar para Secrets Manager antes de produção — não decidido neste spec).

## 7. Testes

- Unitário: usecase de login/refresh/logout com mock do client DynamoDB (duas tabelas) e do serviço JWT.
- Regressão: garantir que `refresh_token_hash` (não o token em texto puro) é o que fica persistido em `auth-sessions`, e que a mensagem de erro de login é idêntica para "email não encontrado" e "senha incorreta".
- Contrato: `httptest` cobrindo os 3 endpoints de `openapi-authentication.yaml` + casos de erro.

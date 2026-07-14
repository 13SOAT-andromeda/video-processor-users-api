# Spec — video-processor-users-api

**Data:** 2026-07-11 (atualizado 2026-07-13 — deploy muda de Lambda para container no EKS)
**Status:** Draft — pronto para virar plano de implementação
**Repo antigo de referência (esqueleto estrutural apenas, não domínio):** `tech-challenge-users`
**Spec guarda-chuva:** `docs/superpowers/specs/2026-07-11-video-processor-auth-infra-migration-design.md` (workspace raiz)

> Este documento copia na íntegra `service-users.md` (fonte: `Video Processing/specs/specs/`) e adiciona, na seção 8, o que é específico deste repositório (porta do código antigo, Terraform local, dependências).

---

## 1. Responsabilidade

CRUD de usuários. Todas as rotas exigem role `administrator` — checado a partir do `context` injetado pelo `authorizer`, e reconferido dentro do handler por defesa em profundidade (nunca confiar só na borda).

## 2. Modelo de dados — dono exclusivo de dois armazenamentos (ADR-010 revisada)

`users-service` é o **único** serviço com acesso de escrita a dados de usuário, em dois armazenamentos distintos (ver `2026-07-09-user-data-access-design.md` no root do projeto):

- **RDS PostgreSQL** (`users`) — perfil administrativo: `name`, `email` (cópia de leitura), `role` (cópia de leitura), `document`. **Não contém `password_hash`.**
- **DynamoDB** (`auth-credentials`) — dados de autenticação: chave de partição `email`, atributos `userId`, `password_hash`, `role`. Fonte de verdade da credencial, consultada por `video-processing-authentication` em modo somente leitura (ver `service-authentication.md`, seção 5.1).

Não existe mais pacote Go compartilhado entre `users-service` e `authentication`. O único acoplamento entre os dois é o **schema do item DynamoDB**, escrito por um e lido pelo outro.

```go
// users-service/internal/domain/user/user.go
type User struct {
    ID        string    `gorm:"type:uuid;primaryKey"`
    Name      string    `gorm:"not null"`
    Email     string    `gorm:"uniqueIndex;not null"`
    Role      string    `gorm:"not null"` // administrator | user
    Document  string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Repository interface {
    FindByEmail(ctx context.Context, email string) (*User, error)
    FindByID(ctx context.Context, id string) (*User, error)
    List(ctx context.Context, limit, offset int) ([]User, error)
    Create(ctx context.Context, u *User, passwordHash string) error // grava RDS + DynamoDB
    Update(ctx context.Context, u *User, passwordHash *string) error // passwordHash nil = não altera credencial
    Delete(ctx context.Context, id string) error // remove RDS + DynamoDB
}
```

`Create`/`Update`/`Delete` escrevem no DynamoDB primeiro (fonte de verdade da credencial) e depois no RDS — se a segunda escrita falhar, retornar erro ao chamador; ver seção 4, item 5, sobre o risco aceito de escrita dupla sem saga.

DTO de resposta (`UserResponse`) **nunca inclui `PasswordHash`** — struct separada, nunca serializar a entidade `user.User` diretamente na resposta HTTP (mais simples agora, já que `PasswordHash` nem existe no struct do RDS).

## 3. Contrato

```
GET    /users              [administrator]  -> lista paginada
GET    /users/:id          [administrator]
POST   /users              [administrator]
PUT    /users/:id          [administrator]
DELETE /users/:id          [administrator]
```

`GET /users` — paginação por `limit` (padrão 20, máx 100) e `offset` (volume esperado baixo no hackathon; migrar para cursor se crescer).

**POST /users** — payload:
```json
{ "name": "John Doe", "email": "john.doe9@example.com", "password": "MockAdminPass!2025", "role": "user", "document": "652.904.150-84" }
```
Resposta `201`: `{ "status": "success", "message": "User successfully created", "id": "..." }`

**Erros:** `400 INVALID_PAYLOAD`, `400 EMAIL_ALREADY_EXISTS`, `401 UNAUTHORIZED`, `403 FORBIDDEN` (role ≠ administrator), `404 USER_NOT_FOUND`.

## 4. Regras de negócio

1. Hash de senha (`bcrypt.GenerateFromPassword`, cost 10-12) ao criar/atualizar — nunca aceitar senha já em hash vindo do client. Gravado **somente** no item DynamoDB (`auth-credentials`), nunca no RDS (ver seção 2).
2. Validação de `email` (formato) e `document` (CPF — validar dígito verificador) antes de persistir.
3. `DELETE /users/:id`: soft-delete opcional a decidir com o grupo; se um usuário com links ativos for removido, os links continuam acessíveis por `linkId` (não fazer cascade delete). Remoção precisa apagar tanto o registro RDS quanto o item DynamoDB (`auth-credentials`).
4. Unicidade de `email` garantida por índice único no banco + checagem prévia amigável (`400 EMAIL_ALREADY_EXISTS` em vez de erro genérico de constraint).
5. Escrita em `Create`/`Update`: gravar primeiro no DynamoDB (fonte de verdade da credencial), depois no RDS. Se a escrita no RDS falhar após o DynamoDB ter sido gravado, retornar `500 INTERNAL_ERROR` e logar para investigação manual — risco aceito dado o volume administrativo baixo esperado (ver `2026-07-09-user-data-access-design.md`, seção 6).

## 5. Dependências

- RDS PostgreSQL (tabela `users`, sem `password_hash`).
- DynamoDB (tabela `auth-credentials`) — `PutItem`, `UpdateItem`, `DeleteItem`, `GetItem`.
- Bibliotecas: `gorm.io/gorm`, `gorm.io/driver/postgres`, `github.com/aws/aws-sdk-go-v2/service/dynamodb`, `golang.org/x/crypto/bcrypt`, `github.com/gin-gonic/gin`, `github.com/awslabs/aws-lambda-go-api-proxy/gin`, `github.com/google/uuid`.

## 6. Config de deploy (EKS/container) — atualizado 2026-07-13

**Backend muda de Lambda para container no EKS** (ver spec guarda-chuva e `iac-video-processor-infra`, seção 1). Motivo: `users-api` passou a ser tratada como uma API "de verdade" (não function-as-a-service), no mesmo padrão de deploy que as futuras `video-processor-api`/`links-generator` vão usar — todas atrás do mesmo Application Load Balancer compartilhado (ver `iac-video-processor-gateway`, seção 7).

- **Recursos do container** (mesma ordem de grandeza do antigo dimensionamento Lambda, adaptado pro padrão de `resources.requests/limits` do `tech-challenge-users/k8s/base/deployment.yaml`): `cpu: 50m` (request) / `200m` (limit), `memory: 64Mi` (request) / `128Mi` (limit). `replicas: 1` + HPA (mesmo padrão do repo antigo) — uso administrativo, baixo volume não justifica réplicas altas por padrão.
- **Porta:** `8081` (mesma do `tech-challenge-users`, sem motivo pra mudar), `livenessProbe`/`readinessProbe` em `/health`.
- **Sem necessidade de reserved concurrency** — esse conceito era específico de Lambda; não se aplica mais.
- **Rede:** o pod precisa alcançar o RDS de `iac-video-processor-data` (porta 5432) — via security group do node group liberando egress pra VPC, não via ENI de VPC de Lambda como antes.
- **Acesso ao DynamoDB (`auth-credentials`):** ver seção 8.2 — via credenciais de sessão do Academy injetadas como Kubernetes `Secret`, não via IAM role de execução (que não existe mais aqui, já que não há mais Lambda). Sem acesso a S3/SQS/SNS.

## 7. Testes

- Unitário: repositório RDS com `sqlmock` (ou testcontainers-go + Postgres real em CI) + mock do client DynamoDB.
- Contrato: `httptest` cobrindo os 5 endpoints + casos de erro de validação e de autorização.
- Teste de regressão: garantir que `password_hash` nunca aparece em nenhuma resposta serializada, e que `Create`/`Update`/`Delete` sempre gravam/removem nos dois armazenamentos (RDS + DynamoDB).

---

## 8. Contexto de migração/repositório (específico deste repo)

### 8.1 Ressalva sobre o repo antigo `tech-challenge-users`

`tech-challenge-users` (commitado recentemente na workspace) **não é do mesmo domínio de negócio**: é um sistema de oficina/garagem (`company`, `employee`, `vehicle`, `customer_vehicle`), não usuários de autenticação. **Não portar** `internal/domain/{company,employee,vehicle,customer*}.go` nem os usecases/services associados a esses domínios.

O que **é** reaproveitável dali é só o **esqueleto de repositório** (organização de pastas, ferramentas de qualidade, testes de carga/e2e):

| Do antigo (esqueleto, reaproveitar) | Propósito |
|---|---|
| `k8s/base/`, `k8s/overlays/aws/` | Manifests Kustomize — **uso ativo nesta fase** (atualizado 2026-07-13): backend deste serviço agora é container no EKS, não mais Lambda. Reaproveitar quase 1:1 (`deployment.yaml`, `service.yaml`, `hpa.yaml`, `kustomization.yaml`), trocando nome/imagem/porta conforme necessário. **Sem `ingress.yaml`** (correção 2026-07-13, ver seção 8.2) — o roteamento é centralizado em `iac-video-processor-infra`. |
| `test/e2e/` | Estrutura de teste e2e (Go) — adaptar para os 5 endpoints de `/users` |
| `test/stress/stress-test.js` | Estrutura de teste de carga (k6/similar) — adaptar para o volume de uso administrativo |
| `sonar-project.properties` | Config de qualidade de código — copiar e renomear `projectKey` |
| `docs/internal-api.md` | Padrão de documentação de API interna — adaptar ao contrato da seção 3 |
| `internal/domain`, `internal/application/{ports,usecases,services}`, `internal/adapter/{http,database,config}` | **Só a organização de camadas**, não o conteúdo — o `user.go`/`services/user.go` de lá tem regras de outro domínio |

### 8.2 Deploy — Kustomize + ECR + credenciais via Kubernetes Secret (reescrito 2026-07-13)

**Não há mais `terraform/` local com `aws_lambda_function`.** O deploy deste serviço passa a ser:

1. **Build + push de imagem** pro ECR provisionado em `iac-video-processor-infra` (repositório `video-processor-users-api`), no pipeline de CI deste repo — mesmo padrão do `tech-challenge-users/.github/workflows/deploy.yml`.
2. **`kubectl apply -k k8s/overlays/aws/`** contra o cluster `video-processor-eks` (kubeconfig obtido via `aws eks update-kubeconfig`) — aplica **Deployment, Service, HPA** deste serviço.
3. **Sem `Ingress` próprio** (corrigido 2026-07-13 — a v1 desta seção propunha um `Ingress` por serviço com `group.name` compartilhado; revertido pelo mesmo motivo que o `tech-challenge` reverteu: sem dono único, um serviço esquecer a annotation gera um ALB extra e quebra a descoberta do gateway). O roteamento `/users/*` → este `Service` (porta 80) é uma regra de path no `Ingress` **centralizado**, mantido e aplicado por `iac-video-processor-infra` (seção 6.1 daquele spec) — este repo só precisa garantir que o `Service` se chame `video-processor-users-api-svc` na porta 80, que é o nome que aquele `Ingress` referencia.
4. **Acesso ao DynamoDB (`auth-credentials`):** sem IAM role de execução (não existe mais Lambda aqui). Em vez disso, as credenciais temporárias da sessão do AWS Academy (`AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`/`AWS_SESSION_TOKEN`, vindas de secrets do GitHub Actions) são injetadas como Kubernetes `Secret` e consumidas pelo container via env vars lidas pelo AWS SDK — **mesmo padrão exato já usado por `tech-challenge-users`** (ver `.env.secrets`/`k8s/overlays/aws/kustomization.yaml` daquele repo). Isso evita precisar de IRSA/OIDC provider, que exigiria criar uma IAM role nova — não permitido sob a `LabRole` do Academy (ver `iac-video-processor-infra`, seção 5).
5. **Acesso ao RDS:** via rede (security group do node group liberado para a porta 5432 do RDS), com credenciais de banco (usuário/senha) injetadas também via `Secret`, exatamente como já é feito hoje em `tech-challenge-users` (`DB_HOST`/`DB_PASSWORD` etc. gerados no pipeline a partir do endpoint do RDS).

### 8.3 Dependências

- Depende de `iac-video-processor-data` (RDS `usersdb` + tabela DynamoDB `auth-credentials` existirem).
- Depende de `iac-video-processor-infra` (cluster EKS `video-processor-eks`, VPC/subnets, ECR, AWS Load Balancer Controller **e o `Ingress` centralizado com a regra `/users` já apontando pro `Service` deste repo**, seção 6.1 daquele spec) — atualizado 2026-07-13, antes dependia só de VPC/subnets pra um Lambda.
- É pré-requisito de `video-processor-authentication-api` (que lê a tabela `auth-credentials` escrita por este serviço). `iac-video-processor-gateway` depende do `Ingress`/ALB de `iac-video-processor-infra`, não deste repo diretamente (corrigido 2026-07-13) — ver ordem de implementação na spec guarda-chuva, seção 8.

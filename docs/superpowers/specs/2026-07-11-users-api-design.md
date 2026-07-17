# Spec — video-processor-users-api

**Data:** 2026-07-11 (atualizado 2026-07-13 — deploy muda de Lambda para container no EKS; renomeado 2026-07-15 — futuro serviço `video-processor-api` passa a se chamar `video-processor-converter`; atualizado 2026-07-16 — Profile puro, self-service progressive profiling, ADR-011)
**Status:** Draft — pronto para virar plano de implementação
**Repo antigo de referência (esqueleto estrutural apenas, não domínio):** `tech-challenge-users`
**Spec guarda-chuva:** `docs/superpowers/specs/2026-07-11-video-processor-auth-infra-migration-design.md` (workspace raiz), atualizada em 2026-07-16
**RFCs de origem desta revisão:** `RFC_service-authentication.md`, `RFC_service-users.md` (ADR-011)

> **Revisão 2026-07-16 — substitui o desenho anterior** (dono de dois armazenamentos — RDS + DynamoDB, com escrita dupla de credencial). `users-service` passa a ser **exclusivamente** o serviço de Profile (RDS): zero acesso a DynamoDB, zero chamada síncrona a `authentication` em qualquer direção (ver spec de `authentication`, ADR-011). O cadastro passa a ser 100% self-service: signup (email/senha) em `authentication`, dados de perfil preenchidos progressivamente aqui via `PUT /users/me`. `POST /users` (criação administrativa de usuário completo) é **removido** — não sobra nenhuma forma de um admin criar um usuário de credencial+perfil de uma vez só, já que todo cadastro passa pelo signup público.

---

## 1. Responsabilidade

Serviço de **Profile**: dono exclusivo da tabela `users` no RDS PostgreSQL (nome, telefone, endereço, documento). Sem nenhuma dependência de DynamoDB e sem nenhuma chamada a `authentication`. Duas superfícies: self-service (o próprio usuário completa/edita seu perfil) e administrativa (leitura/edição/remoção de perfis de terceiros, `role administrator`).

## 2. Modelo de dados — único armazenamento (RDS), Profile puro

`users-service` só grava em RDS PostgreSQL. `email`/`role` são **cópias de leitura** — a fonte de verdade continua em `auth-credentials` (DynamoDB), gerenciada exclusivamente por `authentication`. `users-service` nunca lê nem escreve em DynamoDB.

```go
// users-service/internal/domain/user/user.go
type User struct {
    ID        string `gorm:"type:uuid;primaryKey"`
    Name      string
    Email     string `gorm:"uniqueIndex;not null"`
    Role      string `gorm:"not null"` // cópia de leitura
    Phone     string
    Address   string
    Document  string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Repository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    List(ctx context.Context, limit, offset int) ([]User, error)
    Upsert(ctx context.Context, u *User) error // usado por GET/PUT /users/me
    Update(ctx context.Context, u *User) error // usado por PUT /users/:id [admin]
    Delete(ctx context.Context, id string) error // usado por DELETE /users/:id [admin] — só remove o perfil
}
```

**Não contém `password_hash`** — nunca conteve, e agora não há mais nenhum outro dado de credencial neste serviço.

`ID` é o mesmo valor gerado por `authentication` no signup (`userId`) — a linha em `users` só passa a existir no primeiro `PUT /users/me` (o signup em `authentication` **não** cria linha em `users`; ver seção 4).

## 3. Contrato

Autenticação: `Authorization: Bearer <jwt>` em todas as rotas — `users-api` valida o JWT **por conta própria** (ver seção 5.1), não confia em nenhum header repassado pelo gateway.

### Self-service (dono do recurso)

| Method | Endpoint | Descrição |
|---|---|---|
| GET | `/users/me` | Retorna o próprio perfil (campos podem vir nulos/vazios se ainda incompleto — usado pelo frontend pra decidir se força a tela de completar perfil) |
| PUT | `/users/me` | Upsert do próprio perfil — `name`, `phone`, `address`, `document` |

Request Payload (`PUT /users/me`)
```json
{ "name": "John Doe", "phone": "+55 11 90000-0000", "address": "Rua Exemplo, 123", "document": "652.904.150-84" }
```

### Backoffice: Administração (`role administrator`)

| Method | Endpoint | Descrição |
|---|---|---|
| GET | `/users` | Listagem paginada (`limit` padrão 20, máx 100; `offset`) |
| GET | `/users/:id` | Detalhe |
| PUT | `/users/:id` | Atualização de perfil de terceiro (não cria/altera credencial, não toca `auth-credentials`) |
| DELETE | `/users/:id` | Remove **só** o perfil no RDS — ver seção 4, item 4, sobre a credencial órfã |

**Removido nesta revisão:** `POST /users` (criação administrativa de usuário completo). Não existe mais forma de um admin criar um usuário de credencial+perfil de uma vez — todo cadastro passa pelo signup público de `authentication`.

**Erros:** `400 INVALID_PAYLOAD`, `401 UNAUTHORIZED`, `403 FORBIDDEN` (self-service tentando editar outro `userId`, ou não-admin em rota administrativa), `404 USER_NOT_FOUND`.

## 4. Regras de negócio

1. `GET/PUT /users/me`: o `userId` vem do JWT decodificado por este serviço (nunca de um path param ou header client-controlled). `PUT /users/me` faz upsert — cria a linha no primeiro `PUT`, já que o signup em `authentication` não cria linha em `users`.
2. `PUT /users/me` **nunca** altera `role` ou `email` — esses campos não fazem parte do payload aceito (ver seção 7, teste de regressão).
3. Validação de `document` (CPF — validar dígito verificador) e `phone` antes de persistir.
4. `DELETE /users/:id`: remove **apenas** a linha em `users` (RDS). **Não chama `authentication`** — a credencial em `auth-credentials` fica órfã (o usuário consegue continuar autenticando, mas sem perfil). Limitação aceita nesta fase (decisão de brainstorming, ADR-011) em troca do desacoplamento total entre os dois serviços; revisitar se isso virar um problema de segurança/produto real (ver spec guarda-chuva, seção 10). Sem cascade delete de `links` (mantido da revisão anterior — links continuam acessíveis por `linkId`).
5. Unicidade de `email` garantida por índice único no RDS (cópia de leitura, mas ainda única).

## 5. Dependências

- RDS PostgreSQL (tabela `users`, sem `password_hash`, com `phone`/`address`).
- Secrets Manager: `jwt-signing-key` (mesmo segredo de `authorizer`/`authentication` — novo para este serviço, ver seção 5.1).
- Bibliotecas: `gorm.io/gorm`, `gorm.io/driver/postgres`, `github.com/golang-jwt/jwt/v5` (novo), `github.com/gin-gonic/gin`, `github.com/awslabs/aws-lambda-go-api-proxy/gin` (se aplicável ao adapter HTTP), `github.com/google/uuid`.

**Removido nesta revisão:** `github.com/aws/aws-sdk-go-v2/service/dynamodb`, `golang.org/x/crypto/bcrypt` — este serviço não toca mais em nenhum dado de credencial nem faz hash de senha.

### 5.1 Validação de JWT própria (decisão de brainstorming, substitui repasse de headers pelo gateway)

A integração `HTTP_PROXY` do `iac-video-processor-gateway` pro ALB não repassa automaticamente o `context` do `authorizer` (isso só acontece "de graça" em integrações Lambda proxy). Em vez de o gateway repassar `userId`/`role` como headers customizados (opção descartada — implicaria confiar cegamente na borda, quebrando o padrão de defesa em profundidade já usado no resto do projeto), `users-api` decodifica e valida o JWT por conta própria, com a mesma lib (`golang-jwt/jwt/v5`) e o mesmo segredo (`jwt-signing-key`, Secrets Manager) que `authorizer`/`authentication` já usam. É redundante com a validação do `authorizer` na borda por design — o `authorizer` continua servindo pra rejeitar requisições sem token válido antes de gastar um hop de rede até o ALB, mas `users-api` nunca confia cegamente nisso.

**IAM**: `secretsmanager:GetSecretValue` no ARN de `jwt-signing-key` — via as mesmas credenciais de sessão/`LabRole` já injetadas no pod (sem recurso Terraform novo, já que `users-api` não tem execution role própria).

## 6. Config de deploy (EKS/container)

**Backend é container no EKS** (ver spec guarda-chuva e `iac-video-processor-infra`, seção 1), atrás do Application Load Balancer compartilhado (ver `iac-video-processor-gateway`, seção 7).

- **Recursos do container**: `cpu: 50m` (request) / `200m` (limit), `memory: 64Mi` (request) / `128Mi` (limit). `replicas: 1` + HPA — uso administrativo/self-service, baixo volume não justifica réplicas altas por padrão.
- **Porta:** `8081`, `livenessProbe`/`readinessProbe` em `/health`.
- **Rede:** o pod precisa alcançar o RDS de `iac-video-processor-data` (porta 5432) via security group do node group.
- **Credenciais AWS no pod (revisado 2026-07-16)**: injetadas via Kubernetes `Secret` (sessão do Academy) **apenas** para `secretsmanager:GetSecretValue` (segredo `jwt-signing-key`, ver seção 5.1) e para as credenciais de banco do RDS (`DB_HOST`/`DB_PASSWORD`, via `Secret` separado). **Sem acesso a DynamoDB, S3, SQS ou SNS** — simplificação em relação à revisão anterior, que ainda precisava de credenciais AWS pra escrever em DynamoDB.

## 7. Testes

- Unitário: repositório RDS com `sqlmock` (ou testcontainers-go + Postgres real em CI) + teste de decodificação/validação de JWT.
- Contrato: `httptest` cobrindo os 6 endpoints (`GET/PUT /users/me`, `GET /users`, `GET/PUT/DELETE /users/:id`) + casos de erro de validação e de autorização.
- Teste de regressão: `password_hash` nunca aparece em nenhuma resposta (nem existe no struct); `PUT /users/me` nunca permite alterar `role` ou `email` (campos fora do payload aceito); `GET/PUT /users/me` sempre usa o `userId` extraído do JWT, nunca de um path param ou header client-controlled.

---

## 8. Contexto de migração/repositório (específico deste repo)

### 8.1 Ressalva sobre o repo antigo `tech-challenge-users`

`tech-challenge-users` (commitado recentemente na workspace) **não é do mesmo domínio de negócio**: é um sistema de oficina/garagem (`company`, `employee`, `vehicle`, `customer_vehicle`), não usuários de autenticação. **Não portar** `internal/domain/{company,employee,vehicle,customer*}.go` nem os usecases/services associados a esses domínios.

O que **é** reaproveitável dali é só o **esqueleto de repositório** (organização de pastas, ferramentas de qualidade, testes de carga/e2e):

| Do antigo (esqueleto, reaproveitar) | Propósito |
|---|---|
| `k8s/base/`, `k8s/overlays/aws/` | Manifests Kustomize — backend deste serviço é container no EKS. Reaproveitar quase 1:1 (`deployment.yaml`, `service.yaml`, `hpa.yaml`, `kustomization.yaml`), trocando nome/imagem/porta conforme necessário. **Sem `ingress.yaml`** — o roteamento é centralizado em `iac-video-processor-infra`. |
| `test/e2e/` | Estrutura de teste e2e (Go) — adaptar para os 6 endpoints de `/users` e `/users/me` |
| `test/stress/stress-test.js` | Estrutura de teste de carga (k6/similar) — adaptar para o volume de uso self-service + administrativo |
| `sonar-project.properties` | Config de qualidade de código — copiar e renomear `projectKey` |
| `docs/internal-api.md` | Padrão de documentação de API interna — adaptar ao contrato da seção 3 |
| `internal/domain`, `internal/application/{ports,usecases,services}`, `internal/adapter/{http,database,config}` | **Só a organização de camadas**, não o conteúdo — o `user.go`/`services/user.go` de lá tem regras de outro domínio |

### 8.2 Deploy — Kustomize + ECR + credenciais via Kubernetes Secret

**Não há `terraform/` local com `aws_lambda_function`.** O deploy deste serviço é:

1. **Build + push de imagem** pro ECR provisionado em `iac-video-processor-infra` (repositório `video-processor-users-api`), no pipeline de CI deste repo.
2. **`kubectl apply -k k8s/overlays/aws/`** contra o cluster `video-processor-eks` (kubeconfig via `aws eks update-kubeconfig`) — aplica **Deployment, Service, HPA** deste serviço.
3. **Sem `Ingress` próprio** — o roteamento `/users/*` → este `Service` (porta 80) é uma regra de path no `Ingress` **centralizado**, mantido e aplicado por `iac-video-processor-infra` (seção 6.1 daquele spec) — este repo só precisa garantir que o `Service` se chame `video-processor-users-api-svc` na porta 80.
4. **Acesso ao Secrets Manager (`jwt-signing-key`):** via credenciais temporárias da sessão do AWS Academy (`AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`/`AWS_SESSION_TOKEN`, vindas de secrets do GitHub Actions), injetadas como Kubernetes `Secret` e consumidas pelo container via env vars lidas pelo AWS SDK — mesmo padrão já usado por `tech-challenge-users`. Isso evita precisar de IRSA/OIDC provider, que exigiria criar uma IAM role nova — não permitido sob a `LabRole` do Academy (ver `iac-video-processor-infra`, seção 5).
5. **Acesso ao RDS:** via rede (security group do node group liberado para a porta 5432 do RDS), com credenciais de banco (usuário/senha) injetadas também via `Secret`.

### 8.3 Dependências

- Depende de `iac-video-processor-data` (RDS `usersdb` existir, colunas `phone`/`address` migradas por este serviço).
- Depende de `iac-video-processor-infra` (cluster EKS `video-processor-eks`, VPC/subnets, ECR, AWS Load Balancer Controller e o `Ingress` centralizado com a regra `/users` já apontando pro `Service` deste repo, seção 6.1 daquele spec).
- **Não depende de `video-processor-authentication-api`, nem o contrário, em tempo de execução** — desacoplamento total (ADR-011). A única relação indireta é que o seed de bootstrap de `authentication` espera que a migração do schema `users` já tenha rodado — por isso a ordem de implementação continua colocando este serviço antes de `authentication` (spec guarda-chuva, seção 8), mas por esse motivo, não mais por dependência de dado de teste.

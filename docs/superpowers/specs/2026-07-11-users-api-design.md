# Spec — video-processor-users-api

**Data:** 2026-07-11 (atualizado 2026-07-13 — deploy muda de Lambda para container no EKS; renomeado 2026-07-15 — futuro serviço `video-processor-api` passa a se chamar `video-processor-converter`; atualizado 2026-07-16 — Profile puro, self-service progressive profiling, ADR-011; atualizado 2026-07-18 — worker reativo de signup, remoção de `role` do schema, ADR-012; atualizado 2026-07-18 (continuação) — `phone`/`address` removidos do escopo, `PUT /users/me` removido)
**Status:** Draft — pronto para virar plano de implementação
**Repo antigo de referência (esqueleto estrutural apenas, não domínio):** `tech-challenge-users`
**Spec guarda-chuva:** `docs/superpowers/specs/2026-07-11-video-processor-auth-infra-migration-design.md` (workspace raiz), atualizada em 2026-07-18
**RFCs de origem desta revisão:** `RFC_service-authentication.md`, `RFC_service-users.md` (ADR-011); reconciliação 2026-07-18 com `RFC_arquitetura-video-processing.md` (ver `docs/superpowers/specs/2026-07-18-notification-signup-integration-design.md`, ADR-012)

> **Revisão 2026-07-16 — substitui o desenho anterior** (dono de dois armazenamentos — RDS + DynamoDB, com escrita dupla de credencial). `users-service` passa a ser **exclusivamente** o serviço de Profile (RDS): zero acesso a DynamoDB, zero chamada síncrona a `authentication` em qualquer direção (ver spec de `authentication`, ADR-011). O cadastro passa a ser 100% self-service: signup (email/senha) em `authentication`, dados de perfil preenchidos progressivamente aqui via `PUT /users/me`. `POST /users` (criação administrativa de usuário completo) é **removido** — não sobra nenhuma forma de um admin criar um usuário de credencial+perfil de uma vez só, já que todo cadastro passa pelo signup público.
>
> **Revisão 2026-07-18 (ADR-012)** — signup em `authentication` passa a coletar `name`/`document` além de `email`/`password` (perfil completo já no signup). `users-api` ganha um **segundo componente**: um worker que consome o evento `UserSignedUp` (SQS) e cria a linha em `users` reativamente, em vez de esperar o primeiro `PUT /users/me`. Isso **não** reabre chamada síncrona com `authentication` — é uma dependência assíncrona nova, o princípio "zero chamada HTTP síncrona" do ADR-011 continua valendo. A coluna `role` **sai do schema** de `users` — fica exclusiva em `auth-credentials` (DynamoDB, `authentication`), já que só `authentication` precisa dela (para montar o JWT no login) e nenhuma rota deste serviço decide algo com base nela. Ver seções 2, 4.4 (nova) e 6.
>
> **Revisão 2026-07-18 (continuação) — `phone`/`address` saem de escopo; `PUT /users/me` removido.** `name`/`email`/`document` (os únicos campos que chegam via signup + evento `UserSignedUp`) passam a ser os **únicos** dados armazenados para o usuário — não há mais "progressive profiling" nenhum. O worker (seção 4.4) vira o **único escritor** de `users`. Correção pontual de `name`/`document` fica restrita à rota administrativa (`PUT /users/:id`, `role administrator`) — sem rota self-service de escrita nesta fase. Validação de formato de `document` (CPF) migra para `authentication`, no `POST /auth/signup` (único ponto síncrono onde um payload inválido pode ser rejeitado com `400` — ver spec de `authentication`, seção 4).
>
> **Revisão 2026-07-18 (continuação 2) — `GET /users/me` removido, consolidado em `GET /users/:id`.** As duas rotas faziam a mesma coisa (ler um perfil) com autorização diferente. Em vez de manter as duas, `GET /users/:id` passa a aceitar dois casos: `role administrator` (lê qualquer perfil) **ou** dono do recurso (`:id` no path == `userId` do JWT). Uma rota só, uma autorização só (posse OU admin), em vez de duas rotas fazendo a mesma leitura por caminhos diferentes. O frontend decodifica o próprio JWT (payload base64, sem chamada de rede) para saber seu `userId` e montar a URL — trivial, mesmo padrão usado por qualquer client que já precisa ler `role`/`exp` do token localmente.

---

## 1. Responsabilidade

Serviço de **Profile**: dono exclusivo da tabela `users` no RDS PostgreSQL (nome, email, documento). Sem nenhuma dependência síncrona de DynamoDB/`authentication`. Duas superfícies HTTP: leitura por posse-ou-admin (`GET /users/:id` — dono do recurso ou `role administrator`) e administrativa de escrita (edição/remoção de perfis de terceiros, `role administrator`) — mais um **worker** reativo (ADR-012 — consome o evento de signup e é o **único escritor** de `users`, ver seção 4.4).

## 2. Modelo de dados — único armazenamento (RDS), Profile puro

`users-service` só grava em RDS PostgreSQL. `email` é **cópia de leitura** — a fonte de verdade continua em `auth-credentials` (DynamoDB), gerenciada exclusivamente por `authentication`. `users-service` nunca lê nem escreve em DynamoDB.

**Atualizado 2026-07-18 (ADR-012): `role` sai do schema.** `role` fica **exclusiva** em `auth-credentials` — não existe cópia em `users`. Motivo: `authentication` é quem monta o JWT no login e já lê `role` do mesmo item DynamoDB que valida a senha; não há nenhum momento em que precisaria consultar `users-api`. Manter uma cópia em RDS seria uma segunda fonte da mesma informação sem nenhum consumidor real hoje — nenhuma rota deste serviço decide algo com base em `role`. Consequência: `GET /users`, `GET /users/:id` não retornam mais `role`; quem precisa de `role` (front-end, outros serviços) lê do JWT.

**Atualizado 2026-07-18 (continuação): `phone`/`address` saem do schema.** Só `name`/`email`/`document` são armazenados — os únicos campos que chegam via signup + evento `UserSignedUp`. Não há mais "progressive profiling" — o perfil nasce completo (do ponto de vista deste serviço) no momento em que o worker processa o evento.

```go
// users-service/internal/domain/user/user.go
type User struct {
    ID        string `gorm:"type:uuid;primaryKey"`
    Name      string
    Email     string `gorm:"uniqueIndex;not null"`
    Document  string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Repository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    List(ctx context.Context, limit, offset int) ([]User, error)
    Upsert(ctx context.Context, u *User) error // usado exclusivamente pelo worker (seção 4.4) — único escritor reativo de name/email/document
    Update(ctx context.Context, u *User) error // usado por PUT /users/:id [admin] — única rota de escrita HTTP deste serviço
    Delete(ctx context.Context, id string) error // usado por DELETE /users/:id [admin] — só remove o perfil
}
```

**Não contém `password_hash`** — nunca conteve, e agora não há mais nenhum outro dado de credencial neste serviço.

`ID` é o mesmo valor gerado por `authentication` no signup (`userId`). **Atualizado 2026-07-18 (ADR-012)**: a linha em `users` é criada **exclusivamente pelo worker** (seção 4.4), a partir do evento `UserSignedUp` — não existe mais nenhuma rota HTTP que crie a linha (`PUT /users/me` foi removido, ver seção 3).

## 3. Contrato

Autenticação: `Authorization: Bearer <jwt>` em todas as rotas — `users-api` valida o JWT **por conta própria** (ver seção 5.1), não confia em nenhum header repassado pelo gateway. Nenhuma rota de escrita HTTP neste serviço — `PUT`/criação de perfil é exclusividade do worker (seção 4.4); as rotas administrativas abaixo só editam/removem perfis já existentes.

| Method | Endpoint | Autorização | Descrição |
|---|---|---|---|
| GET | `/users/:id` | `role administrator` **OU** dono do recurso (`:id` == `userId` do JWT) | Detalhe do perfil — única rota de leitura deste serviço (substitui o antigo par `GET /users/me` + `GET /users/:id`, ver revisão 2026-07-18 continuação 2) |
| GET | `/users` | `role administrator` | Listagem paginada (`limit` padrão 20, máx 100; `offset`) |
| PUT | `/users/:id` | `role administrator` | Atualização de perfil de terceiro (`name`, `document` — não cria/altera credencial, não toca `auth-credentials`) |
| DELETE | `/users/:id` | `role administrator` | Remove **só** o perfil no RDS — ver seção 4, item 4, sobre a credencial órfã |

Request Payload (`PUT /users/:id`)
```json
{ "name": "John Doe", "document": "652.904.150-84" }
```

**Removido nesta revisão:** `POST /users` (criação administrativa de usuário completo — todo cadastro passa pelo signup público de `authentication`); `GET/PUT /users/me` (leitura consolidada em `GET /users/:id` por posse; escrita self-service eliminada, worker é o único escritor).

**Erros:** `400 INVALID_PAYLOAD`, `401 UNAUTHORIZED`, `403 FORBIDDEN` (não-admin tentando ler o perfil de outro `userId`, ou não-admin em rota administrativa de escrita), `404 USER_NOT_FOUND`.

## 4. Regras de negócio

1. `GET /users/:id`: libera se `role == administrator` (lê qualquer perfil) **ou** se `:id` do path == `userId` extraído do JWT (dono lendo o próprio perfil) — nunca confia em `:id` sozinho sem essa checagem. `userId` do JWT nunca vem de path param/header client-controlled sem essa validação de posse.
2. `PUT /users/:id`, `DELETE /users/:id`: exigem `role administrator` — não há exceção de posse nessas duas (diferente do `GET`, que aceita dono OU admin). `PUT /users/:id` **nunca** altera `email` — não faz parte do payload aceito (ver seção 7, teste de regressão). `role` não existe mais no schema (seção 2), então não é uma preocupação de payload aqui.
3. **Validação de `document` (CPF — dígito verificador) migrou para `authentication`**, em `POST /auth/signup` (único ponto síncrono do sistema onde um payload malformado pode ser rejeitado com `400` antes de qualquer coisa ser persistida — ver spec de `authentication`, seção 4). Este serviço não valida `document` de novo nem no worker (mensagem já validada na origem) nem em `PUT /users/:id` administrativo (assume-se operador de confiança; validação de formato aqui fica como debt técnico se algum dia a rota permitir escrita não confiável — ver seção 9).
4. `DELETE /users/:id`: remove **apenas** a linha em `users` (RDS). **Não chama `authentication`** — a credencial em `auth-credentials` fica órfã (o usuário consegue continuar autenticando, mas sem perfil). Limitação aceita nesta fase (decisão de brainstorming, ADR-011) em troca do desacoplamento total entre os dois serviços; revisitar se isso virar um problema de segurança/produto real (ver spec guarda-chuva, seção 10). Sem cascade delete de `links` (mantido da revisão anterior — links continuam acessíveis por `linkId`).
5. Unicidade de `email` garantida por índice único no RDS (cópia de leitura, mas ainda única).

### 4.4 Worker reativo de signup (novo, ADR-012 — 2026-07-18) — único escritor de `users`

Segundo componente deste repositório: `cmd/worker/main.go`, processo Go de longa duração, mesma imagem Docker do `cmd/api` (HTTP), compartilhando `internal/domain` e `internal/adapter/database` — **não** é uma Lambda (ver seção 6.1, decisão de manter `users-api` 100% container/EKS).

**Fluxo:**
1. Long-poll (`WaitTimeSeconds: 20`) em `video-processor-user-events-queue-${var.environment}` (SQS, provisionada em `iac-video-processor-infra`).
2. Parseia o envelope SNS-em-SQS e o evento `UserSignedUp` (`user_id`, `name`, `email`, `document`, `occurred_at` — sem `role`, ver seção 2).
3. `INSERT INTO users (id, name, email, document) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, email = EXCLUDED.email, document = EXCLUDED.document`. `DO UPDATE` em vez de `DO NOTHING` — idempotente para reentrega *at-least-once* do SQS (reescreve os mesmos valores, sem dano) e defesa em profundidade contra qualquer outro escritor futuro (hoje só `PUT /users/:id` administrativo também escreve `name`/`document`, ver seção 4, item 3).
4. Deleta a mensagem da fila após sucesso.

**Por que não reabre a chamada síncrona do ADR-011:** o worker consome um evento assíncrono publicado por `authentication` — não há chamada HTTP em nenhuma direção entre os dois serviços. O princípio "zero chamada síncrona" continua valendo integralmente.

**Único escritor da criação de perfil (revisão 2026-07-18, continuação):** desde que `GET/PUT /users/me` foram removidos (seção 3), este worker é a **única** forma de uma linha nascer em `users` — não existe mais nenhuma rota HTTP de criação/upsert self-service. `PUT /users/:id` (admin) só edita uma linha já existente, não cria.

## 5. Dependências

- RDS PostgreSQL (tabela `users`: `id`, `name`, `email`, `document`, `created_at`, `updated_at` — sem `password_hash`, `role`, `phone` ou `address`, ver seção 2).
- Secrets Manager: `jwt-signing-key` (mesmo segredo de `authorizer`/`authentication` — novo para este serviço, ver seção 5.1).
- Bibliotecas: `gorm.io/gorm`, `gorm.io/driver/postgres`, `github.com/golang-jwt/jwt/v5` (novo), `github.com/gin-gonic/gin`, `github.com/awslabs/aws-lambda-go-api-proxy/gin` (se aplicável ao adapter HTTP), `github.com/google/uuid`, `github.com/aws/aws-sdk-go-v2/service/sqs` (novo, ADR-012 — só usado pelo `cmd/worker`, não pelo `cmd/api`).

**Removido nesta revisão:** `github.com/aws/aws-sdk-go-v2/service/dynamodb`, `golang.org/x/crypto/bcrypt` — este serviço não toca mais em nenhum dado de credencial nem faz hash de senha.

### 5.1 Validação de JWT própria (decisão de brainstorming, substitui repasse de headers pelo gateway)

A integração `HTTP_PROXY` do `iac-video-processor-gateway` pro ALB não repassa automaticamente o `context` do `authorizer` (isso só acontece "de graça" em integrações Lambda proxy). Em vez de o gateway repassar `userId`/`role` como headers customizados (opção descartada — implicaria confiar cegamente na borda, quebrando o padrão de defesa em profundidade já usado no resto do projeto), `users-api` decodifica e valida o JWT por conta própria, com a mesma lib (`golang-jwt/jwt/v5`) e o mesmo segredo (`jwt-signing-key`, Secrets Manager) que `authorizer`/`authentication` já usam. É redundante com a validação do `authorizer` na borda por design — o `authorizer` continua servindo pra rejeitar requisições sem token válido antes de gastar um hop de rede até o ALB, mas `users-api` nunca confia cegamente nisso.

**IAM**: `secretsmanager:GetSecretValue` no ARN de `jwt-signing-key` — via as mesmas credenciais de sessão/`LabRole` já injetadas no pod (sem recurso Terraform novo, já que `users-api` não tem execution role própria).

## 6. Config de deploy (EKS/container)

**Backend é container no EKS** (ver spec guarda-chuva e `iac-video-processor-infra`, seção 1), atrás do Application Load Balancer compartilhado (ver `iac-video-processor-gateway`, seção 7).

- **Recursos do container**: `cpu: 50m` (request) / `200m` (limit), `memory: 64Mi` (request) / `128Mi` (limit). `replicas: 1` + HPA — uso administrativo/self-service, baixo volume não justifica réplicas altas por padrão.
- **Porta:** `8081`, `livenessProbe`/`readinessProbe` em `/health`.
- **Rede:** o pod precisa alcançar o RDS de `iac-video-processor-data` (porta 5432) via security group do node group.
- **Credenciais AWS no pod (revisado 2026-07-16)**: injetadas via Kubernetes `Secret` (sessão do Academy) **apenas** para `secretsmanager:GetSecretValue` (segredo `jwt-signing-key`, ver seção 5.1) e para as credenciais de banco do RDS (`DB_HOST`/`DB_PASSWORD`, via `Secret` separado). **Sem acesso a DynamoDB ou SNS** — simplificação em relação à revisão anterior, que ainda precisava de credenciais AWS pra escrever em DynamoDB.

**Atualizado 2026-07-18 (ADR-012) — segundo componente, worker:**

```
k8s/base/
├── deployment.yaml          # API HTTP (existente)
├── worker-deployment.yaml   # NOVO — mesma imagem ECR, command diferente (cmd/worker)
├── service.yaml             # só a API HTTP — worker não expõe porta
└── hpa.yaml                 # só a API HTTP — worker fica em replicas: 1 fixo
```

- Worker: mesma imagem Docker, `command`/`args` apontando para o binário `cmd/worker`. Sem `Service`/`Ingress` — não recebe tráfego, só consome SQS.
- `replicas: 1` fixo (sem HPA) — volume de signup não justifica múltiplas réplicas nesta fase.
- Health check: sem endpoint HTTP nesta fase (worker não serve HTTP); mecanismo exato de liveness fica para o plano de implementação (ver seção 9, Questões em Aberto).
- IAM: ganha, via o mesmo `Secret` de sessão AWS Academy, `sqs:ReceiveMessage`/`DeleteMessage`/`GetQueueAttributes` no ARN de `video-processor-user-events-queue-${var.environment}` (provisionada em `iac-video-processor-infra`, ver ADR-012).

## 7. Testes

- Unitário: repositório RDS com `sqlmock` (ou testcontainers-go + Postgres real em CI) + teste de decodificação/validação de JWT.
- Contrato: `httptest` cobrindo os 4 endpoints (`GET /users/:id`, `GET /users`, `PUT /users/:id`, `DELETE /users/:id`) + casos de erro de validação e de autorização — incluindo os dois casos de `GET /users/:id` (dono lendo o próprio `:id`, e admin lendo qualquer `:id`) e o caso negativo (não-dono, não-admin → `403`).
- Teste de regressão: `password_hash` nunca aparece em nenhuma resposta (nem existe no struct); `role` nunca aparece em nenhuma resposta (nem existe no struct, ADR-012); `phone`/`address` não existem no struct nem no schema (ADR-012, continuação); `PUT /users/:id` nunca permite alterar `email` (campo fora do payload aceito); `GET /users/:id` sempre valida posse (`:id` == `userId` do JWT) antes de liberar pra não-admin, nunca confia no `:id` do path sozinho.
- **Novo (ADR-012)**: teste do worker — evento `UserSignedUp` reprocessado (entrega duplicada do SQS) é idempotente, reescrevendo os mesmos valores em `name`/`email`/`document` sem erro; mensagem malformada não trava o worker (loga e segue, ou vai para DLQ conforme `maxReceiveCount`); `INSERT ... ON CONFLICT DO UPDATE` nunca lança erro nem duplica linha em reentregas.
- **Novo (ADR-012)**: `User.ID` nunca é sobrescrito por geração automática. Se o repositório usar um hook `BeforeCreate` do GORM, ele só pode gerar um `ID` quando o campo chegar vazio (`if u.ID == ""`) — nunca incondicionalmente. Na prática, nenhum caminho deste serviço deveria acionar essa branch: o worker (único escritor de criação, seção 4.4) sempre atribui o `ID` a partir do evento `UserSignedUp` antes de persistir, porque a fonte de verdade do identificador é `authentication`, não este serviço (ver ADR-012, discussão sobre geração de `userId`). Teste de regressão: inserir um `User` com `ID` já setado e confirmar que o valor persistido é exatamente o mesmo, não um novo UUID gerado pelo GORM/hook.

---

## 8. Contexto de migração/repositório (específico deste repo)

### 8.1 Ressalva sobre o repo antigo `tech-challenge-users`

`tech-challenge-users` (commitado recentemente na workspace) **não é do mesmo domínio de negócio**: é um sistema de oficina/garagem (`company`, `employee`, `vehicle`, `customer_vehicle`), não usuários de autenticação. **Não portar** `internal/domain/{company,employee,vehicle,customer*}.go` nem os usecases/services associados a esses domínios.

O que **é** reaproveitável dali é só o **esqueleto de repositório** (organização de pastas, ferramentas de qualidade, testes de carga/e2e):

| Do antigo (esqueleto, reaproveitar) | Propósito |
|---|---|
| `k8s/base/`, `k8s/overlays/aws/` | Manifests Kustomize — backend deste serviço é container no EKS. Reaproveitar quase 1:1 (`deployment.yaml`, `service.yaml`, `hpa.yaml`, `kustomization.yaml`), trocando nome/imagem/porta conforme necessário. **Sem `ingress.yaml`** — o roteamento é centralizado em `iac-video-processor-infra`. |
| `test/e2e/` | Estrutura de teste e2e (Go) — adaptar para os 4 endpoints de `/users` (seção 3) |
| `test/stress/stress-test.js` | Estrutura de teste de carga (k6/similar) — adaptar para o volume de uso administrativo + leitura por posse |
| `sonar-project.properties` | Config de qualidade de código — copiar e renomear `projectKey` |
| `docs/internal-api.md` | Padrão de documentação de API interna — adaptar ao contrato da seção 3 |
| `internal/domain`, `internal/application/{ports,usecases,services}`, `internal/adapter/{http,database,config}` | **Só a organização de camadas**, não o conteúdo — o `user.go`/`services/user.go` de lá tem regras de outro domínio |

### 8.2 Deploy — Kustomize + ECR + credenciais via Kubernetes Secret

**Não há `terraform/` local com `aws_lambda_function`.** O deploy deste serviço é:

1. **Build + push de imagem** pro ECR provisionado em `iac-video-processor-infra` (repositório `video-processor-users-api`), no pipeline de CI deste repo.
2. **`kubectl apply -k k8s/overlays/aws/`** contra o cluster `video-processor-eks` (kubeconfig via `aws eks update-kubeconfig`) — aplica **Deployment, Service, HPA** da API HTTP, e (ADR-012) o segundo `Deployment` do worker (`worker-deployment.yaml`, sem `Service`/`HPA`, ver seção 6).
3. **Sem `Ingress` próprio** — o roteamento `/users/*` → este `Service` (porta 80) é uma regra de path no `Ingress` **centralizado**, mantido e aplicado por `iac-video-processor-infra` (seção 6.1 daquele spec) — este repo só precisa garantir que o `Service` se chame `video-processor-users-api-svc` na porta 80.
4. **Acesso ao Secrets Manager (`jwt-signing-key`):** via credenciais temporárias da sessão do AWS Academy (`AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`/`AWS_SESSION_TOKEN`, vindas de secrets do GitHub Actions), injetadas como Kubernetes `Secret` e consumidas pelo container via env vars lidas pelo AWS SDK — mesmo padrão já usado por `tech-challenge-users`. Isso evita precisar de IRSA/OIDC provider, que exigiria criar uma IAM role nova — não permitido sob a `LabRole` do Academy (ver `iac-video-processor-infra`, seção 5).
5. **Acesso ao RDS:** via rede (security group do node group liberado para a porta 5432 do RDS), com credenciais de banco (usuário/senha) injetadas também via `Secret`.

### 8.3 Dependências

- Depende de `iac-video-processor-data` (RDS `usersdb` existir, schema `id`/`name`/`email`/`document`/`created_at`/`updated_at` migrado por este serviço — **sem** `role`, `phone` ou `address`, ADR-012).
- Depende de `iac-video-processor-infra` (cluster EKS `video-processor-eks`, VPC/subnets, ECR, AWS Load Balancer Controller e o `Ingress` centralizado com a regra `/users` já apontando pro `Service` deste repo, seção 6.1 daquele spec; **e a fila `video-processor-user-events-queue-${var.environment}` existir, para o worker funcionar — ADR-012**).
- **Continua sem nenhuma chamada HTTP síncrona com `video-processor-authentication-api`, em qualquer direção** — o princípio do ADR-011 é mantido. **Atualizado 2026-07-18 (ADR-012)**: ganha uma dependência **assíncrona** nova — o worker (seção 4.4) só cria a linha em `users` depois que `authentication` publicar `UserSignedUp`; é uma relação *eventually consistent*, não uma chamada de rede direta entre os dois serviços. A relação indireta pré-existente (o seed de bootstrap de `authentication` espera o schema `users` já migrado) continua valendo, e por isso a ordem de implementação continua colocando este serviço antes de `authentication` (spec guarda-chuva, seção 8).

## 9. Questões em Aberto (ADR-012, 2026-07-18)

- Mecanismo de liveness/readiness do `Deployment` do worker — sem endpoint HTTP nesta fase (não serve tráfego). Avaliar `exec` probe com arquivo de heartbeat, ou aceitar rodar sem probe nesta fase (worker de baixo volume). Decidir no plano de implementação.
- Se, no futuro, algum consumidor precisar filtrar/exibir `role` a partir deste serviço, avaliar nesse momento se ainda faz sentido mantê-la exclusiva em `auth-credentials` ou introduzir uma cópia de leitura — não antecipar essa necessidade agora (YAGNI, ver spec `2026-07-18-notification-signup-integration-design.md`).
- **(continuação 2026-07-18)** Sem nenhuma rota self-service de escrita, um usuário que errou `name`/`document` no signup depende de um `administrator` chamar `PUT /users/:id` pra corrigir. Avaliar se isso é aceitável pro produto ou se vale reintroduzir uma rota de auto-correção pontual no futuro — não bloqueante nesta fase.
- **(continuação 2026-07-18)** `PUT /users/:id` administrativo não revalida formato de `document` (a validação de CPF vive em `authentication`, no signup — seção 4, item 3). Se um admin conseguir gravar um `document` malformado via essa rota, não há rejeição nesta fase — avaliar se vale duplicar a validação aqui como defesa em profundidade.

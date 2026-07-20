# Spec — Destravar deploy de `video-processor-users-api` no EKS (rota `/users`)

**Data:** 2026-07-19
**Status:** Draft — pronto para virar plano de implementação
**Spec guarda-chuva:** `iac-video-processor-infra/docs/superpowers/specs/2026-07-11-infra-design.md` (seções 6, 6.1, 7)
**Spec relacionada:** `docs/superpowers/specs/2026-07-11-users-api-design.md` (seção 6, deploy EKS/container)
**Contexto:** conta AWS Academy Learner Lab vazia (sessão reiniciada) — todo o ambiente `prod` precisa ser recriado do zero para testar o fluxo signup → verify → login → `GET /users/:id`. Ao verificar a ordem de deploy, dois gaps foram encontrados entre o que a spec guarda-chuva descreve e o que existe hoje nos repos: (1) `video-processor-users-api` não tem nenhum manifest Kubernetes, apesar da spec dizer "existente"; (2) o JWT signing key é lido de uma env var texto-puro (`JWT_SECRET`), não buscado do Secrets Manager em runtime como o resto do domínio (`authorizer`, `authentication-api`) já faz.

---

## 1. Objetivo

Destravar a rota `ANY /users/{proxy+}` do `iac-video-processor-gateway`, que hoje falha porque `data "aws_lb" "eks_alb"` não encontra nenhum ALB com a tag `video-processor/alb=unified` — não existe Load Balancer Controller instalado, nem Ingress aplicado, nem `Deployment`/`Service` do `users-api` no cluster. Sem isso, o último passo do teste manual (`GET /users/:id` via API Gateway) não tem como funcionar, independente da ordem de `terraform apply`.

Escopo desta spec é só o que falta para essa rota funcionar em `prod`; não cobre `dev`/LocalStack (a spec guarda-chuva já documenta que Helm/Ingress só existem no pipeline de `prod`, seção 6).

## 2. Fora de escopo

- Automatizar isso em pipeline CI (`.github/workflows/infra-pipeline.yml`) — a spec guarda-chuva já descreve esse workflow, mas para o teste manual desta sessão os comandos equivalentes (`kubectl`/`helm`) são rodados diretamente no terminal, usando a sessão AWS já configurada em `~/.aws/credentials`. Criar o workflow real fica para depois.
- `video-processor-link-api` — a regra `/links` do Ingress centralizado (`iac-video-processor-infra/k8s/ingress.yaml`) referencia um `Service` que não existe (`video-processor-link-api-svc`); isso é aceito como está, não bloqueia a criação do ALB nem a regra `/users`.
- Qualquer mudança em `iac-video-processor-gateway` ou `video-processor-authorizer`/`authentication-api` — já cobertos pela spec de ADR-013 e sem gaps encontrados.

## 3. Componentes

### 3.1 AWS Load Balancer Controller + Ingress (`iac-video-processor-infra`, execução manual)

Sem mudança de código no repo (o `k8s/ingress.yaml` já existe e já está correto). Nesta sessão, executar diretamente:

1. `aws eks update-kubeconfig --region us-east-1 --name video-processor-eks-prod`
2. Criar `Secret` `aws-alb-credentials` em `kube-system` a partir das credenciais atuais de `~/.aws/credentials` (perfil `default`)
3. `helm repo add eks https://aws.github.io/eks-charts && helm repo update`
4. `helm upgrade --install aws-load-balancer-controller eks/aws-load-balancer-controller -n kube-system --set clusterName=video-processor-eks-prod --set serviceAccount.create=true --set region=us-east-1 --set vpcId=<vpc id> --set "envFrom[0].secretRef.name=aws-alb-credentials" --wait` (mesmo padrão da spec guarda-chuva seção 6 — sem IRSA/OIDC, credenciais de sessão via env)
5. `kubectl apply -f k8s/ingress.yaml`

### 3.2 `video-processor-users-api` — busca do JWT signing key via Secrets Manager (código Go)

Hoje `internal/adapter/config/config.go` lê `JWT_SECRET` como env var texto-puro e `middleware.AuthRequired(jwtSecret string)` recebe esse valor direto — diferente do padrão já validado em `video-processor-authorizer/cmd/authorizer/main.go` e `video-processor-authentication-api/cmd/api/main.go` (busca única no startup, via `secretsmanager.GetSecretValue`, nunca por request).

Mudança, espelhando esse padrão exato:

- `cmd/api/main.go`: antes de montar o router, carregar `aws-sdk-go-v2/config` (`awsconfig.LoadDefaultConfig`), buscar o secret cujo nome vem de `JWT_SIGNING_KEY_SECRET_NAME` (novo env var), e passar o `[]byte` resultante para o middleware.
- `internal/adapter/config/config.go`: `JWTConfig.Secret` (env var `JWT_SECRET`) é substituído por `JWTConfig.SigningKeySecretName` (env var `JWT_SIGNING_KEY_SECRET_NAME`). Os campos `AdminUser`/`AuthConfig.ServiceURL` não são usados por este fluxo e não são alterados nesta spec.
- `internal/adapter/http/middleware/auth.go`: assinatura de `AuthRequired` não muda (já recebe `jwtSecret []byte`/`string` resolvido) — só quem chama passa a vir do fetch acima em vez do env var.
- Credenciais AWS para o SDK chamar Secrets Manager: injetadas via `Secret` do Kubernetes (`aws-session-credentials`, ver 3.3) — mesmo padrão descrito na spec guarda-chuva seção 5 (sem IRSA).

Banco de dados **não muda**: a spec já define que `DB_HOST`/`DB_USER`/`DB_PASSWORD`/`DB_NAME` chegam prontos via `Secret` separado (não fetch em runtime), o que já bate com `config.go` atual.

### 3.3 `video-processor-users-api` — manifests Kubernetes (novos)

```
k8s/base/deployment.yaml         # API (cmd/api), porta 8081, cpu 50m/200m, mem 64Mi/128Mi,
                                  # livenessProbe/readinessProbe em /health, replicas: 1
k8s/base/worker-deployment.yaml  # worker (cmd/worker), mesma imagem ECR, command diferente,
                                  # replicas: 1 fixo (sem HPA), sem probes/Service
k8s/base/service.yaml            # video-processor-users-api-svc, porta 80 -> targetPort 8081
k8s/base/hpa.yaml                # só a API (worker fica fixo)
k8s/base/kustomization.yaml
k8s/overlays/aws/kustomization.yaml   # aplicado via `kubectl apply -k k8s/overlays/aws/`
```

`Secret`s consumidos pelos dois `Deployment`s, criados manualmente nesta sessão (não versionados no repo):

- `aws-session-credentials`: `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`/`AWS_SESSION_TOKEN`/`AWS_REGION`, extraídos da sessão atual (`~/.aws/credentials`).
- `users-api-db-credentials`: `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME`, extraídos uma vez do secret mestre do RDS (`rds_master_user_secret_arn`, output de `iac-video-processor-data`) + `rds_endpoint` (mesmo output).

Env vars não-secretas: `JWT_SIGNING_KEY_SECRET_NAME=jwt-signing-key-prod` (só na API), `SQS_QUEUE_URL=<url da fila video-processor-user-events-queue-prod>` (só no worker).

## 4. Fluxo de dados

```
kubectl apply -k k8s/overlays/aws/
  -> Deployment (API)   -> pod: fetch JWT key (Secrets Manager, aws-session-credentials)
                         -> pod: connect RDS (users-api-db-credentials)
                         -> Service :80 -> :8081
  -> Deployment (worker) -> pod: connect RDS + SQS (aws-session-credentials, users-api-db-credentials)
  -> HPA (API apenas)

Ingress (iac-video-processor-infra/k8s/ingress.yaml)
  -> AWS Load Balancer Controller cria/gerencia o ALB interno (tag video-processor/alb=unified)
  -> regra /users -> Service video-processor-users-api-svc:80

iac-video-processor-gateway
  -> data "aws_lb" "eks_alb" (por tag) resolve o ALB acima
  -> rota ANY /users/{proxy+} -> VPC Link -> ALB -> Service -> pod da API
```

## 5. Tratamento de erro

- Fetch do JWT signing key falha no startup da API (`cmd/api/main.go`) → `log.Fatalf`, mesmo padrão de `authorizer`/`authentication-api` — o pod não fica pronto (`readinessProbe` nunca passa a reportar OK porque o processo nem chega a subir o servidor HTTP), o `Deployment` fica em `CrashLoopBackOff` visível via `kubectl get pods`, sem mascarar a falha.
- `aws-session-credentials` expirado (sessão Academy tem TTL) → chamadas ao Secrets Manager/SQS falham com erro de autenticação da AWS SDK; fora de escopo desta spec resolver renovação automática — é um problema operacional conhecido do ambiente Academy, mitigado recriando o `Secret` manualmente se a sessão for renovada.

## 6. Testes

- Sem teste automatizado novo para os manifests Kubernetes em si (não há framework de teste de manifest neste repo hoje).
- Mudança em `cmd/api/main.go`/`config.go`: cobertura existente de `config_test.go` precisa ser atualizada para refletir `JWTSigningKeySecretName` no lugar de `Secret` (remove o teste que asserta `cfg.JWT.Secret` vazio por default, adiciona equivalente para o novo campo). Teste do fetch em si (chamada real ao Secrets Manager) não é unitário — validado na prática pelo teste manual end-to-end desta sessão (signup → verify → login → `GET /users/:id`).
- Validação final é o próprio teste manual do fluxo completo via API Gateway, não um teste automatizado desta spec.

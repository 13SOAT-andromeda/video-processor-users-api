# Destravar deploy do users-api no EKS (rota /users) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fazer a rota `ANY /users/{proxy+}` do `iac-video-processor-gateway` funcionar de ponta a ponta, pra permitir o teste manual completo (signup → verify → login → `GET /users/:id`) contra a conta AWS Academy Learner Lab (`prod`, `us-east-1`).

**Architecture:** `video-processor-users-api` passa a buscar o JWT signing key do Secrets Manager em runtime (mesmo padrão de `authorizer`/`authentication-api`) em vez de ler `JWT_SECRET` como env var texto-puro; ganha manifests Kubernetes (Deployment da API + Deployment do worker + Service + HPA, via kustomize) que hoje não existem; e o Dockerfile passa a empacotar os dois binários (`cmd/api` e `cmd/worker`) na mesma imagem. Em paralelo, o AWS Load Balancer Controller é instalado manualmente no cluster (`iac-video-processor-infra`) e o Ingress centralizado já existente (`k8s/ingress.yaml`) é aplicado, criando o ALB que o gateway procura pela tag `video-processor/alb=unified`.

**Tech Stack:** Go 1.25, Gin, `aws-sdk-go-v2` (`config`, `secretsmanager`), Docker multi-stage build, Kubernetes (kustomize), Helm (AWS Load Balancer Controller), Terraform (só leitura de outputs, sem mudança de `.tf` nesta spec).

## Global Constraints

- Ambiente fixo: `prod`, região `us-east-1`, namespace Kubernetes `default` (mesmo namespace do `Ingress` existente).
- Sem IRSA/OIDC — credenciais AWS chegam nos pods só via `Secret` do Kubernetes com as credenciais de sessão atuais (`~/.aws/credentials`, perfil `default`), nunca via `ServiceAccount` anotado.
- Nome do secret do JWT signing key é fixo por convenção Terraform: `jwt-signing-key-prod` (`aws_secretsmanager_secret.jwt_signing_key`, `iac-video-processor-infra`).
- Nenhuma mudança em `iac-video-processor-gateway`, `video-processor-authorizer` ou `video-processor-authentication-api` — fora de escopo desta spec.
- Pré-requisito (fora desta plan, deve já estar feito antes de começar a Task 1): `terraform apply` em `iac-video-processor-infra/prod` (cria VPC, cluster EKS, ECR, secret `jwt-signing-key-prod`, filas SQS) e em `iac-video-processor-data/prod` (cria RDS) já rodaram com sucesso.

---

### Task 1: AWS Load Balancer Controller + Ingress centralizado (`iac-video-processor-infra`, operacional)

**Files:** nenhuma — só comandos de terminal, nenhum arquivo do repo muda (`k8s/ingress.yaml` já existe e já está correto).

**Interfaces:**
- Consome: outputs Terraform `vpc_id`, `cluster_name` de `iac-video-processor-infra/prod` (já existem, `outputs.tf`).
- Produz: um ALB interno com a tag `video-processor/alb=unified`, consumido pela Task 6 (verificação end-to-end) e pelo `data "aws_lb" "eks_alb"` de `iac-video-processor-gateway`.

- [ ] **Step 1: Configurar o kubeconfig local para o cluster**

```bash
cd /home/juliovaz/workspaces/video-processor-hackathon/iac-video-processor-infra/prod
CLUSTER_NAME=$(terraform output -raw cluster_name)
aws eks update-kubeconfig --region us-east-1 --name "$CLUSTER_NAME"
kubectl get nodes
```
Expected: pelo menos 1 node em `Ready`.

- [ ] **Step 2: Criar o secret com as credenciais de sessão atuais em `kube-system`**

```bash
CREDS=$(aws configure export-credentials --profile default)
kubectl create secret generic aws-alb-credentials \
  -n kube-system \
  --from-literal=AWS_ACCESS_KEY_ID="$(echo "$CREDS" | jq -r .AccessKeyId)" \
  --from-literal=AWS_SECRET_ACCESS_KEY="$(echo "$CREDS" | jq -r .SecretAccessKey)" \
  --from-literal=AWS_SESSION_TOKEN="$(echo "$CREDS" | jq -r .SessionToken)" \
  --dry-run=client -o yaml | kubectl apply -f -
```
Expected: `secret/aws-alb-credentials configured` (ou `created`).

- [ ] **Step 3: Instalar o AWS Load Balancer Controller via Helm**

```bash
VPC_ID=$(terraform output -raw vpc_id)
CLUSTER_NAME=$(terraform output -raw cluster_name)

helm repo add eks https://aws.github.io/eks-charts
helm repo update eks

helm upgrade --install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName="$CLUSTER_NAME" \
  --set serviceAccount.create=true \
  --set region=us-east-1 \
  --set vpcId="$VPC_ID" \
  --set "envFrom[0].secretRef.name=aws-alb-credentials" \
  --wait
```
Expected: `STATUS: deployed`. Verificar com `kubectl get deployment -n kube-system aws-load-balancer-controller` → `READY 2/2` (ou `1/1`, dependendo do node disponível).

- [ ] **Step 4: Aplicar o Ingress centralizado**

```bash
kubectl apply -f /home/juliovaz/workspaces/video-processor-hackathon/iac-video-processor-infra/k8s/ingress.yaml
kubectl get ingress video-processor-ingress -n default -w
```
Expected: coluna `ADDRESS` preenche com um hostname `*.elb.amazonaws.com` em até ~2-3 minutos (`Ctrl+C` depois de ver o endereço — a regra `/links` pode gerar um warning de backend ausente nos eventos do Ingress, isso é esperado e não bloqueia o ALB nem a regra `/users`, ver spec seção 2 "Fora de escopo").

- [ ] **Step 5: Confirmar o ALB pela tag que o gateway usa**

Sem assumir o padrão de nome que o controller gera — busca por tag em todos os ALBs da conta:

```bash
for arn in $(aws elbv2 describe-load-balancers --query "LoadBalancers[].LoadBalancerArn" --output text); do
  match=$(aws elbv2 describe-tags --resource-arns "$arn" \
    --query "TagDescriptions[0].Tags[?Key=='video-processor/alb' && Value=='unified']" \
    --output text)
  if [ -n "$match" ]; then
    echo "ALB encontrado: $arn"
  fi
done
```
Expected: imprime exatamente um `ALB encontrado: arn:aws:elasticloadbalancing:...`.

Sem commit nesta task — nenhuma mudança de código, só operações no cluster/AWS.

---

### Task 2: `JWTConfig` — nome do secret em vez de segredo texto-puro

**Files:**
- Modify: `internal/adapter/config/config.go`
- Test: `internal/adapter/config/config_test.go`

**Interfaces:**
- Produz: `config.JWTConfig{ SigningKeySecretName string }`, consumido pela Task 3 (`cmd/api/main.go`).

- [ ] **Step 1: Escrever o teste falhando**

Substituir o conteúdo de `internal/adapter/config/config_test.go`:

```go
package config_test

import (
	"os"
	"testing"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/config"
	"github.com/stretchr/testify/assert"
)

func TestInit_Defaults(t *testing.T) {
	// unset any existing vars that might interfere
	os.Unsetenv("DB_HOST")
	os.Unsetenv("JWT_SIGNING_KEY_SECRET_NAME")
	os.Unsetenv("DD_AGENT_HOST")

	cfg, err := config.Init()
	assert.NoError(t, err)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "", cfg.JWT.SigningKeySecretName)
	assert.True(t, cfg.DogStatsD.Disabled)
}

func TestInit_WithEnvVars(t *testing.T) {
	os.Setenv("DB_HOST", "myhost")
	os.Setenv("JWT_SIGNING_KEY_SECRET_NAME", "jwt-signing-key-prod")
	os.Setenv("HTTP_PORT", "9090")
	os.Setenv("AUTH_SERVICE_URL", "http://auth:8081")
	os.Setenv("DD_AGENT_HOST", "dd-agent")
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("JWT_SIGNING_KEY_SECRET_NAME")
		os.Unsetenv("HTTP_PORT")
		os.Unsetenv("AUTH_SERVICE_URL")
		os.Unsetenv("DD_AGENT_HOST")
	}()

	cfg, err := config.Init()
	assert.NoError(t, err)
	assert.Equal(t, "myhost", cfg.Database.Host)
	assert.Equal(t, "jwt-signing-key-prod", cfg.JWT.SigningKeySecretName)
	assert.Equal(t, "9090", cfg.Http.Port)
	assert.Equal(t, "http://auth:8081", cfg.Auth.ServiceURL)
	assert.False(t, cfg.DogStatsD.Disabled)
}
```

- [ ] **Step 2: Rodar o teste e confirmar que falha**

```bash
cd /home/juliovaz/workspaces/video-processor-hackathon/video-processor-users-api
go test ./internal/adapter/config/... -v
```
Expected: FAIL — `cfg.JWT.SigningKeySecretName undefined (type *config.JWTConfig has no field or method SigningKeySecretName)`.

- [ ] **Step 3: Implementar a mudança**

Em `internal/adapter/config/config.go`, trocar:

```go
type JWTConfig struct {
	Secret string
}
```
por:
```go
type JWTConfig struct {
	SigningKeySecretName string
}
```

E trocar, dentro de `Init()`:
```go
	jwt := &JWTConfig{
		Secret: getEnv("JWT_SECRET", ""),
	}
```
por:
```go
	jwt := &JWTConfig{
		SigningKeySecretName: getEnv("JWT_SIGNING_KEY_SECRET_NAME", ""),
	}
```

- [ ] **Step 4: Rodar o teste e confirmar que passa**

```bash
go test ./internal/adapter/config/... -v
```
Expected: `PASS`, `TestInit_Defaults` e `TestInit_WithEnvVars` verdes.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/config/config.go internal/adapter/config/config_test.go
git commit -m "feat: JWTConfig busca nome do secret, não o segredo em texto-puro"
```

---

### Task 3: Buscar o JWT signing key do Secrets Manager no startup e propagar pro router

**Files:**
- Modify: `cmd/api/main.go`
- Modify: `internal/adapter/http/router.go`
- Modify: `test/e2e/users_test.go`
- Modify: `go.mod`, `go.sum` (via `go get`)

**Interfaces:**
- Consome: `cfg.JWT.SigningKeySecretName` (Task 2).
- Produz: `httpAdapter.NewRouter(cfg config.Config, logger *zap.Logger, userHandler handlers.UserHandler, jwtSecret string) *Router` — novo 4º parâmetro, consumido só dentro deste task (os dois call sites existentes).

- [ ] **Step 1: Adicionar a dependência do SDK do Secrets Manager**

```bash
cd /home/juliovaz/workspaces/video-processor-hackathon/video-processor-users-api
go get github.com/aws/aws-sdk-go-v2/service/secretsmanager@v1.43.1
go mod tidy
```
Expected: `go.mod` ganha uma linha direta `github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.43.1` e `github.com/aws/aws-sdk-go-v2/config` deixa de ser `// indirect`.

- [ ] **Step 2: Mudar a assinatura de `NewRouter` em `internal/adapter/http/router.go`**

Trocar:
```go
func NewRouter(cfg config.Config, logger *zap.Logger, userHandler handlers.UserHandler) *Router {
```
por:
```go
func NewRouter(cfg config.Config, logger *zap.Logger, userHandler handlers.UserHandler, jwtSecret string) *Router {
```

E trocar as duas ocorrências de `middleware.AuthRequired(cfg.JWT.Secret)` (linhas do grupo `authed` e do grupo `admin`) por `middleware.AuthRequired(jwtSecret)`.

- [ ] **Step 3: Buscar o secret no `cmd/api/main.go` antes de montar o router**

Adicionar aos imports de `cmd/api/main.go`:
```go
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
```

Trocar o trecho:
```go
	ctx := context.Background()
	db, err := database.Init(ctx, *cfg.Database)
	if err != nil {
		sugar.Fatalf("failed to connect database: %v", err)
	}
```
por:
```go
	ctx := context.Background()

	// JWT signing key é buscado uma vez aqui, antes do servidor começar a
	// aceitar requisições — nunca dentro de um handler por request — mesmo
	// padrão de video-processor-authorizer/cmd/authorizer/main.go e
	// video-processor-authentication-api/cmd/api/main.go.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		sugar.Fatalf("failed to load AWS config: %v", err)
	}
	secretsClient := secretsmanager.NewFromConfig(awsCfg)
	secretValue, err := secretsClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &cfg.JWT.SigningKeySecretName,
	})
	if err != nil {
		sugar.Fatalf("failed to load jwt signing key: %v", err)
	}
	jwtSecret := *secretValue.SecretString

	db, err := database.Init(ctx, *cfg.Database)
	if err != nil {
		sugar.Fatalf("failed to connect database: %v", err)
	}
```

E trocar:
```go
	router := httpAdapter.NewRouter(*cfg, logger, *userHandler)
```
por:
```go
	router := httpAdapter.NewRouter(*cfg, logger, *userHandler, jwtSecret)
```

- [ ] **Step 4: Atualizar o e2e test pra nova assinatura**

Em `test/e2e/users_test.go`, dentro de `setupRouter`, trocar:
```go
	cfg := config.Config{
		Env:  "test",
		Http: &config.HttpConfig{AllowedOrigins: []string{"*"}, Port: "8080"},
		JWT:  &config.JWTConfig{Secret: testJWTSecret},
	}

	router := httpAdapter.NewRouter(cfg, zap.NewNop(), *handler)
```
por:
```go
	cfg := config.Config{
		Env:  "test",
		Http: &config.HttpConfig{AllowedOrigins: []string{"*"}, Port: "8080"},
		JWT:  &config.JWTConfig{},
	}

	router := httpAdapter.NewRouter(cfg, zap.NewNop(), *handler, testJWTSecret)
```

- [ ] **Step 5: Compilar e rodar a suíte inteira**

```bash
go build ./... && go test ./... -v
```
Expected: build limpo, todos os testes `PASS` (inclusive `test/e2e/users_test.go`, que continua usando `testJWTSecret` pra gerar/validar tokens — só a origem do valor dentro do router mudou).

- [ ] **Step 6: Commit**

```bash
git add cmd/api/main.go internal/adapter/http/router.go test/e2e/users_test.go go.mod go.sum
git commit -m "feat: buscar JWT signing key do Secrets Manager no startup da API"
```

---

### Task 4: Dockerfile — empacotar o binário do worker na imagem de produção

**Files:**
- Modify: `Dockerfile`

**Interfaces:**
- Produz: imagem com `/app/main` (API, `cmd/api`, entrypoint padrão) e `/app/worker` (worker, `cmd/worker`, entrypoint via override de `command` no Deployment da Task 5).

- [ ] **Step 1: Adicionar o build do worker no estágio `production_builder`**

Em `Dockerfile`, logo depois de:
```dockerfile
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /usr/local/bin/main ./cmd/api
```
adicionar:
```dockerfile
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /usr/local/bin/worker ./cmd/worker
```

- [ ] **Step 2: Copiar o binário do worker no estágio `production`**

Trocar:
```dockerfile
COPY --from=production_builder /usr/local/bin/main /app/main
COPY --from=production_builder /app/swagger /app/swagger

RUN chmod +x /app/main
```
por:
```dockerfile
COPY --from=production_builder /usr/local/bin/main /app/main
COPY --from=production_builder /usr/local/bin/worker /app/worker
COPY --from=production_builder /app/swagger /app/swagger

RUN chmod +x /app/main /app/worker
```

- [ ] **Step 3: Buildar a imagem localmente e verificar os dois binários**

```bash
cd /home/juliovaz/workspaces/video-processor-hackathon/video-processor-users-api
docker build --target production -t video-processor-users-api:local-test .
docker run --rm --entrypoint sh video-processor-users-api:local-test -c "ls -la /app/main /app/worker"
```
Expected: os dois arquivos listados, ambos com permissão de execução (`-rwxr-xr-x`).

- [ ] **Step 4: Commit**

```bash
git add Dockerfile
git commit -m "build: empacotar binário do worker na imagem de produção"
```

---

### Task 5: Manifests Kubernetes (`k8s/base` + `k8s/overlays/aws`)

**Files:**
- Create: `k8s/base/deployment.yaml`
- Create: `k8s/base/worker-deployment.yaml`
- Create: `k8s/base/service.yaml`
- Create: `k8s/base/hpa.yaml`
- Create: `k8s/base/kustomization.yaml`
- Create: `k8s/overlays/aws/kustomization.yaml`

**Interfaces:**
- Consome: `Secret`s `aws-session-credentials` e `users-api-db-credentials` (criados na Task 6, não versionados), env var não-secreta `JWT_SIGNING_KEY_SECRET_NAME=jwt-signing-key-prod` (fixa, ver Global Constraints).
- Produz: `Service` `video-processor-users-api-svc` porta 80, consumido pelo `Ingress` já existente em `iac-video-processor-infra/k8s/ingress.yaml` (Task 1) e verificado na Task 6.

- [ ] **Step 1: `k8s/base/deployment.yaml`**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: video-processor-users-api
  namespace: default
  labels:
    app: video-processor-users-api
spec:
  replicas: 1
  selector:
    matchLabels:
      app: video-processor-users-api
  template:
    metadata:
      labels:
        app: video-processor-users-api
    spec:
      containers:
        - name: users-api
          image: video-processor-users-api:latest
          ports:
            - containerPort: 8080
          env:
            - name: ENV
              value: production
            - name: JWT_SIGNING_KEY_SECRET_NAME
              value: jwt-signing-key-prod
            - name: DB_SSLMODE
              value: require
          envFrom:
            - secretRef:
                name: aws-session-credentials
            - secretRef:
                name: users-api-db-credentials
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
          livenessProbe:
            httpGet:
              path: /api/health
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 15
          readinessProbe:
            httpGet:
              path: /api/health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
```

- [ ] **Step 2: `k8s/base/worker-deployment.yaml`**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: video-processor-users-api-worker
  namespace: default
  labels:
    app: video-processor-users-api-worker
spec:
  replicas: 1
  selector:
    matchLabels:
      app: video-processor-users-api-worker
  template:
    metadata:
      labels:
        app: video-processor-users-api-worker
    spec:
      containers:
        - name: users-api-worker
          image: video-processor-users-api:latest
          command: ["/app/worker"]
          env:
            - name: DB_SSLMODE
              value: require
          envFrom:
            - secretRef:
                name: aws-session-credentials
            - secretRef:
                name: users-api-db-credentials
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
```

`SQS_QUEUE_URL` não entra aqui — é setado depois do apply, na Task 6 (`kubectl set env`), porque o valor depende do account ID da sessão Academy atual (não é conhecido em tempo de commit, ver spec seção 5).

- [ ] **Step 3: `k8s/base/service.yaml`**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: video-processor-users-api-svc
  namespace: default
  labels:
    app: video-processor-users-api
spec:
  type: ClusterIP
  selector:
    app: video-processor-users-api
  ports:
    - port: 80
      targetPort: 8080
      protocol: TCP
```

- [ ] **Step 4: `k8s/base/hpa.yaml`**

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: video-processor-users-api
  namespace: default
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: video-processor-users-api
  minReplicas: 1
  maxReplicas: 3
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

- [ ] **Step 5: `k8s/base/kustomization.yaml`**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
  - worker-deployment.yaml
  - service.yaml
  - hpa.yaml
```

- [ ] **Step 6: `k8s/overlays/aws/kustomization.yaml`**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
images:
  - name: video-processor-users-api
    newName: video-processor-users-api
    newTag: latest
```

`newName`/`newTag` ficam apontando pra imagem local até a Task 6 rodar `kustomize edit set image` com a URL real do ECR.

- [ ] **Step 7: Validar que o kustomize builda sem erro**

```bash
cd /home/juliovaz/workspaces/video-processor-hackathon/video-processor-users-api
kubectl kustomize k8s/overlays/aws | head -5
```
Expected: YAML válido impresso (sem erro de parsing/referência).

- [ ] **Step 8: Commit**

```bash
git add k8s/
git commit -m "feat: manifests Kubernetes (Deployment API + worker, Service, HPA) via kustomize"
```

---

### Task 6: Build, push, secrets do cluster e deploy (operacional)

**Files:** nenhuma mudança de código — `k8s/overlays/aws/kustomization.yaml` é editado em disco pelo comando do Step 3, mas não precisa ser commitado (aponta pra imagem publicada, estado mutável de deploy).

**Interfaces:**
- Consome: `Service` `video-processor-users-api-svc` (Task 5), `Ingress`/ALB (Task 1), outputs Terraform `users_api_ecr_repository_url` (`iac-video-processor-infra`) e `rds_endpoint`/`rds_master_user_secret_arn` (`iac-video-processor-data`), output `user_events_queue_arn`/nome da fila (`iac-video-processor-infra`).
- Produz: pods `Running`/`Ready` da API e do worker — pré-condição para a Task #5 da lista geral de tarefas ("Executar fluxo de teste end-to-end"), que testa `GET /users/:id` através do API Gateway (fora do escopo desta plan).

- [ ] **Step 1: Build e push da imagem pro ECR**

```bash
cd /home/juliovaz/workspaces/video-processor-hackathon/iac-video-processor-infra/prod
ECR_URL=$(terraform output -raw users_api_ecr_repository_url)

cd /home/juliovaz/workspaces/video-processor-hackathon/video-processor-users-api
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin "${ECR_URL%%/*}"
docker build --target production -t "$ECR_URL:latest" .
docker push "$ECR_URL:latest"
```
Expected: push concluído, `docker images` mostra a tag `latest` local igual ao `$ECR_URL`.

- [ ] **Step 2: Criar o secret `aws-session-credentials` no namespace `default`**

```bash
CREDS=$(aws configure export-credentials --profile default)
kubectl create secret generic aws-session-credentials \
  -n default \
  --from-literal=AWS_ACCESS_KEY_ID="$(echo "$CREDS" | jq -r .AccessKeyId)" \
  --from-literal=AWS_SECRET_ACCESS_KEY="$(echo "$CREDS" | jq -r .SecretAccessKey)" \
  --from-literal=AWS_SESSION_TOKEN="$(echo "$CREDS" | jq -r .SessionToken)" \
  --from-literal=AWS_REGION=us-east-1 \
  --dry-run=client -o yaml | kubectl apply -f -
```

- [ ] **Step 3: Criar o secret `users-api-db-credentials` a partir do secret mestre do RDS**

```bash
cd /home/juliovaz/workspaces/video-processor-hackathon/iac-video-processor-data/prod
RDS_SECRET_ARN=$(terraform output -raw rds_master_user_secret_arn)
RDS_ENDPOINT=$(terraform output -raw rds_endpoint)
DB_HOST="${RDS_ENDPOINT%%:*}"
DB_PORT="${RDS_ENDPOINT##*:}"

RDS_CREDS=$(aws secretsmanager get-secret-value --secret-id "$RDS_SECRET_ARN" --query SecretString --output text)
DB_USER=$(echo "$RDS_CREDS" | jq -r .username)
DB_PASSWORD=$(echo "$RDS_CREDS" | jq -r .password)

kubectl create secret generic users-api-db-credentials \
  -n default \
  --from-literal=DB_HOST="$DB_HOST" \
  --from-literal=DB_PORT="$DB_PORT" \
  --from-literal=DB_USER="$DB_USER" \
  --from-literal=DB_PASSWORD="$DB_PASSWORD" \
  --from-literal=DB_NAME=usersdb \
  --dry-run=client -o yaml | kubectl apply -f -
```

- [ ] **Step 4: Apontar o overlay pra imagem real e aplicar os manifests**

```bash
cd /home/juliovaz/workspaces/video-processor-hackathon/video-processor-users-api/k8s/overlays/aws
kustomize edit set image video-processor-users-api="$ECR_URL:latest"
kubectl apply -k .
kubectl rollout status deployment/video-processor-users-api -n default --timeout=120s
kubectl rollout status deployment/video-processor-users-api-worker -n default --timeout=120s
```
Expected: os dois `rollout status` terminam com `successfully rolled out`.

- [ ] **Step 5: Setar a URL da fila SQS no worker**

```bash
QUEUE_URL=$(aws sqs get-queue-url --queue-name video-processor-user-events-queue-prod --query QueueUrl --output text)
kubectl set env deployment/video-processor-users-api-worker -n default SQS_QUEUE_URL="$QUEUE_URL"
kubectl rollout status deployment/video-processor-users-api-worker -n default --timeout=60s
```

- [ ] **Step 6: Verificar os pods e testar o `Service` diretamente (antes do Gateway)**

```bash
kubectl get pods -n default -l app=video-processor-users-api
kubectl get pods -n default -l app=video-processor-users-api-worker

kubectl run curl-test --rm -i --restart=Never --image=curlimages/curl -n default -- \
  curl -sf http://video-processor-users-api-svc.default.svc.cluster.local/api/health
```
Expected: ambos os `kubectl get pods` mostram `1/1 Running`; o `curl` retorna `{"status":"ok"}`.

Sem commit nesta task (só o `kustomize edit set image` no Step 4 muda um arquivo em disco — deixar sem commitar, é estado de deploy, não código-fonte).

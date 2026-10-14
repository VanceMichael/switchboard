# 用命名空间标签控制自动化接管范围

> **Workspace:** /Users/vance/project/mineProject/swe/09101005/project-02

---

**User**

共享集群的运维人员用命名空间标签开通或收回自动证书与 DNS 时，已有 IngressRoute 应随标签变化完成接管，而不要求逐个修改业务对象。请增加可选的 Namespace label selector 配置，并把它与现有 ingressClass 和 integration ignore 条件共同用于每次 reconcile：标签从不匹配变为匹配时，为该命名空间内符合条件的路由创建或修复 Certificate 与 DNSEndpoint；明确变为不匹配时，只删除由当前 IngressRoute 拥有的这两类派生资源，外来同名对象不得受影响；标签再次恢复后自动重建。Namespace 事件应只 enqueue 该命名空间中的 IngressRoute，配置的 target Service 发生变化时也只能更新当前被接管的路由。Namespace Get、List 或 watch 短暂失败表示成员状态未知，必须保留现有派生资源并以可重试错误结束，API 恢复后再按真实标签收敛，不能把控制面抖动当作退出范围。未设置 selector 时继续按现有 cluster-wide 方式工作；非法 selector 在启动时明确失败，Helm values、配置渲染和 Namespace RBAC 必须完整可用。

**Assistant**

我先探索代码库，理解现有的控制器结构、ingressClass 与 ignore 条件的实现方式。

toolName: Shell

status: success

toolName: no_need_execute

status: no_need_execute

toolName: Shell

status: success

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/cmd/main.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/selector.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/config/v1/config.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/utils.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/interface.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/certmanager.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/externaldns.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/utils.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/enqueue.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/hosts.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/targets.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/selector_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/values.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/config.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/deployment.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/rbac.yaml

toolName: Shell

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/_config.tpl

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/_helpers.tpl

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/utils_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8tests/client.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8tests/namespace.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/cmd/main_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8tests/dummies.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/ext/optionals.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/ext/slices.go

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/enqueue_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/tests/config/switchboard.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/dev/config.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/tests/resources/ingress.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/certmanager_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/pixi.toml

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/externaldns_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/tests/deployment.bats

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/monitor.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/issuer.yaml

toolName: Shell

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/README.md

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8tests/client_test.go

我已完成对整个仓库的彻底排查（所有 Go 源码、chart 模板、测试与配置均逐一阅读，不存在 envtest/fake client，也没有任何现存的 Namespace watch 或 `labels.Selector` 使用）。以下是完整的代码地图，所有路径为绝对路径，行号对应当前工作树。

---

# 0. 项目身份与技术栈

这是 **Switchboard**（`github.com/borchero/switchboard`）：监听 Traefik `IngressRoute`（traefik.io/v1alpha1），按集成配置派生 cert-manager `Certificate` 和 external-dns `DNSEndpoint`。

- Go 1.27；`sigs.k8s.io/controller-runtime v0.25.1`、`cert-manager v1.21.2`、`traefik/v3 v3.7.14`、`external-dns v0.23.0`（见 `/Users/vance/project/mineProject/swe/09101005/project-02/go.mod`）。
- **没有 finalizer**（全仓 grep 无任何 `Finalizer` 使用）：IngressRoute 删除后，派生物依赖 Kubernetes owner-reference 垃圾回收清理。

---

# 1. `internal/` 与 `cmd/` 目录树（每个文件一句话用途）

```
cmd/
├── main.go                 # 入口：解析 --config、加载 YAML 配置、建 scheme/manager、装配并启动 controller
└── main_test.go            # 仅测试 external-dns GroupVersion 的 parameter codec 编码（方案冒烟测试）

internal/
├── config/v1/
│   └── config.go           # 唯一的配置类型定义：Config/ControllerConfig/IngressSelector/两个集成配置/ServiceRef
├── controllers/
│   ├── ingressroute.go     # IngressRouteReconciler：Reconcile 主循环 + SetupWithManager（For/Owns/Watches）
│   ├── ingressroute_test.go# 针对真实集群的 reconcile 端到端用例（5 个 host 场景）+ runTest 测试夹具
│   ├── utils.go            # integrationsFromConfig（集成装配+启动校验）与 builderWithIntegrations（watch 装配）
│   └── utils_test.go       # integrationsFromConfig 的组合与校验失败用例
├── switchboard/            # （领域逻辑包，与 controller-runtime 无关）
│   ├── selector.go         # ★核心：Selector（ingress class + ignore 注解判定），Matches/MatchesIntegration
│   ├── selector_test.go    # selector 的全部判定矩阵测试
│   ├── hosts.go            # HostCollection：从 TLS.domains 与 Route Match 规则中提取/去重 host
│   ├── hosts_test.go       # host 解析测试
│   ├── targets.go          # Target 接口 + serviceTarget（动态取 Service LB/ClusterIP/hostname 注解）+ staticTarget
│   └── targets_test.go     # target 取值测试
├── integrations/
│   ├── interface.go        # Integration 接口（Name/OwnedResource/WatchedObject/UpdateResource）+ IngressInfo
│   ├── certmanager.go      # cert-manager 集成：CreateOrPatch Certificate，无 TLS/hosts 时 DeleteIfFound
│   ├── certmanager_test.go # Certificate 创建/更新/删除全流程测试（真实集群）
│   ├── externaldns.go      # external-dns 集成：CreateOrPatch DNSEndpoint，无 host 时删除；WatchedObject=目标 Service
│   ├── externaldns_test.go # DNSEndpoint 流程、A/AAAA/CNAME 判定、TTL、watched object 测试
│   ├── utils.go            # reconcileMetadata：managed-by 标签、ingress.class 注解、SetControllerReference
│   └── utils_test.go       # 元数据/owner ref 协调测试
├── k8s/
│   ├── delete.go           # DeleteIfFound（Delete + IsNotFound 容错）
│   ├── delete_test.go      # 重复删除不报错测试
│   ├── enqueue.go          # ★泛型 EnqueueMapFunc：目标对象变更时 list 全量某类资源并入队
│   └── enqueue_test.go     # 仅当 GVK+name+namespace 命中配置目标时才全量入队的测试
├── ext/
│   ├── optionals.go        # 泛型 AndThen[*T]（用于 TLS.SecretName 解引用）
│   ├── optionals_test.go
│   ├── slices.go           # 泛型 Map
│   └── slices_test.go
└── k8tests/                # 测试辅助（连真实集群，非 fake/envtest）
    ├── client.go           # NewScheme（注册 core/traefik/cert-manager/external-dns）+ NewClient（读 ~/.kube/config）
    ├── client_test.go      # scheme codec 冒烟
    ├── namespace.go        # NewNamespace：建随机 UUID namespace，返回名称与清理函数
    └── dummies.go          # DummyService：造一个 selector 不匹配任何 Pod 的 Service
```

根目录另有：`dev/config.yaml`（本地 `go run` 配置）、`dev/manifests/*`、`tests/`（bats E2E + helm values + IngressRoute 样例）。

---

# 2. 主 reconcile 循环

## 2.1 位置与签名

文件：`/Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go`

- 结构体 `IngressRouteReconciler`（第 19-24 行）：内嵌 `client.Client`，持有 `logger`、`selector switchboard.Selector`、`integrations []integrations.Integration`。
- 构造函数（第 27-40 行）：
```go
func NewIngressRouteReconciler(
	client client.Client, logger *slog.Logger, config configv1.Config,
) (IngressRouteReconciler, error)
```
  其中第 37 行：`selector: switchboard.NewSelector(config.Selector.IngressClass)`。
- 入口（第 44-46 行）：
```go
func (r *IngressRouteReconciler) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error)
```

## 2.2 精确逻辑流（ingressroute.go）

1. **Get IngressRoute**（第 50-57 行）：查不到返回 `client.IgnoreNotFound(err)`，不做任何级联删除（派生物靠 K8s GC）。
2. ★**总开关判定**（第 60-63 行）：
```go
if !r.selector.Matches(ingressRoute.Annotations) {
	logger.Debug("ignoring ingress route")
	return ctrl.Result{}, nil
}
```
   注意：判定输入**只有 IngressRoute 的 annotations**。一旦不匹配直接返回——不会调用任何集成，因此**也不会清理此前已创建的 Certificate/DNSEndpoint**（这点对你新功能很关键，见第 8 节）。
3. **提取 host**（第 68-74 行）：`switchboard.NewHostCollection().WithTLSHostsIfAvailable(Spec.TLS).WithRouteHostsIfRequired(Spec.Routes)`；TLS domains 优先，若已有 host 则跳过路由解析（hosts.go 第 38-43 行）。
4. **组装 IngressInfo**（第 75-80 行）：`Hosts` 与 `TLSSecretName`（用 `ext.AndThen` 从 `*traefik.TLS` 解引用）。
5. **逐集成执行**（第 83-96 行）：
```go
for _, itg := range r.integrations {
	if !r.selector.MatchesIntegration(ingressRoute.Annotations, itg.Name()) {
		continue                       // 该集成被 ignore 注解单独关闭
	}
	if err := itg.UpdateResource(ctx, &ingressRoute, info); err != nil {
		return ctrl.Result{}, err
	}
}
```
集成顺序由 `integrationsFromConfig` 决定：external-dns 先、cert-manager 后（utils.go 第 22-48 行）。

## 2.3 ingressClass 与 ignore 判定（决策核心）

文件：`/Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/selector.go`

常量（第 5-8 行）：
```go
ingressAnnotationKey = "kubernetes.io/ingress.class"
ignoreAnnotationKey  = "switchboard.borchero.com/ignore"
```

- `NewSelector(ingressClass *string)`（第 17-19 行）：nil 表示不限制 class。
- `Matches(annotations)`（第 23-42 行）——**全局接管门**：
  1. `ignore` 为 `"true"` 或 `"all"` → 一律 false（第 25-29 行）；
  2. 配置了 ingressClass 时：注解存在且相等才 true；**注解缺失即 false**（第 32-38 行，注意不是"默认 class"语义）；
  3. 未配置 class：其余一律 true（第 41 行）。
- `MatchesIntegration(annotations, integration)`（第 46-59 行）——**每集成门**：`true`/`all` 关闭全部；否则把值按逗号分割、TrimSpace 后与集成名（`"external-dns"`、`"cert-manager"`，见 externaldns.go 第 35 行、certmanager.go 第 27 行）精确比较。判定矩阵测试在 `selector_test.go` 第 9-106 行。

README 对应文档：`/Users/vance/project/mineProject/swe/09101005/project-02/README.md` 第 151-166 行。

## 2.4 Certificate / DNSEndpoint 的写、改、删

### cert-manager（`internal/integrations/certmanager.go`）

`UpdateResource`（第 38-73 行）：
- **删除分支**（第 43-49 行）——`TLSSecretName == nil` 或 `len(Hosts)==0`：
```go
certificate := certmanager.Certificate{ObjectMeta: c.objectMeta(owner)}
if err := k8s.DeleteIfFound(ctx, c.client, &certificate); err != nil { ... }
```
- **upsert 分支**（第 52-71 行）：`controllerutil.CreateOrPatch`，mutate 内调 `reconcileMetadata`，再把 `template.Spec` 深拷贝后强制覆盖 `SecretName=TLSSecretName`、`DNSNames=Hosts`，用 mergo WithOverride 合并。
- 命名（第 79-84 行）：**`<ingressroute-name>-tls`**，同 namespace；不带 owner 注解。

### external-dns（`internal/integrations/externaldns.go`）

`UpdateResource`（第 55-89 行）：
- **删除分支**（第 60-66 行）：`len(info.Hosts)==0` 时用仅含 objectMeta 的 DNSEndpoint 调 `DeleteIfFound`。
- 取 target（第 69-72 行）：`e.target.Targets`（Service 的 external-dns hostname 注解 → LB hostname/IP → ClusterIP，targets.go 第 63-102 行）。
- **upsert 分支**（第 75-87 行）：`CreateOrPatch`，写 `Spec.Endpoints`。注意第 78-80 行有个现存小缺陷——`reconcileMetadata` 出错时 `return nil` 吞掉了 error。
- 命名（第 95-101 行）：**与 IngressRoute 同名**、同 namespace，并**整体复制 owner 的 annotations**。
- 记录类型由 govalidator 判 IPv4/IPv6 决定 A/AAAA/CNAME（第 126-134 行）；TTL 默认 300（第 26-32 行）。

### 删除原语

`/Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete.go` 第 13-21 行：`client.Delete` + `apierrs.IsNotFound` 容错。**删除只按 name/namespace 直发，不校验 owner**。

## 2.5 Owner references：如何区分"自有 vs 外来"对象

`/Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/utils.go`，`reconcileMetadata`（第 12-44 行）：
- 强制标签 `kubernetes.io/managed-by: switchboard`（常量在 interface.go 第 11 行），再合并模板标签/注解；
- 第 40 行 `ctrl.SetControllerReference(owner, target, scheme)`：**controller ownerRef 指向 IngressRoute**。

"自有 vs 外来"完全靠这套机制区分，而**不是**靠业务代码判断：
- controller-runtime 的 `Owns(...)` watch 只把 ownerRef 指向某 IngressRoute 的派生物事件映射回该 owner；
- name 冲突时 `CreateOrPatch` 直接 get 同名对象——若同名 Certificate/DNSEndpoint 已存在但无本 controller 的 ownerRef，`SetControllerReference` 会因已有 controller 而报错（天然保护外来对象）。

---

# 3. Watch 装配

文件：`/Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go` 第 103-107 行 + `utils.go` 第 52-82 行。

```go
// ingressroute.go:104
builder := ctrl.NewControllerManagedBy(mgr).For(&traefik.IngressRoute{})
builder = builderWithIntegrations(builder, r.integrations, r, r.logger)
return builder.Complete(r)
```

`builderWithIntegrations` 做两类附加 watch：

1. **Owns**（utils.go 第 59-61 行）：对每个启用的集成 `builder.Owns(itg.OwnedResource())`：
   - cert-manager → `&certmanager.Certificate{}`（certmanager.go 第 30-32 行）
   - external-dns → `&externaldnsv1alpha1.DNSEndpoint{}`（externaldns.go 第 38-40 行）
   - 走 controller-runtime 内置 ownerRef handler，无需自定义映射。

2. **Watches + EnqueueRequestsFromMapFunc**（utils.go 第 64-79 行）：仅当 `itg.WatchedObject() != nil`。
   - 只有配置了 **targetService** 的 external-dns 会返回具体对象 `&v1.Service{Name, Namespace}`（externaldns.go 第 42-53 行）；静态 IP（staticTarget）返回 nil（targets.go 第 126-128 行）。
   - 映射器：`k8s.EnqueueMapFunc(ctrlClient, logger, itg.WatchedObject(), &traefik.IngressRouteList{}, ...)`，包成 `handler.EnqueueRequestsFromMapFunc(enqueue)`。

`EnqueueMapFunc` 泛型实现：`/Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/enqueue.go` 第 14-44 行：
- 先比对 **GVK + namespace + name 必须精确等于配置的目标 Service**，否则返回 nil（第 23-27 行）——集群里其他 Service 变更不触发；
- 命中后 **`ctrlClient.List(ctx, list)` 全集群列出所有 IngressRoute**（第 30 行，无 namespace/label 过滤），全部转成 reconcile.Request（第 36-42 行）；selector 过滤发生在各自 Reconcile 内部。

**目前没有任何 Namespace watch**（全仓 grep 仅 test helper 用到 `v1.Namespace`），也没有 predicate。

---

# 4. 配置加载

- 唯一 flag：`--config`，默认 `/etc/switchboard/config.yaml`（`cmd/main.go` 第 26-28 行）。无 env 变量、无其他 flag。
- 读取 + `sigs.k8s.io/yaml.Unmarshal` 到 `configv1.Config`（main.go 第 38-49 行），读/解析失败直接 `os.Exit(1)`。
- 配置结构：`/Users/vance/project/mineProject/swe/09101005/project-02/internal/config/v1/config.go`
  - `Config`（第 8-12 行）= 内联 `ControllerConfig` + `Selector IngressSelector` + `Integrations IntegrationConfigs`；
  - **`IngressSelector`（第 43-45 行）当前只有一个字段**：
    ```go
    type IngressSelector struct {
        IngressClass *string `json:"ingressClass,omitempty"`
    }
    ```
  - 集成配置：`ExternalDNSIntegrationConfig`（TargetService/TargetIPs/TTL，第 55-59 行）、`CertManagerIntegrationConfig`（整个 `v1.Certificate` 模板，第 62-64 行）、`ServiceRef`（第 67-70 行）。
- Manager Options 直接读 `config.LeaderElection/*`、`config.Metrics.BindAddress`、`config.Health.HealthProbeBindAddress`（main.go 第 52-61 行）。
- Scheme 条件注册（main.go 第 101-113 行 `initScheme`）：core + traefik 恒注册；cert-manager 仅在 `Integrations.CertManager != nil`；external-dns 同理（外加 `metav1.AddToGroupVersion`）。**core v1 始终在 scheme 中，新增 Namespace watch 无需改 scheme。**
- **启动期校验**：唯一一处是 `integrationsFromConfig`（`controllers/utils.go` 第 24-28 行）——external-dns 必须恰好二选一设置 targetService / targetIPs，否则返回 error，main 第 72-76 行退出。**Selector 当前无任何校验**（新功能若用 `labels.Parse`/`LabelSelectorAsSelector`，应在此构造路径返回 error）。
- 本地样例：`/Users/vance/project/mineProject/swe/09101005/project-02/dev/config.yaml`（注意它是旧字段格式 `externalDns.target`，与现行 struct 不一致，仅遗留文件）。

---

# 5. Helm chart（`chart/`）

```
chart/
├── Chart.yaml              # 条件依赖 bitnami external-dns 8.7.11 与 jetstack cert-manager 1.17.1
├── values.yaml             # 全部可配项（下详）
├── README.md / .gotmpl     # helm-docs 生成
└── templates/
    ├── _helpers.tpl        # 仅 image.tag 一个 helper
    ├── _config.tpl         # ★define "config"：把 values 渲染成 config.yaml
    ├── config.yaml         # ConfigMap <release>-config，挂载为 config.yaml
    ├── deployment.yaml     # Deployment，subPath 挂载到 /etc/switchboard/config.yaml（无 env/flags 传参）
    ├── rbac.yaml           # ServiceAccount + ClusterRole + ClusterRoleBinding
    ├── issuer.yaml         # 可选 ClusterIssuer（ACME）
    └── monitor.yaml        # 可选 PodMonitor
```

关键 values 键（`chart/values.yaml`）：
- `selector.ingressClass`（第 33-36 行，默认 `~`）——唯一的 selector 配置；
- `integrations.certManager.{enabled,certificateTemplate}`（第 39-49 行）、`integrations.externalDNS.{enabled,targetIPs,targetService.{name,namespace},ttl}`（第 50-65 行）；
- 另有 image/replicas/podAnnotations/resources/nodeSelector/tolerations/affinity/metrics/podMonitor/certificateIssuer/第三方依赖透传。

配置如何贯通（**全部走挂载文件，不走 env/CLI**）：
- `templates/config.yaml`：ConfigMap 数据键 `config.yaml: {{ include "config" . | ... }}`。
- `templates/_config.tpl` 第 15-18 行渲染 selector：
```yaml
{{ if .Values.selector.ingressClass }}
selector:
  ingressClass: {{ .Values.selector.ingressClass }}
{{ end }}
```
  第 22-55 行按 enabled 渲染 integrations，并用 helm `fail` 做模板期校验（certManager 必须有 template 或建 issuer；externalDNS 必须二选一）。
- `deployment.yaml` 第 34-37、53-56 行把 ConfigMap 以 subPath 挂到容器 `/etc/switchboard/config.yaml`——与 main.go 默认 flag 路径一致。

RBAC（`chart/templates/rbac.yaml`，ClusterRole 第 8-39 行）：
- 恒有：`traefik.io/ingressroutes` get/list/watch；configmaps、leases、events（leader election）；
- 条件：cert-manager enabled → `cert-manager.io/certificates` 全量；external-dns enabled → `externaldns.k8s.io/dnsendpoints` 全量 + `"" /services` get/list/watch（第 22-29 行）。
- **没有 `namespaces` 权限条目**——新功能 watch Namespace 需要在此添加 `apiGroups: [""] resources: ["namespaces"] verbs: ["get","list","watch"]`。

---

# 6. 测试体系

- **框架：标准 `go test` + `stretchr/testify`（assert/require）**。**没有 envtest、没有 ginkgo、没有 fake client（controller-runtime fake 包未被使用）**。
- 单元级纯逻辑测试（无需集群）：`switchboard/selector_test.go`、`hosts_test.go`、`targets_test.go` 部分、`integrations/externaldns_test.go` 的 endpoint/TTL 部分、`ext/*_test.go`、`cmd/main_test.go`。
- **集成测试直连真实集群**：`internal/k8tests/client.go` 第 38-45 行 `NewClient` 读取 `~/.kube/config` 建真实 client；`namespace.go` 每个用例建 UUID 随机 namespace 并在用例后删除。因此这些测试面向 minikube/kind 运行（pixi 任务 `cluster-setup` 装 CRD，`pixi.toml` 第 40-47 行）：
  - reconcile 主测：`internal/controllers/ingressroute_test.go`（`runTest` 第 191-249 行：建 Service + IngressRoute → 直接手调 `Reconcile`，第 255-268 行 → 断言 `<name>-tls` Certificate 与同名 DNSEndpoint 的存在/缺失与字段；`createConfig` 第 270-291 行）。
  - 集成级：`certmanager_test.go`、`externaldns_test.go`、`k8s/delete_test.go`、`k8s/enqueue_test.go`、`switchboard/targets_test.go`、`integrations/utils_test.go`。
- E2E（bats + kubectl/helm）：`tests/deployment.bats`（装 chart → apply `tests/resources/ingress.yaml` → 断言 dnsendpoint/certificate 被创建；删 IngressRoute → 断言二者被 GC 删除）、`tests/lib/helpers.bash`、values 覆盖文件 `tests/config/switchboard.yaml`。pixi 入口：`pixi task test-e2e <image> <tag>`（pixi.toml 第 56-61 行）；覆盖率：`go test ./... -coverprofile cover.out`（第 53-54 行）。

新功能测试的落点建议：selector 纯判定可仿 `selector_test.go` 做无集群单测；reconcile 门控测试可在 `ingressroute_test.go` 的夹具里给 namespace 打标签（client 是真实 client，可直接 update Namespace labels）。

---

# 7. 现存 namespace-label / selector 逻辑排查结果

结论：**不存在任何 namespace 标签选择器逻辑**，可放心新增，无冲突抽象。证据：

- 全仓 Go 代码 grep `LabelSelector|labels.Selector|MatchingLabels`：**零命中**。
- 唯一的 "selector" 业务概念是 `switchboard.Selector`（注解判定，selector.go）与配置字段 `IngressSelector.IngressClass`（config.go 第 43-45 行；ingressroute.go 第 37 行装配；chart values 第 33-36 行、_config.tpl 第 15-18 行）。
- 代码中 `Namespace` 仅出现在：对象元数据透传、target Service 的 namespace（config.ServiceRef / serviceTarget）、测试夹具 `k8tests/namespace.go`，以及 `EnqueueMapFunc` 对 target GVK+ns+name 的过滤（enqueue.go 第 24 行）。
- chart 里另一处 `selector` 是 PodMonitor 的 `spec.selector.matchLabels`（monitor.yaml 第 11-14 行），与本功能无关。
- 无 Namespace informer/watch、无 namespaces RBAC、无 `corev1.Namespace` 读取（除测试建 ns）。

---

# 8. 给多文件改动的关键提示（基于上述事实）

1. **配置**：扩展 `internal/config/v1/config.go` 第 43-45 行的 `IngressSelector`（例如新增 `NamespaceLabels map[string]string` 或 `*metav1.LabelSelector`）；启动解析/校验失败应让 `NewIngressRouteReconciler`（ingressroute.go 第 27 行）返回 error，由 main.go 第 72-76 行退出。
2. **门控组合点**：ingressroute.go 第 60 行是唯一总闸；当前 `Selector.Matches` 只收 annotations（selector.go 第 23 行），要么改签名（同时收 namespace labels），要么在 Reconcile 里先 `Get` IngressRoute 所在的 `corev1.Namespace` 再判定。判定顺序建议与现有 ignore/class 串联。
3. **Watch 触发缺口**：Namespace 打标签/改标签必须能触发 reconcile，否则已存在的 IngressRoute 不会重新评估。需在 `builderWithIntegrations`（controllers/utils.go 第 52 行）附近增加 `builder.Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(...))`，map 函数可复用 `k8s.EnqueueMapFunc` 的思路但不能照搬——它按单个目标 name 过滤（enqueue.go 第 23-27 行）；Namespace watch 需要 list IngressRoute 时按 namespace 收窄（现有实现是无差别全量 List）。
4. **RBAC/scheme**：rbac.yaml 增加 namespaces get/list/watch；scheme 无需改（clientgoscheme 已注册，main.go 第 102 行）。
5. **取消接管不清理的既有语义**：当 class 不匹配/ignore 命中/（未来）namespace 标签不匹配时，Reconcile 在第 60-63 行直接返回，**不会删除已生成资源**；派生物只在 IngressRoute 被删除时由 ownerRef GC 回收（无 finalizer，见第 0/2.5 节；bats 用例也只覆盖父对象删除）。若新功能期望"移出选择范围即回收 Certificate/DNSEndpoint"，这是当前不存在的行为，需要新增专门的下线逻辑，并注意 DeleteIfFound（delete.go 第 13 行）不校验 owner、同名外来对象可能被误删。
6. **Helm**：values.yaml `selector` 段（第 33 行）新增键 + `_config.tpl` 第 15-18 行同段渲染即可自动经 ConfigMap 挂载贯通，无需改 deployment（无 env/flags 路径）。
7. **测试**：纯门控逻辑放 `selector_test.go` 风格的无集群单测；端到端走 `ingressroute_test.go` 现有真实集群夹具（可对 `k8tests.NewNamespace` 返回的 ns 追加 labels）。

主要文件绝对路径汇总：
- `/Users/vance/project/mineProject/swe/09101005/project-02/cmd/main.go`
- `/Users/vance/project/mineProject/swe/09101005/project-02/internal/config/v1/config.go`
- `/Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go`
- `/Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/utils.go`
- `/Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/selector.go`
- `/Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/{interface.go,certmanager.go,externaldns.go,utils.go}`
- `/Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/{enqueue.go,delete.go}`
- `/Users/vance/project/mineProject/swe/09101005/project-02/chart/{values.yaml,templates/_config.tpl,templates/rbac.yaml,templates/deployment.yaml}`

探索完成。让我仔细阅读关键文件，确认实现细节。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/selector.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/config/v1/config.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/utils.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/interface.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/certmanager.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/externaldns.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/utils.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/enqueue.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/cmd/main.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/values.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/_config.tpl

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/rbac.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/selector_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/enqueue_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8tests/namespace.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8tests/client.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/deployment.yaml

继续阅读其余相关文件。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/utils_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/utils_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/dev/config.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/README.md

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/tests/config/switchboard.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/tests/deployment.bats

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/README.md

代码结构已清楚。这是一个多文件特性，我先建立任务清单，然后确认工具链可用。

toolName: todo_write

status: success

Todos updated: 9 items

toolName: Shell

status: success

工具链就绪。开始实现。

**1. 配置结构**

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/config/v1/config.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/config/v1/config.go

changes: undefined

toolName: todo_write

status: failed

Todos updated: 2 items

toolName: todo_write

status: success

Todos updated: 9 items

**2. switchboard.Selector 命名空间匹配**

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/selector.go

changes: undefined

**3. k8s.DeleteIfOwned**

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete.go

changes: undefined

**4. Integration 接口与两个实现**

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/interface.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/certmanager.go

changes: undefined

操作失误，恢复该行并在正确位置插入 DeleteResource。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/certmanager.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/certmanager.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/externaldns.go

changes: undefined

**5. Reconciler 构造校验与 Reconcile 门控**

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go

changes: undefined

现在在 Reconcile 中加入命名空间门控：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go

changes: undefined

**6. Watch 装配**

先给 `EnqueueMapFunc` 增加按当前接管状态过滤的能力：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/enqueue.go

changes: undefined

新建 `watches.go`：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/watches.go

changes: undefined

更新 `utils.go` 的 watch 装配：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/utils.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/utils.go

changes: undefined

更新 `SetupWithManager` 注册 Namespace watch：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go

changes: undefined

先编译看一下所有受影响点：

toolName: Shell

status: success

参数名 `client` 遮蔽了包名，用 `types.NamespacedName` 修正：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

v0.25 中是包级函数，不是 builder 方法：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go

changes: undefined

局部变量遮蔽了 builder 包。查看泛型签名后用包别名处理：

toolName: view_files

status: success

filePath: /Users/vance/go/pkg/mod/sigs.k8s.io/controller-runtime@v0.25.1/pkg/builder/options.go

toolName: Shell

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go

changes: undefined

toolName: Shell

status: success

ingressroute.go 原本没有导入 builder 包，补上：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go

changes: undefined

toolName: Shell

status: success

toolName: todo_write

status: success

Todos updated: 9 items

**7. 测试**

先更新 selector 单测：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/selector_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/selector_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/selector_test.go

changes: undefined

在 selector_test.go 末尾追加命名空间选择器测试：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/selector_test.go

changes: undefined

更新 enqueue 测试，加入过滤行为覆盖：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/enqueue_test.go

changes: undefined

为 `DeleteIfOwned` 添加集群测试：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete_test.go

changes: undefined

现在新增基于 fake client 的核心门控测试（无需真实集群即可运行）：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_namespace_test.go

changes: undefined

去掉多余的占位引用：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_namespace_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_namespace_test.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

修复两处类型问题：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_namespace_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_namespace_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_namespace_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_namespace_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_namespace_test.go

changes: undefined

修复 delete_test.go 的指针参数：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete_test.go

changes: undefined

toolName: Shell

status: success

Vet 通过。运行无需集群的测试：

toolName: Shell

status: success

`Targets` 是命名类型，改用 `ElementsMatch`：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_namespace_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_namespace_test.go

changes: undefined

toolName: Shell

status: success

`DoesNotExist` 是合法操作符，改用非法值 `Bogus`：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/switchboard/selector_test.go

changes: undefined

现在补充真实集群测试（与仓库现有约定一致）。先在 ingressroute_test.go 追加标签生命周期测试：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute_test.go

changes: undefined

新增 watch 映射的集群测试：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/watches_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/watches_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/watches_test.go

changes: undefined

toolName: Shell

status: success

测试代码完成。开始 Helm 部分。

toolName: todo_write

status: failed

Todos updated: 10 items

toolName: todo_write

status: success

Todos updated: 9 items

**8. Helm chart**

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/values.yaml

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/_config.tpl

changes: undefined

RBAC 增加 namespaces 只读权限：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/rbac.yaml

changes: undefined

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/README.md.gotmpl

chart README 表格补上一行：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/chart/README.md

changes: undefined

渲染 Helm 模板验证配置与 RBAC：

toolName: Shell

status: success

依赖未下载（CI 会先 `helm dependency build`）。在临时副本中去掉依赖声明做渲染验证：

toolName: Shell

status: success

渲染结果正确（空行是模板原有风格）。用 YAML 解析器确认结构有效：

toolName: Shell

status: success

用真实 Go 配置结构做端到端解析校验（临时程序，验证后删除）：

toolName: Shell

status: success

再验证启用集成时完整渲染无回归：

toolName: Shell

status: success

**根 README 文档**

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/README.md

changes: undefined

最后做完整校验。先通读最终 reconcile 文件：

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go

**9. 最终验证**

toolName: Shell

status: success

全部编译通过。运行所有不依赖真实集群的测试：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

检查完整变更集：

toolName: Shell

status: success

核对 CI 流程是否需要同步调整：

toolName: Shell

status: success

toolName: Shell

status: success

加固新 enqueue 过滤测试，避免共享集群中的同名干扰：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/enqueue_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/enqueue_test.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: todo_write

status: success

Todos updated: 9 items

功能已实现完成。以下是变更总结：

## 核心行为

**配置与启动校验**
- [config.go](file:///Users/vance/project/mineProject/swe/09101005/project-02/internal/config/v1/config.go#L43-L51)：`IngressSelector` 新增可选 `namespaceSelector`（标准 `metav1.LabelSelector`，支持 matchLabels/matchExpressions）。
- [ingressroute.go](file:///Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go#L42-L51)：构造时用 `LabelSelectorAsSelector` 预解析，非法 selector 返回 error，由 [main.go](file:///Users/vance/project/mineProject/swe/09101005/project-02/cmd/main.go#L72-L76) 退出；未设置时 `RestrictsNamespaces()==false`，完全保持 cluster-wide 路径（不装 Namespace watch）。

**每次 reconcile 的三层共同判定**（[ingressroute.go#L78-L111](file:///Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/ingressroute.go#L78-L111)）
1. Namespace Get 失败（含控制面抖动）→ 成员状态未知，**保留派生物并返回可重试 error**，API 恢复后按真实标签收敛；
2. 明确不匹配 → 对每个集成调 `DeleteResource`，只删除由当前 IngressRoute 控制（controller ownerRef UID 比对）的 Certificate/DNSEndpoint，**外来同名对象不动**；
3. 匹配后再串联现有 ingressClass 门与逐集成 ignore 注解，创建或修复派生物。标签恢复后走正常 upsert 自动重建。

**所有权保护**：新增 [DeleteIfOwned](file:///Users/vance/project/mineProject/swe/09101005/project-02/internal/k8s/delete.go#L27-L48)（Get→`metav1.IsControlledBy`→Delete），集成接口新增 `DeleteResource`（[certmanager.go](file:///Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/certmanager.go#L75-L81)、[externaldns.go](file:///Users/vance/project/mineProject/swe/09101005/project-02/internal/integrations/externaldns.go#L91-L97)）。

**事件映射**（[watches.go](file:///Users/vance/project/mineProject/swe/09101005/project-02/internal/controllers/watches.go)）
- Namespace watch 仅在配置 selector 时注册，使用 `LabelChangedPredicate`；map 函数 `InNamespace` **只 enqueue 该命名空间中的 IngressRoute**；
- target Service 事件的入队新增 `isManagedRoute` 过滤（命名空间标签 + class/ignore），只更新当前被接管的路由；成员查询失败时宁可不入队，绝不触发误删除。

**Helm**：[values.yaml](file:///Users/vance/project/mineProject/swe/09101005/project-02/chart/values.yaml#L33-L42) 新增 `selector.namespaceSelector`；[_config.tpl](file:///Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/_config.tpl#L15-L24) 三种组合渲染；[rbac.yaml](file:///Users/vance/project/mineProject/swe/09101005/project-02/chart/templates/rbac.yaml#L16-L19) 增加 namespaces get/list/watch；chart/root README 同步。

## 验证
- `gofmt`、`go vet`、`go build ./...`、全部测试编译通过；
- fake client 无需集群的测试全部 PASS：非法 selector 启动失败、Get 失败保留资源且返回 error、完整生命周期（进入接管→出范围只删自有→恢复重建）、外来同名对象全程不受影响；
- 集群测试（沿用仓库 minikube 约定，CI 中运行）：标签四阶段生命周期、namespace 事件精确 enqueue、`DeleteIfOwned`、enqueue 过滤；
- `helm template` 三种 selector 组合渲染后均经真实 Go 配置结构反序列化验证，`helm lint` 通过。

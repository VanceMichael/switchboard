# 让无头服务的 DNS 地址随端点收敛

> **Workspace:** /Users/vance/project/mineProject/swe/09101005/project-01

---

**User**

平台团队把 external-dns 的目标指向无头 Service 后，DNSEndpoint 不能把 ClusterIP 的哨兵值 None 当作真实 CNAME，也不能在 Pod 下线后继续发布已经失效的地址。请让该目标从属于同一 Service 的 EndpointSlice 汇总当前可发布的 ready IPv4/IPv6 地址，并在切片创建、地址或 readiness 更新、删除以及 Service 在 headless 与普通地址来源之间切换时，驱动所有相关 IngressRoute 重新收敛；没有 ready 地址时应撤销陈旧 DNSEndpoint，新的可用地址出现后自动恢复。只接受目标 Service 对应切片中的有效地址，未就绪、正在终止、其他 Service 的端点和重复地址不得进入结果。hostname annotation、LoadBalancer hostname/IP、普通 ClusterIP、静态 target 的现有选择顺序必须保持，未使用无头 Service 的部署不改变行为；manager 的 scheme、watch 映射和 Helm RBAC 也要支持生产环境的 EndpointSlice list/watch。证书 integration、现有 Service 更新触发、双栈普通 Service 与既有 chart 安装结果应继续通过回归。

**Assistant**

我先了解项目结构和代码，再制定实施方案。

toolName: Shell

status: success

toolName: no_need_execute

status: no_need_execute

toolName: Shell

status: success

toolName: search_by_regex

status: success

toolName: file_search

status: success

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/switchboard/targets.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/cmd/main.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/ingressroute.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/externaldns.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/utils.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/k8s/enqueue.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/chart/templates/rbac.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/switchboard/targets_test.go

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/pixi.toml

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/tests/deployment.bats

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/k8tests/client.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/config/v1/config.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/k8tests/dummies.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/ingressroute_test.go

toolName: Shell

status: success

# Switchboard codebase map (module `github.com/borchero/switchboard`)

This is **borchero/switchboard**, a controller that watches Traefik `IngressRoute` CRs and creates external-dns `DNSEndpoint` (and cert-manager `Certificate`) resources. My search was exhaustive: all Go files under `cmd/`/`internal/`, the full Helm chart, and both test layers were reviewed.

## 1. Directory layout

- `cmd/main.go`, `cmd/main_test.go` — entrypoint, scheme/manager bootstrap.
- `internal/config/v1/config.go` — YAML config types (`ExternalDNSIntegrationConfig.TargetService`/`TargetIPs`).
- `internal/controllers/ingressroute.go` — `IngressRouteReconciler` (Reconcile, `SetupWithManager`); `utils.go` — builds integrations from config + wires watches; `utils_test.go`, `ingressroute_test.go` — live-cluster integration tests.
- `internal/switchboard/targets.go` (+`targets_test.go`) — **target resolution** (`Target` interface, `serviceTarget`, `staticTarget`); `hosts.go` host extraction from routes/TLS; `selector.go` annotation-based integration toggle.
- `internal/integrations/externaldns.go` (+test) — builds/patches `DNSEndpoint`; `certmanager.go` — `Certificate`; `interface.go` (`Integration`: `Name/OwnedResource/WatchedObject/UpdateResource`); `utils.go`.
- `internal/k8s/enqueue.go` — generic watch→reconcile mapper; `delete.go` — DeleteIfFound.
- `internal/k8tests/` — live-cluster test helpers: `client.go` (`NewScheme`, `NewClient` from `~/.kube/config`), `namespace.go`, `dummies.go` (`DummyService`).
- `internal/ext/` — small option/slice helpers.
- `chart/templates/` — `rbac.yaml`, `deployment.yaml`, `config.yaml`, `_config.tpl`, `issuer.yaml`, `monitor.yaml`.
- `tests/deployment.bats` + `lib/helpers.bash`, `resources/ingress.yaml`, `config/switchboard.yaml` — e2e bats tests.
- `dev/manifests/` — local CA/issuer/ingress; `pixi.toml` (no Makefile).

## 2. Manager setup — `cmd/main.go`

`main()` parses `--config`, builds `ctrl.Options{Scheme: runtime.NewScheme()}`, calls `initScheme` (lines 101-113): `clientgoscheme`, `traefik.AddToScheme`, conditionally cert-manager and `externaldnsv1alpha1.AddToScheme`. Then `controllers.NewIngressRouteReconciler` (line 72) and `controller.SetupWithManager(manager)` (line 77).

`internal/controllers/ingressroute.go:103` `SetupWithManager`: `ctrl.NewControllerManagedBy(mgr).For(&traefik.IngressRoute{})` then `builderWithIntegrations` (`utils.go:52`) which adds `.Owns(itg.OwnedResource())` for `DNSEndpoint`/`Certificate`, and `.Watches(itg.WatchedObject(), handler.EnqueueRequestsFromMapFunc(...))`.

## 3. Reconciliation & target computation

`Reconcile` (`ingressroute.go:44`) gets the IngressRoute, checks selector, builds host collection, then loops integrations calling `itg.UpdateResource`. external-dns integration calls `e.target.Targets(ctx, e.client)` at `internal/integrations/externaldns.go:69`, then `endpoints()` (line 103) groups targets into A/AAAA/CNAME records via `recordType` (line 126).

**All address-selection logic lives in `internal/switchboard/targets.go`:**

```go
// 63
func (t serviceTarget) Targets(ctx, client) ([]string, error) {
	var service v1.Service
	if err := client.Get(ctx, t.name, &service); err != nil { ... }
	return t.targetsFromService(service), nil
}
// 72  selection order:
// 1) annotation external-dns.alpha.kubernetes.io/hostname (line 76) -> single CNAME
// 2) service.Status.LoadBalancer.Ingress: first Hostname wins, else append IPs (84-95)
// 3) fallback: service.Spec.ClusterIPs (98-100)
```

Line 98-100 is the headless bug: for a headless Service `ClusterIPs == ["None"]`, so `"None"` becomes the DNSEndpoint target. `serviceTarget` only does a single `client.Get` of the Service — this is where EndpointSlice aggregation must be added. Static targets (`NewStaticTarget`, line 118) come from config `targetIPs` (`utils.go:29-42`).

## 4. EndpointSlice / headless / "None" references

There are **no** EndpointSlice/Endpoints/headless references anywhere. Only matches: `targets.go:99` (ClusterIP fallback), `targets_test.go:53-63`, and `ingressroute_test.go:246` (`assert.Equal(t, service.Spec.ClusterIP, ep.Targets[0])`). The discovery k8s.io `discovery/v1` API is not yet imported in `go.mod`.

## 5. Watches / enqueue mapping

`externalDNS.WatchedObject()` (`externaldns.go:42-53`) returns a stub `&v1.Service{}` with the configured name/namespace (nil for static targets). Mapping is `k8s.EnqueueMapFunc` (`internal/k8s/enqueue.go:14`): it filters events to that exact Service GVK/name/namespace (lines 23-27), then `List`s **all** IngressRoutes and enqueues every one (selector filtering happens in Reconcile). Note: watching EndpointSlices will need a new mapper (and EndpointSlice→owning Service→IngressRoute resolution), since the current filter compares one fixed object name.

## 6. Helm RBAC — `chart/templates/rbac.yaml`

Single ClusterRole (lines 8-39): `traefik.io/inggressroutes` get/list/watch; conditional `cert-manager.io/certificates` full; conditional `externaldns.k8s.io/dnsendpoints` full and core `services` get/list/watch (lines 26-29); plus configmaps/leases/events. **You must add `endpointslices` (apiGroup `discovery.k8s.io`, verbs get/list/watch) under the externalDNS block** (CRDs themselves are applied from upstream URLs in `pixi.toml cluster-setup`, not bundled).

## 7. Tests

- Unit/live: `internal/switchboard/targets_test.go` — `TestServiceTargetTargetsFromService` is a pure table-style test of the precedence chain (ideal place for headless cases; needs injection or an EndpointSlice-listing helper); `TestServiceTargetTargets` hits a real cluster (`k8tests.NewClient`, uses `~/.kube/config`, sleeps 10s for LB). `ingressroute_test.go` cases (`TestSimpleIngress`, `TestIngressNoTLS`, `TestIngressCustomDNS`, `TestIngressMultipleRules`, `TestIngressHeader`) create `DummyService` + IngressRoute, run Reconcile once, assert Certificate and DNSEndpoint target equals `service.Spec.ClusterIP` (line 246). No dual-stack or Service-update test exists yet; dual-stack is only asserted via two ClusterIPs in `targets_test.go:61`.
- e2e: `pixi run test-e2e image_name image_tag` → bats (`tests/deployment.bats`) against minikube; unit: `pixi run test-coverage` = `go test ./... -coverprofile cover.out`; `pixi run lint`.

## 8. CRD types / API groups

- IngressRoute: `traefik.io/v1alpha1` (`traefik.IngressRoute`), from `github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1`; key fields `Spec.Routes`, `Spec.TLS{SecretName}`, `Spec.EntryPoints`.
- DNSEndpoint: `externaldns.k8s.io/v1alpha1` (`externaldnsv1alpha1.DNSEndpoint`, `sigs.k8s.io/external-dns v0.23.0`), with `endpoint.Endpoint{DNSName, Targets, RecordType, RecordTTL}` from `sigs.k8s.io/external-dns/endpoint`.
- EndpointSlice (to add): `discovery.k8s.io/v1` in `k8s.io/api/discovery/v1` — filter `addressType: IPv4/IPv6`, `endpoints[].conditions.ready == true`, collect `endpoints[].addresses`.

**Implementation insertion points:** (a) headless detection + EndpointSlice query in `serviceTarget.Targets`/`targetsFromService` in `internal/switchboard/targets.go`; (b) EndpointSlice watch in `internal/controllers/utils.go` with a new mapper (likely in `internal/k8s/enqueue.go`); (c) RBAC rule in `chart/templates/rbac.yaml`; (d) tests in `targets_test.go` and a new service-update/dual-stack case in `ingressroute_test.go`.

让我读取关键文件以确认实现细节。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/switchboard/targets.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/externaldns.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/utils.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/k8s/enqueue.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/ingressroute.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/cmd/main.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/chart/templates/rbac.yaml

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/switchboard/targets_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/interface.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/k8tests/dummies.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/ingressroute_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/k8tests/client.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/go.mod

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/cmd/main_test.go

toolName: file_search

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/tests/deployment.bats

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/externaldns_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/pixi.toml

再看几个相关文件，确认集成接口与测试辅助。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/certmanager.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/utils_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/k8s/delete.go

代码结构已清楚。这是 borchero/switchboard 项目。我先建立任务清单，然后检查工具链与集群可用性。

toolName: todo_write

status: success

Todos updated: 7 items

toolName: Shell

status: success

toolName: Shell

status: success

无可用集群，纯单元测试本地可跑；live 测试沿用现有 `~/.kube/config` 模式编写。开始实现。

**1) 修改 `targets.go`：headless 检测 + EndpointSlice 汇总**

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/switchboard/targets.go

changes: undefined

**2) 扩展 Integration watch 接口并修改 external-dns 集成**

toolName: todo_write

status: success

Todos updated: 7 items

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/interface.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/certmanager.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/externaldns.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/externaldns.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/externaldns.go

changes: undefined

**3) 新增 label 匹配 mapper 并接线 EndpointSlice watch**

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/k8s/enqueue.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/utils.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/utils.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/utils.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/utils.go

changes: undefined

**4) main.go 显式注册 discovery/v1 scheme**

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/cmd/main.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/cmd/main.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/cmd/main.go

changes: undefined

**5) Helm RBAC 增加 endpointslices 权限**

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/chart/templates/rbac.yaml

changes: undefined

先编译确认接线无误，再补测试。

toolName: todo_write

status: success

Todos updated: 7 items

toolName: Shell

status: success

参数名 `client` 遮蔽了 client 包，重命名为 `ctrlClient`。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/switchboard/targets.go

changes: undefined

toolName: Shell

status: success

现在更新测试。先确认本地测试环境约束。

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/k8s/enqueue_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/k8s/delete_test.go

现在更新测试。先改 `targets_test.go`：适配新签名并增加 headless 纯单元测试与 fake-client 测试。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/switchboard/targets_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/switchboard/targets_test.go

changes: undefined

现在在纯测试末尾追加 headless 优先级用例，并新增汇总函数与 fake-client 测试。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/switchboard/targets_test.go

changes: undefined

更新 external-dns 集成测试：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/externaldns_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/externaldns_test.go

changes: undefined

新增 mapper 的 fake-client 测试（label 匹配，含创建/更新/删除事件对象）：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/k8s/enqueue_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/k8s/enqueue_test.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/utils.go

新增 reconciler 层 fake-client 测试（headless 生命周期 + 双栈普通 Service 回归 + cert 回归）：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/ingressroute_headless_test.go

changes: undefined

刚才的 helper 抽象写得过度且有误，重写为直接使用 `configv1.Config`：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/ingressroute_headless_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/ingressroute_headless_test.go

changes: undefined

toolName: Shell

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/externaldns_test.go

changes: undefined

toolName: Shell

status: success

编译和 vet 通过。运行不依赖集群的测试（fake client + 纯函数）：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

仅一个断言类型问题（`endpoint.Targets` 命名类型），改用 ElementsMatch：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/externaldns_test.go

changes: undefined

toolName: Shell

status: success

本地可运行测试全部通过。补充一个真实 apiserver 的 live 测试（沿用现有 harness，CI/minikube 运行），先看现有 chart values 与 README：

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/chart/values.yaml

toolName: search_by_regex

status: success

RBAC 无需文档更新。在 live 测试文件中追加一个真实 apiserver 的 headless 场景（沿用现有 harness）：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/switchboard/targets_test.go

changes: undefined

toolName: Shell

status: success

全部测试编译通过。验证 Helm chart 渲染（externalDNS 启用/禁用两种情况）：

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/chart/Chart.yaml

toolName: Shell

status: success

子 chart 未 vendor。在 /tmp 中复制 chart 并剥离依赖来验证模板渲染（不污染项目目录）：

toolName: Shell

status: success

RBAC 条件渲染正确。运行 lint：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

`go mod tidy` 无变更。最后通读关键改动文件并完整跑一遍所有非 live 测试。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/externaldns.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/utils.go

最终完整运行所有不依赖集群的测试：

toolName: Shell

status: success

全部通过。检查整体变更清单：

toolName: Shell

status: success

toolName: todo_write

status: success

Todos updated: 7 items

已完成。external-dns 目标指向 headless Service 时，现在从该 Service 的 EndpointSlice 汇总可发布地址，不再把 `None` 当 CNAME。

## 核心改动

**地址解析** [targets.go](file:///Users/vance/project/mineProject/swe/09101005/project-01/internal/switchboard/targets.go)
- `isHeadlessService` 以 `ClusterIP/ClusterIPs == "None"` 判定无头 Service；headless 时按 namespace + `kubernetes.io/service-name` 标签 list EndpointSlice
- `ReadyAddressesFromEndpointSlices`：仅收 IPv4/IPv6 切片中 `ready=true` 且非 `terminating` 的地址；防御性复核 service-name 标签（排除其他 Service）、`net.ParseIP` 校验、跨切片去重、排序稳定输出
- 选择顺序保持：annotation hostname → LB hostname/IP → headless 就绪地址（无就绪则为空）→ 普通 ClusterIP；静态 target 完全不动

**撤销/恢复** [externaldns.go](file:///Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/externaldns.go#L91-L100)
- targets 为空时删除陈旧 DNSEndpoint；地址恢复后的下一次 reconcile（由切片 watch 驱动）自动重建

**Watch 收敛**（切片创建/地址-readiness 更新/删除、headless↔普通切换）
- 接口 `WatchedObject()` 改为 `WatchedObjects()`（[interface.go](file:///Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/interface.go)、[certmanager.go](file:///Users/vance/project/mineProject/swe/09101005/project-01/internal/integrations/certmanager.go)），externalDNS 额外返回带 service-name 标签的 EndpointSlice 模板
- 新增 [EnqueueMapFuncMatching](file:///Users/vance/project/mineProject/swe/09101005/project-01/internal/k8s/enqueue.go#L46-L84)（原 `EnqueueMapFunc` 变为其薄封装，Service 精确匹配语义不变），[utils.go](file:///Users/vance/project/mineProject/swe/09101005/project-01/internal/controllers/utils.go#L91-L118) 按类型选择 mapper，切片的增/改/删均 enqueue 全部 IngressRoute

**生产支持**
- [main.go](file:///Users/vance/project/mineProject/swe/09101005/project-01/cmd/main.go#L104) 显式注册 `discovery.k8s.io/v1`
- [rbac.yaml](file:///Users/vance/project/mineProject/swe/09101005/project-01/chart/templates/rbac.yaml#L29-L31) externalDNS 启用时新增 `discovery.k8s.io/endpointslices` get/list/watch（已验证禁用时不渲染）

## 回归与验证
- 新增 fake-client/纯单元测试：切片过滤规则、headless 全生命周期（无地址撤销→恢复→切片删除→切普通双栈）、reconciler 层双栈普通 Service 仍走 ClusterIP、撤 DNS 时 Certificate 不受影响、mapper 标签匹配；另加一个 minikube live 用例 `TestServiceTargetTargetsHeadlessLive`
- 本地（无 ~/.kube/config）：`go build`、`go vet`、gofmt 干净，所有非 live 测试通过；live 测试全部编译通过，沿用既有 harness 待集群环境执行；`helm template` 确认既有安装结果仅增量增加一条 RBAC

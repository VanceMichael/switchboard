# 避免派生资源名称冲突时误接管

> **Workspace:** /Users/vance/project/mineProject/swe/09101005/project-03

---

**User**

共享命名空间中，平台团队可能预先保留与 IngressRoute 派生名称相同的 Certificate 或 DNSEndpoint；这些手工对象或其他 controller 的资源既不能被 Switchboard 改写，也不能在路由失去 TLS/host 后按名称误删。请让两种 integration 在创建、更新和清理前，以 controller ownerReference 的 kind、name 与 UID 判断资源身份：首选名称空闲或已经属于当前 IngressRoute 时继续沿用现有命名；若被无 owner、其他 controller 或旧 UID 对象占用，则完整保留冲突对象，并为当前 parent 选择确定、合法且在重试和 manager 重启后稳定的替代名称。控制器还要监听这些未受自己控制的同类资源；只有与某个路由首选名称有关的冲突对象发生更新或删除时，才精确触发该路由重新收敛。首选名称重新可用后，先成功创建或修复首选 child，再删除当前 UID 拥有的替代 child；首选创建失败时必须保留仍有效的替代资源并返回可重试错误，不能留下无可用派生资源的窗口。Certificate 与 DNSEndpoint 遵循同一身份和迁移规则，设置 owner 失败不得被吞掉。TLS secret 或 hosts 消失时只删除当前 UID 实际拥有的首选或替代 child；同名 IngressRoute 删除后以新 UID 重建不得接管旧 child。没有冲突的资源名称、managed-by/annotations、owned-child 事件恢复、证书和 DNS 结果保持兼容，无关的外来资源事件不得造成全量路由重建。

**Assistant**

我先了解项目结构和现有代码，再制定实施方案。

toolName: Shell

status: success

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/certmanager.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/externaldns.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/interface.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/utils.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/controllers/ingressroute.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/controllers/utils.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/k8s/enqueue.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/k8s/delete.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/certmanager_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/externaldns_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/k8tests/dummies.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/cmd/main.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/k8tests/client.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/controllers/ingressroute_test.go

现在查看其余相关文件与依赖版本。

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/k8s/enqueue_test.go

我已经理解了现有代码。这是对 switchboard 两个 integration（cert-manager / external-dns）的身份判定与冲突避让改造。先看一下 go 版本和辅助函数，然后制定任务清单。

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/ext/optionals.go

toolName: todo_write

status: success

Todos updated: 7 items

开始实现。先在 [utils.go](file:///Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/utils.go) 中加入共享的身份判定、替代命名与协调编排逻辑。

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/utils.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/utils.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/utils.go

changes: undefined

现在扩展 Integration 接口。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/interface.go

changes: undefined

改造 cert-manager integration。

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/certmanager.go

changes: undefined

改造 external-dns integration，同时修复 `reconcileMetadata` 错误被吞掉的问题。

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/externaldns.go

changes: undefined

现在为 k8s 包添加精确 enqueue mapper，并在 controller 中接线。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/k8s/enqueue.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/k8s/enqueue.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/k8s/enqueue.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/controllers/utils.go

changes: undefined

先编译验证。

toolName: Shell

status: success

编译通过。查看现有 `utils_test.go` 后补充测试。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/utils_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/switchboard/targets.go

toolName: todo_write

status: success

Todos updated: 7 items

现在编写基于 fake client 的单元测试（无需真实集群）。

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/ownership_test.go

changes: undefined

清理测试文件中几处粗糙写法，重写为更清晰的断言。

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/ownership_test.go

changes: undefined

为精确 enqueue mapper 补充 fake client 测试。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/k8s/enqueue_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/k8s/enqueue_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/k8s/enqueue_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/k8s/enqueue_test.go

changes: undefined

运行 vet 和新增测试。

toolName: Shell

status: success

toolName: Shell

status: success

三处测试问题（都是测试代码本身）：`BlockOwnerDeletion`、`endpoint.Targets` 类型、末尾重建外来对象的时序。修复。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/ownership_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/ownership_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/ownership_test.go

changes: undefined

toolName: Shell

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/ownership_test.go

changes: undefined

toolName: Shell

status: success

全部通过。再编译所有测试包并跑可离线的测试。

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

基线计数差异是因为 stash 时未跟踪的新测试文件仍在（引用新符号导致编译失败，少计 2 个）。快速核验基线失败原因同为缺少 kubeconfig。

toolName: Shell

status: success

剩余核实：controller-runtime v0.25 是否允许同一类型同时 `Owns` 和 `Watches`，以及 controllers 包是否有其它 Integration 实现。

toolName: Shell

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/go/pkg/mod/sigs.k8s.io/controller-runtime@v0.25.1/pkg/builder/controller.go

toolName: Shell

status: success

无重复 watch 限制。格式化并做最终 diff 审查。

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101005/project-03/.golangci.yml

toolName: Shell

status: success

查看完整最终 diff 做审查。

toolName: Shell

status: success

toolName: Shell

status: success

最终验证：build + vet + 全部可离线测试。

toolName: Shell

status: success

toolName: todo_write

status: success

Todos updated: 7 items

已完成。两种 integration 现在统一按 controller ownerReference 的 **kind/name/UID** 判定资源身份，Certificate 与 DNSEndpoint 共用同一套引擎。

## 改动概览

**共享协调引擎** — [utils.go](file:///Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/utils.go)
- `controlledBy`：仅当 controller 类 ownerRef 的 Kind、Name、UID 三者全匹配才视为己有；无 owner、其他 controller、旧 UID 一律视为外来对象。
- `AlternativeName`：`截断后的首选名 + "-" + sha256(UID)[:16]`，确定性生成、合法（RFC 1123、≤253）、重试和 manager 重启后稳定；同名 IngressRoute 以新 UID 重建必然得到不同替代名，不会接管旧 child。
- 协调规则（创建/更新/清理前统一执行）：
  - 首选名**空闲或属当前 UID** → 沿用首选名；
  - 首选名被**无 owner / 他 controller / 旧 UID** 占用 → 冲突对象原样保留（不打标签、不改 spec、不删除），改用替代名；
  - 首选重新可用 → **先 create/patch 首选 child 成功，再删本 UID 的替代 child**；首选失败时直接返回可重试错误，替代资源继续服役，无空窗；
  - TLS secret/hosts 消失 → 只列举并删除当前 UID 实控的首选/替代 child，同名外来对象完整保留。

**两个 integration** — [certmanager.go](file:///Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/certmanager.go)（首选名 `<route>-tls`）与 [externaldns.go](file:///Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/externaldns.go)（首选名 `<route>`）接入同一引擎；顺带修复了 DNSEndpoint 中 `reconcileMetadata` 失败时 `return nil` 吞掉 owner 设置错误的 bug；managed-by 标签、新建时继承 owner annotations、证书/DNS 结果均保持兼容。

**精确 watch** — [enqueue.go](file:///Users/vance/project/mineProject/swe/09101005/project-03/internal/k8s/enqueue.go) 新增 `EnqueuePreferredOwnerFunc`：把 child 名反解为路由名（cert 去 `-tls` 后缀、dns 同名），Get 确认该 IngressRoute 存在后**只 enqueue 这一个路由**；[utils.go](file:///Users/vance/project/mineProject/swe/09101005/project-03/internal/controllers/utils.go#L63-L75) 在既有 `Owns`（兼容 owned-child 事件恢复）之外对两类资源各加一个 `Watches`，外来冲突对象的增/改/删会精确触发关联路由收敛，无关外来事件不产生任何请求（不全量重建）。反解方法通过新接口 [interface.go](file:///Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/interface.go#L38-L43) 暴露。

**验证** — 新增 [ownership_test.go](file:///Users/vance/project/mineProject/swe/09101005/project-03/internal/integrations/ownership_test.go) 与 [enqueue_test.go](file:///Users/vance/project/mineProject/swe/09101005/project-03/internal/k8s/enqueue_test.go#L66-L120) 的 fake-client 用例，覆盖：外来/他控/旧 UID 避让、替代名稳定性、迁移顺序（操作记录断言 create 先于 delete）、首选创建失败保留替代、清理保留外来对象、同名重建不接管、精确 enqueue。`go build`、`go vet` 与全部离线测试通过；依赖真实集群（`~/.kube/config`）的既有集成测试本机无法运行，基线环境下同样失败，与本次改动无关。

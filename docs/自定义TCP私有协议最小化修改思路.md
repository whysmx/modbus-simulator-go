# 私有协议 V1 最小化修改方案

## 1. 结论

V1 先按**可落地优先**收敛范围：

- `connections` 新增 `service_type`
- 新增 `private_protocols` 表
- 私有协议与 Modbus 同级
- 私有协议内容单独接口保存，不和 Modbus 配置混在同一次提交里
- V1 只支持：**HEX 存储 + 完整匹配 + 分隔符分帧**
- 协议类型创建后**禁止切换**；如需改协议，必须新建连接

---

## 2. V1 范围约束

### 2.1 协议类型

`connections.service_type` 使用枚举值：

- `0`：Modbus
- `1`：私有协议

说明：

- 数据库字段名：`service_type`
- API 字段名：`serviceType`
- 创建连接时必须显式传入 `serviceType`
- 连接创建后，`serviceType` 不允许修改
- 如需改为另一种协议，必须新建连接
- 不再通过“有没有 `private_protocols`”推断协议类型
- 这样可以避免 Modbus / 私有协议并存、空配置、切换残留等歧义

### 2.2 私有协议规则能力

V1 只支持：

- `HEX` 存储
- `完整匹配`
- `分隔符分帧`

V1 暂不支持：

- ASCII 作为存储格式
- 包含匹配
- 定长分帧
- 长度字段分帧
- 空闲超时分帧

前端可以提供 ASCII 辅助展示或转换，但后端落库统一为：

- 大写 HEX
- 不带空格

例如：

- `AA 01 0D` -> `AA010D`

---

## 3. 数据结构

### 3.1 `connections` 表

新增字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `service_type` | `INTEGER` | 协议类型：`0=modbus`，`1=private_protocol` |

### 3.2 `private_protocols` 表

新增表：`private_protocols`

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | `TEXT` | 私有协议主键 |
| `conn_id` | `TEXT` | 所属连接 ID，关联 `connections.id` |
| `name` | `TEXT` | 私有协议名称 |
| `frame_delimiter_hex` | `TEXT` | 分隔符 HEX，例如 `0D`、`0D0A` |
| `rules_json` | `TEXT` | 多条规则 JSON |

约束：

- `conn_id` 唯一，一个连接最多一套私有协议配置
- 外键：`conn_id -> connections.id ON DELETE CASCADE`

### 3.3 `rules_json` 结构

每条规则建议包含：

- `name`
- `matchMode`
- `requestHex`
- `responseHex`
- `randomConfig`

枚举约定：

- `matchMode = 0`：完整匹配
- V1 仅允许 `0`

`randomConfig` 每项建议字段：

- `token`：如 `@1`
- `min`
- `max`
- `widthBytes`

随机数替换规则：

- 按整数生成随机值
- 按 `widthBytes` 转成固定长度 HEX
- 大端编码
- 不足补 `0`
- 大写输出

示例：

```json
[
  {
    "name": "规则1",
    "matchMode": 0,
    "requestHex": "AA01",
    "responseHex": "BB@1CC",
    "randomConfig": [
      { "token": "@1", "min": 1, "max": 999, "widthBytes": 2 }
    ]
  }
]
```

说明：

- `requestHex` / `responseHex` 都不包含分隔符
- 分隔符由 `frame_delimiter_hex` 统一管理

---

## 4. 运行规则

### 4.1 分帧

私有协议 V1 固定使用**分隔符分帧**：

- TCP 流按 `frame_delimiter_hex` 拆帧
- 未读到完整分隔符前，继续缓存
- 读到分隔符后，取分隔符前的数据作为一帧
- 分隔符本身不参与规则匹配

### 4.2 匹配与响应

当 `service_type = 1` 时：

1. 按 `frame_delimiter_hex` 拆帧
2. 将帧内容转为标准化 HEX
3. 按 `requestHex` 做完整匹配
4. 命中后，对 `responseHex` 做随机数替换
5. 回写 `responseHex + frame_delimiter_hex`

当 `service_type = 0` 时：

- 继续走现有 Modbus 流程

---

## 5. 接口设计

不再要求把私有协议整体塞进 `POST/PUT /api/connections`。

建议拆分为：

### 5.1 连接接口

继续用于保存连接基础信息：

- `GET /api/connections/tree`
- `POST /api/connections`
- `PUT /api/connections/:id`
- `DELETE /api/connections/:id`

约定：

- `POST /api/connections`：创建连接，必须传 `serviceType`
- `PUT /api/connections/:id`：只允许修改基础信息，如 `name`、`port`
- `PUT /api/connections/:id` 不允许修改 `serviceType`
- 若请求中传入的 `serviceType` 与已有值不同，后端返回 `400`

### 5.2 私有协议接口

新增私有协议专用接口：

- `GET /api/connections/:id/private-protocol`
- `PUT /api/connections/:id/private-protocol`
- `DELETE /api/connections/:id/private-protocol`

这样更符合当前前端“分区即时保存”的结构，改动也更直接。

---

## 6. 前端设计

连接编辑页分为两部分：

- 公共配置区：名称、端口、协议类型
- 协议配置区：标签切换 `Modbus配置` / `私有协议`

要求：

- 新建连接时，必须先选择协议类型
- 编辑已有连接时，协议类型只读禁用，不允许切换
- Modbus 与私有协议不混合展示
- 选择 `Modbus配置` 时，只展示从机/寄存器相关内容
- 选择 `私有协议` 时，只展示私有协议名称、分隔符、规则列表
- 编辑已有连接时，根据 `service_type` 默认选中对应标签

私有协议页主要内容：

- 私有协议名称
- 分隔符 HEX
- 多条规则编辑
- 每条规则的请求 HEX / 响应 HEX
- 随机数配置

前端可提供 ASCII 辅助输入/预览，但提交到后端前统一转成标准 HEX。

---

## 7. 切换规则

V1 不支持协议切换：

- `service_type` 在连接创建后即固定，不允许从 Modbus 改为私有协议
- `service_type` 在连接创建后即固定，不允许从私有协议改为 Modbus
- 如需使用另一种协议，必须新建连接

后端校验要求：

- `service_type = 0` 时，不允许保存 `private_protocols`
- `service_type = 1` 时，不允许继续使用 `slaves/registers`
- 如发现协议类型与配置内容不一致，后端直接拒绝保存

前端交互要求：

- 编辑连接时协议类型控件禁用
- 若用户想使用另一种协议，提示“请新建连接”

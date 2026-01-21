# API 接口规范

## 接口列表

### 组合查询
```
GET /api/connections/tree    获取连接+从机树形结构
```

### 连接管理
```
POST   /api/connections           创建连接
PUT    /api/connections/{id}      更新连接
DELETE /api/connections/{id}      删除连接
```

### 从机管理
```
POST   /api/connections/{id}/slaves              创建从机
PUT    /api/connections/{id}/slaves/{slaveId}    更新从机
DELETE /api/connections/{id}/slaves/{slaveId}    删除从机
```

### 寄存器管理
```
GET    /api/connections/{id}/slaves/{slaveId}/registers           获取寄存器列表
POST   /api/connections/{id}/slaves/{slaveId}/registers           创建寄存器组
PUT    /api/connections/{id}/slaves/{slaveId}/registers/{regId}   更新寄存器组
DELETE /api/connections/{id}/slaves/{slaveId}/registers/{regId}   删除寄存器组
```

## 数据结构

### Connection
```json
{
  "id": "0123456789abcdef0123456789abcdef",
  "name": "主连接",
  "port": 502,
  "protocolType": 0
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 32位无横杠UUID |
| name | string | 连接名称 |
| port | number | 端口号（自动分配，从502开始递增） |
| protocolType | number | 协议类型：0=Modbus RTU Over TCP（默认），1=Modbus TCP |

### Slave
```json
{
  "id": "00112233445566778899aabbccddeeff",
  "connId": "0123456789abcdef0123456789abcdef",
  "name": "从机1",
  "slaveId": 1
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 32位无横杠UUID |
| connId | string | 所属连接ID |
| name | string | 从机名称 |
| slaveId | number | 从机地址（1-247） |

### Register
```json
{
  "id": "0a1b2c3d4e5f60718293a4b5c6d7e8f9",
  "slaveId": "00112233445566778899aabbccddeeff",
  "startAddr": 40001,
  "hexData": "01020304",
  "names": "Temperature,Humidity",
  "coefficients": "0.1,0.01"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 32位无横杠UUID |
| slaveId | string | 所属从机ID |
| startAddr | number | 起始逻辑地址 |
| hexData | string | 连续数据的16进制字符串（大写、无0x前缀） |
| names | string | 逗号分隔的寄存器名称（可选） |
| coefficients | string | 逗号分隔的显示系数（可选） |

#### hexData 格式说明
- 当 `startAddr ∈ [30001–39999]` 或 `[40001–49999]`（功能码04/03，寄存器型）：
  - 长度为4的倍数
  - 寄存器数量 = hexData.length / 4（每4个hex=1个16位寄存器）
- 当 `startAddr ∈ [00001–09999]` 或 `[10001–19999]`（功能码01/02，位型）：
  - 长度为2的倍数
  - 覆盖位数 = (hexData.length / 2) * 8（每2个hex=1字节=8位）

### ConnectionTree
```json
{
  "id": "0123456789abcdef0123456789abcdef",
  "name": "主连接",
  "port": 502,
  "protocolType": 0,
  "slaves": [
    {
      "id": "00112233445566778899aabbccddeeff",
      "connId": "0123456789abcdef0123456789abcdef",
      "name": "从机1",
      "slaveId": 1
    }
  ]
}
```

## 响应规范

### 成功响应

| 操作 | 状态码 | Body |
|------|--------|------|
| 列表查询 | 200 OK | 资源数组（如 Connection[] / Slave[] / Register[]） |
| 树查询 | 200 OK | ConnectionTree[] |
| 创建 | 201 Created | 新建实体 |
| 更新 | 200 OK | 更新后实体 |
| 删除 | 204 No Content | 无 |

### 错误响应

```json
{
  "error": "具体错误描述",
  "code": 400
}
```

| 状态码 | 说明 |
|--------|------|
| 400 Bad Request | 业务验证失败（如端口已被使用、参数错误等） |
| 404 Not Found | 资源不存在 |

### 错误示例
```
PUT /api/connections/abc123 端口冲突
→ 400 { "error": "端口已被使用", "code": 400 }

PUT /api/connections/notfound 连接不存在
→ 404 { "error": "连接不存在", "code": 404 }

POST /api/connections/{id}/slaves/{slaveId}/registers 地址重叠
→ 400 { "error": "地址范围与已有记录重叠", "code": 400 }
```

## 接口详情

### GET /api/connections/tree

获取连接和从机的树形结构（不包含寄存器）。

**响应示例：**
```json
[
  {
    "id": "0123456789abcdef0123456789abcdef",
    "name": "主连接",
    "port": 502,
    "protocolType": 0,
    "slaves": [
      { "id": "00112233445566778899aabbccddeeff", "connId": "0123456789abcdef0123456789abcdef", "name": "从机1", "slaveId": 1 },
      { "id": "ffeeddccbbaa99887766554433221100", "connId": "0123456789abcdef0123456789abcdef", "name": "从机2", "slaveId": 2 }
    ]
  }
]
```

### POST /api/connections

创建新连接。端口自动分配（从502开始递增）。

**请求：**
```json
{
  "name": "新连接",
  "protocolType": 0
}
```

**响应：** 201 Created
```json
{
  "id": "abc123...",
  "name": "新连接",
  "port": 503,
  "protocolType": 0
}
```

### GET /api/connections/{id}/slaves/{slaveId}/registers

获取指定从机下的所有寄存器组。

**响应示例：**
```json
[
  {
    "id": "0a1b2c3d4e5f60718293a4b5c6d7e8f9",
    "slaveId": "00112233445566778899aabbccddeeff",
    "startAddr": 40001,
    "hexData": "01020304",
    "names": "Temperature,Humidity",
    "coefficients": "0.1,0.01"
  },
  {
    "id": "9f8e7d6c5b4a3928171605f4e3d2c1b0",
    "slaveId": "00112233445566778899aabbccddeeff",
    "startAddr": 30001,
    "hexData": "DEADBEEF",
    "names": ",Pressure",
    "coefficients": ",2.0"
  }
]
```

### POST /api/connections/{id}/slaves/{slaveId}/registers

创建寄存器组。

**请求：**
```json
{
  "startAddr": 40001,
  "hexData": "00000000",
  "names": "",
  "coefficients": ""
}
```

**响应：** 201 Created

## 地址区间与类型映射

前端根据 `startAddr` 字段自动分类寄存器类型：

| 寄存器类型 | 功能码 | 地址范围 |
|------------|--------|----------|
| 线圈（Coil） | FC01 | 00001–09999 |
| 离散输入（Discrete Input） | FC02 | 10001–19999 |
| 输入寄存器（Input Register） | FC04 | 30001–39999 |
| 保持寄存器（Holding Register） | FC03 | 40001–49999 |

## 数据加载策略

1. **初始加载（两层）**：使用 `GET /api/connections/tree` 一次性获取连接和从机数据（不包含寄存器）
2. **展开加载（第三层）**：首次展开某从机时调用 `GET /api/connections/{id}/slaves/{slaveId}/registers`
3. **前端分类**：根据 `startAddr` 按地址范围自动分类到不同寄存器类型节点

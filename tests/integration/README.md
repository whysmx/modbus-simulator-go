# Modbus 模拟器集成测试

本目录包含 Modbus TCP 协议的集成测试,使用真实的 TCP 连接测试模拟器的功能。

## 测试内容

### TestModbusTCPCommunication
完整的 Modbus TCP 通信集成测试,包括:

1. **CreateAndReadHoldingRegisters** - 创建并读取保持寄存器
   - 创建寄存器(地址 40001,数据 0x000A, 0x0014)
   - 通过 handler 直接读取验证数据正确性
   - 验证读取到值 10 和 20

2. **ReadCoils** - 读取线圈
   - 创建线圈寄存器(地址 1,数据 0xA5)
   - 验证读取到正确的位数据

3. **FullTCPCommunication** - 完整 TCP 通信测试
   - 使用真实 TCP 客户端连接到测试服务器
   - 发送标准 Modbus TCP 请求(MBAP 头 + PDU)
   - 验证响应的 MBAP 头(事务 ID、协议 ID、长度、单元 ID)
   - 验证功能码和数据内容
   - 确认端到端通信正常

### 测试架构
- **测试服务器**: 在 127.0.0.1:15021 启动 TCP listener
- **数据存储**: 使用内存存储创建测试数据(连接、从站、寄存器)
- **Modbus Handler**: 使用真实的 ModbusHandler 处理请求
- **TCP 客户端**: 原始 TCP 连接,手动构建 Modbus TCP 帧

### 支持的功能码
- `0x01` - Read Coils (读线圈)
- `0x02` - Read Discrete Inputs (读离散输入)
- `0x03` - Read Holding Registers (读保持寄存器)
- `0x04` - Read Input Registers (读输入寄存器)

## 运行测试

### 运行所有集成测试
```bash
go test ./tests/integration/... -v
```

### 运行特定的子测试
```bash
# 只测试完整 TCP 通信
go test ./tests/integration/... -v -run TestModbusTCPCommunication/FullTCPCommunication

# 只测试保持寄存器
go test ./tests/integration/... -v -run TestModbusTCPCommunication/CreateAndReadHoldingRegisters
```

### 查看测试覆盖率
```bash
go test ./tests/integration/... -cover
```

## 测试原理

1. **准备测试环境**
   - 创建内存存储
   - 创建连接(connID)、从站(slaveID)、寄存器
   - 为所有实体生成 UUID

2. **启动测试服务器**
   - 在固定端口(15021)启动 TCP listener
   - 使用 goroutine 处理传入连接
   - 解析 Modbus TCP 请求并调用 handler

3. **执行测试用例**
   - 直接调用 handler 方法验证数据层
   - 使用 TCP 客户端发送原始 Modbus 帧
   - 验证响应格式和数据内容

4. **验证响应**
   - MBAP 头: Transaction ID, Protocol ID, Length, Unit ID
   - PDU: Function Code, Byte Count, Data
   - 数据内容: 寄存器值正确(大端序)

## 关键实现细节

### Modbus TCP 帧结构
```
[MBAP Header - 7 bytes]
  Transaction ID (2 bytes)
  Protocol ID (2 bytes, always 0)
  Length (2 bytes)
  Unit ID (1 byte)

[PDU - variable]
  Function Code (1 byte)
  Data (depends on function code)
```

### Read Holding Registers 响应
```
Function Code: 0x03
Byte Count: n
Register Data[0]: high byte, low byte
Register Data[1]: high byte, low byte
...
```

### 地址映射
- PDU 地址 0 → 逻辑地址 40001 (保持寄存器)
- PDU 地址 0 → 逻辑地址 1 (线圈)
- Handler 自动处理地址转换

## 扩展测试

可以添加更多测试:
- **写操作**: Write Single Coil (0x05), Write Single Register (0x06)
- **多写操作**: Write Multiple Coils (0x0F), Write Multiple Registers (0x10)
- **异常处理**: 非法地址、非法功能码、设备故障
- **边界条件**: 最大寄存器数量、超长请求
- **性能测试**: 并发连接、大数据传输
- **稳定性测试**: 长时间运行、连接重连

## 测试覆盖

✅ **单元测试**: `internal/` 目录下的包测试
✅ **集成测试**: 本目录的 Modbus TCP 通信测试
✅ **前端测试**: Web UI 的 Playwright 测试(见 `docs/frontend-test-results.md`

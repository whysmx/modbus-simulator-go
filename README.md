# modbus-simulator-go 私有 TCP 协议完整变更代码

这个包包含本次功能涉及的所有完整文件，路径与仓库根目录一致。

使用方法：

```bash
cd modbus-simulator-go
# 解压本包后，把文件覆盖到仓库根目录
cp -R /path/to/modbus_complete_code_bundle/* .

gofmt -w $(find internal -name '*.go')
node --check web/static/js/private_protocol.js
node --check internal/web/static/js/private_protocol.js
go test ./...

git add .
git commit -m "feat: add private TCP protocol support"
git push origin main
```

注意：我在当前沙箱里无法访问 GitHub 和 Go proxy，所以未能执行完整仓库级 go test，也未能 push。前面暴露过的 GitHub token 请立即撤销并重新生成。

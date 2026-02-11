// State
let deviceTree = [];
let registerCache = {};
let openTabs = [];
let activeTab = null;
// Changes are persisted immediately on edit.
let expandedNodes = new Set(); // 存储展开的节点ID
let importInProgress = false;
let selectedTreeKey = null;

// API functions
const api = {
    async getTree() {
        const res = await fetch('/api/connections/tree');
        return res.json();
    },
    async createConnection(data) {
        const res = await fetch('/api/connections', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(data)
        });
        if (!res.ok) throw await res.json();
        return res.json();
    },
    async updateConnection(id, data) {
        const res = await fetch(`/api/connections/${id}`, {
            method: 'PUT',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(data)
        });
        if (!res.ok) throw await res.json();
        return res.json();
    },
    async deleteConnection(id) {
        const res = await fetch(`/api/connections/${id}`, {method: 'DELETE'});
        if (!res.ok) throw await res.json();
    },
    async createSlave(connId, data) {
        const res = await fetch(`/api/connections/${connId}/slaves`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(data)
        });
        if (!res.ok) throw await res.json();
        return res.json();
    },
    async updateSlave(connId, slaveId, data) {
        const res = await fetch(`/api/connections/${connId}/slaves/${slaveId}`, {
            method: 'PUT',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(data)
        });
        if (!res.ok) throw await res.json();
        return res.json();
    },
    async deleteSlave(connId, slaveId) {
        const res = await fetch(`/api/connections/${connId}/slaves/${slaveId}`, {method: 'DELETE'});
        if (!res.ok) throw await res.json();
    },
    async getRegisters(connId, slaveId) {
        const res = await fetch(`/api/connections/${connId}/slaves/${slaveId}/registers`);
        return res.json();
    },
    async getRegister(connId, slaveId, regId) {
        const res = await fetch(`/api/connections/${connId}/slaves/${slaveId}/registers/${regId}`);
        if (!res.ok) throw await res.json();
        return res.json();
    },
    async createRegister(connId, slaveId, data) {
        const res = await fetch(`/api/connections/${connId}/slaves/${slaveId}/registers`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(data)
        });
        if (!res.ok) throw await res.json();
        return res.json();
    },
    async updateRegister(connId, slaveId, regId, data) {
        const res = await fetch(`/api/connections/${connId}/slaves/${slaveId}/registers/${regId}`, {
            method: 'PUT',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(data)
        });
        if (!res.ok) throw await res.json();
        return res.json();
    },
    async deleteRegister(connId, slaveId, regId) {
        const res = await fetch(`/api/connections/${connId}/slaves/${slaveId}/registers/${regId}`, {method: 'DELETE'});
        if (!res.ok) throw await res.json();
    }
};

// Register type helpers
const regTypes = [
    {title: '01线圈Coils', range: '[00001-09999]', fc: 1, min: 1, max: 9999},
    {title: '02离散输入Discrete Inputs', range: '[10001-19999]', fc: 2, min: 10001, max: 19999},
    {title: '04输入寄存器Input Registers', range: '[30001-39999]', fc: 4, min: 30001, max: 39999},
    {title: '03保持寄存器Holding Registers', range: '[40001-105536]', fc: 3, min: 40001, max: 105536}
];

const regTypeLabels = {
    1: {cn: '线圈', en: 'Coil'},
    2: {cn: '离散输入', en: 'Discrete Input'},
    3: {cn: '保持寄存器', en: 'Holding Register'},
    4: {cn: '输入寄存器', en: 'Input Register'}
};

function formatRegTypeLabel(rt) {
    const label = regTypeLabels[rt.fc] || {cn: rt.title, en: ''};
    const fcLabel = String(rt.fc).padStart(2, '0');
    return `<span class="regtype-line"><span class="regtype-fc">${fcLabel}</span><span class="regtype-range"> ${label.en}</span> <span class="regtype-title">${label.cn}</span></span>`;
}

function formatRegTypeTitle(rt) {
    const label = regTypeLabels[rt.fc] || {cn: rt.title, en: ''};
    const fcLabel = String(rt.fc).padStart(2, '0');
    return `<span class="regtype-title">${label.cn}</span><br><span class="regtype-range">${fcLabel}:${label.en}</span>`;
}

function formatSlaveAddr(addr) {
    return String(addr).padStart(2, '0');
}

function buildTreeKey(type, connId, slaveId, fc) {
    return [type, connId || '', slaveId || '', fc || ''].join('|');
}

function applyTreeSelection() {
    const container = document.getElementById('deviceTree');
    if (!container) return;
    container.querySelectorAll('.tree-node-content.selected').forEach(el => el.classList.remove('selected'));
    if (!selectedTreeKey) return;
    const selectedEl = container.querySelector(`.tree-node-content[data-key="${selectedTreeKey}"]`);
    if (selectedEl) selectedEl.classList.add('selected');
}

function setSelectedTreeKey(key) {
    selectedTreeKey = key;
    applyTreeSelection();
}

function getHostLabel() {
    return window.location.hostname || 'localhost';
}

function formatNumber(value, maxDecimals = 4) {
    if (value === null || value === undefined) return 'NA';
    const num = Number(value);
    if (!Number.isFinite(num)) return 'NA';
    const abs = Math.abs(num);
    if (abs >= 1e6) {
        return num
            .toExponential(maxDecimals)
            .replace(/\.?0+e/, 'e');
    }
    const fixed = num.toFixed(maxDecimals);
    const trimmed = fixed.replace(/\.?0+$/, '');
    return trimmed === '-0' ? '0' : trimmed;
}

function formatHexAddr(addr, fc) {
    if (addr === null || addr === undefined) return '';
    const num = Number(addr);
    if (!Number.isFinite(num)) return '';
    let base = 1;
    switch (fc) {
        case 2:
            base = 10001;
            break;
        case 4:
            base = 30001;
            break;
        case 3:
            base = 40001;
            break;
        case 1:
        default:
            base = 1;
            break;
    }
    let offset = Math.trunc(num - base + 1);
    if (!Number.isFinite(offset) || offset < 1) {
        offset = Math.trunc(num);
    }
    return offset.toString(16).toUpperCase().padStart(4, '0');
}

function getRegType(startAddr) {
    return regTypes.find(t => startAddr >= t.min && startAddr <= t.max);
}

function getRegTypeByFc(fc) {
    return regTypes.find(t => t.fc === fc);
}

function getLogicalStart(fc, pduAddr) {
    switch (fc) {
        case 1:
            return pduAddr + 1;
        case 2:
            return pduAddr + 10001;
        case 4:
            return pduAddr + 30001;
        case 3:
            return pduAddr + 40001;
        default:
            return pduAddr + 1;
    }
}

// Tree rendering
function renderTree() {
    const container = document.getElementById('deviceTree');
    container.innerHTML = '';

    deviceTree.forEach(node => {
        node.slaves.forEach(slave => {
            const endpointEl = createEndpointNode(node.connection, slave);
            container.appendChild(endpointEl);
        });
    });

    applyTreeSelection();
}

function createEndpointNode(conn, slave) {
    const el = document.createElement('div');
    el.className = 'tree-node';
    const slaveNodeId = `${conn.id}-${slave.id}`;
    const slaveKey = buildTreeKey('slave', conn.id, slave.id);
    const isExpanded = expandedNodes.has(slaveNodeId);
    el.innerHTML = `
        <div class="tree-node-content" data-type="slave" data-conn-id="${conn.id}" data-id="${slave.id}" data-key="${slaveKey}">
            <span class="tree-expand">${isExpanded ? '▼' : '▶'}</span>
            <span class="tree-label tree-label-endpoint">
                <span class="tree-label-main">${slave.name}</span>
                <span class="tree-label-sub">${getHostLabel()}:${conn.port} 从机地址:${formatSlaveAddr(slave.slaveAddr)}</span>
            </span>
            <div class="tree-actions">
                <button class="btn btn-sm" onclick="event.stopPropagation(); showAddRegister('${conn.id}', '${slave.id}')">添加数据</button>
                <button class="btn btn-sm" onclick="event.stopPropagation(); copyDevice('${conn.id}', '${slave.id}')">复制</button>
                <button class="btn btn-sm btn-danger" onclick="event.stopPropagation(); deleteDevice('${conn.id}', '${slave.id}')">删除</button>
            </div>
        </div>
        <div class="tree-children ${isExpanded ? '' : 'collapsed'}"></div>
    `;

    const childrenContainer = el.querySelector('.tree-children');
    regTypes.forEach(rt => {
        const rtEl = document.createElement('div');
        rtEl.className = 'tree-node';
        const rtKey = buildTreeKey('regtype', conn.id, slave.id, rt.fc);
        rtEl.innerHTML = `
            <div class="tree-node-content" data-type="regtype" data-conn-id="${conn.id}" data-slave-id="${slave.id}" data-fc="${rt.fc}" data-key="${rtKey}">
                <span class="tree-expand"></span>
                <span class="tree-label">${formatRegTypeLabel(rt)}</span>
            </div>
        `;
        rtEl.querySelector('.tree-node-content').onclick = () => {
            setSelectedTreeKey(rtKey);
            openRegisterTab(conn.id, slave, rt);
        };
        childrenContainer.appendChild(rtEl);
    });

    updateRegTypeCounts(conn.id, slave.id, childrenContainer);

    el.querySelector('.tree-node-content').onclick = () => {
        setSelectedTreeKey(slaveKey);
        toggleNodeAndOpen(el, slaveNodeId, conn, slave);
    };

    return el;
}

async function updateRegTypeCounts(connId, slaveId, container) {
    let regs = registerCache[slaveId];
    if (!regs) {
        try {
            regs = await api.getRegisters(connId, slaveId);
            registerCache[slaveId] = regs;
        } catch {
            regs = [];
        }
    }

    const counts = {};
    regTypes.forEach(rt => {
        counts[rt.fc] = 0;
    });

    regs.forEach(reg => {
        const rt = getRegType(reg.startAddr);
        if (!rt) return;
        if (rt.fc === 1 || rt.fc === 2) {
            counts[rt.fc] += Math.floor((reg.hexData.length / 2) * 8);
        } else {
            counts[rt.fc] += Math.floor(reg.hexData.length / 4);
        }
    });

    regTypes.forEach(rt => {
        const count = counts[rt.fc] || 0;
        const el = container.querySelector(`.regtype-count[data-fc="${rt.fc}"]`);
        if (el) {
            el.textContent = count > 0 ? ` (${count})` : '';
        }
        const row = container.querySelector(`.tree-node-content[data-fc="${rt.fc}"]`);
        if (row) {
            const node = row.closest('.tree-node');
            if (node) node.style.display = count > 0 ? '' : 'none';
        }
    });

    const hasChildren = Array.from(container.children).some(child => child.style.display !== 'none');
    const parentNode = container.closest('.tree-node');
    const parentContent = parentNode?.querySelector('.tree-node-content');
    const expand = parentContent?.querySelector('.tree-expand');
    if (expand) {
        if (hasChildren) {
            const isCollapsed = container.classList.contains('collapsed');
            expand.textContent = isCollapsed ? '▶' : '▼';
        } else {
            expand.textContent = '';
            container.classList.add('collapsed');
        }
    }
    if (parentContent) {
        parentContent.dataset.hasChildren = hasChildren ? '1' : '0';
    }
}

function toggleNode(el, nodeId) {
    const children = el.querySelector('.tree-children');
    const content = el.querySelector('.tree-node-content');
    const expand = el.querySelector('.tree-expand');
    if (content && content.dataset.hasChildren === '0') {
        if (expand) expand.textContent = '';
        return;
    }
    if (children) {
        const isCollapsed = children.classList.contains('collapsed');
        children.classList.toggle('collapsed');
        const nowCollapsed = children.classList.contains('collapsed');

        if (nowCollapsed && !isCollapsed) {
            // 折叠:从集合中移除
            expandedNodes.delete(nodeId);
        } else if (!nowCollapsed && isCollapsed) {
            // 展开:添加到集合
            expandedNodes.add(nodeId);
        }

        expand.textContent = nowCollapsed ? '▶' : '▼';
    }
}

async function toggleNodeAndOpen(el, nodeId, conn, slave) {
    toggleNode(el, nodeId);
    await openFirstRegTypeIfAny(conn, slave);
}

async function openFirstRegTypeIfAny(conn, slave) {
    let regs = registerCache[slave.id];
    if (!regs) {
        try {
            regs = await api.getRegisters(conn.id, slave.id);
            registerCache[slave.id] = regs;
        } catch {
            regs = [];
        }
    }

    const counts = {};
    regTypes.forEach(rt => {
        counts[rt.fc] = 0;
    });

    regs.forEach(reg => {
        const rt = getRegType(reg.startAddr);
        if (!rt) return;
        if (rt.fc === 1 || rt.fc === 2) {
            counts[rt.fc] += Math.floor((reg.hexData.length / 2) * 8);
        } else {
            counts[rt.fc] += Math.floor(reg.hexData.length / 4);
        }
    });

    const firstRt = regTypes.find(rt => (counts[rt.fc] || 0) > 0);
    if (!firstRt) return;
    openRegisterTab(conn.id, slave, firstRt);
}

// Tab management
function openRegisterTab(connId, slave, regType) {
    const tabId = `${slave.id}-${regType.fc}`;
    const tab = {
        id: tabId,
        connId,
        slave,
        regType,
        title: `${slave.name} - ${formatRegTypeTitle(regType)}`
    };

    openTabs = [tab];
    activeTab = tabId;
    renderTabs();
    loadTabContent(tab);
}

function renderTabs() {
    const container = document.getElementById('tabHeaders');
    container.innerHTML = '';
}

function selectTab(tabId) {
    activeTab = tabId;
    renderTabs();
    const tab = openTabs.find(t => t.id === tabId);
    if (tab) loadTabContent(tab);
}

function closeTab(tabId) {
    openTabs = openTabs.filter(t => t.id !== tabId);
    if (activeTab === tabId) {
        activeTab = openTabs.length ? openTabs[0].id : null;
    }
    renderTabs();
    if (activeTab) {
        const tab = openTabs.find(t => t.id === activeTab);
        if (tab) loadTabContent(tab);
    } else {
        document.getElementById('tabContent').innerHTML = '<div class="empty-state">请从设备树中选择寄存器类型</div>';
    }
}

function closeTabsByConnection(connId) {
    const tabsToClose = openTabs.filter(t => t.connId === connId);
    if (tabsToClose.length === 0) return;

    openTabs = openTabs.filter(t => t.connId !== connId);
    if (tabsToClose.some(t => t.id === activeTab)) {
        activeTab = openTabs.length ? openTabs[0].id : null;
    }
    renderTabs();
    if (activeTab) {
        const tab = openTabs.find(t => t.id === activeTab);
        if (tab) loadTabContent(tab);
    } else {
        document.getElementById('tabContent').innerHTML = '<div class="empty-state">请从设备树中选择寄存器类型</div>';
    }
}

function closeTabsBySlave(slaveId) {
    const tabsToClose = openTabs.filter(t => t.slave && t.slave.id === slaveId);
    if (tabsToClose.length === 0) return;

    openTabs = openTabs.filter(t => !(t.slave && t.slave.id === slaveId));
    if (tabsToClose.some(t => t.id === activeTab)) {
        activeTab = openTabs.length ? openTabs[0].id : null;
    }
    renderTabs();
    if (activeTab) {
        const tab = openTabs.find(t => t.id === activeTab);
        if (tab) loadTabContent(tab);
    } else {
        document.getElementById('tabContent').innerHTML = '<div class="empty-state">请从设备树中选择寄存器类型</div>';
    }
}

async function loadTabContent(tab) {
    const content = document.getElementById('tabContent');
    const cacheKey = tab.slave.id;

    if (!registerCache[cacheKey]) {
        try {
            registerCache[cacheKey] = await api.getRegisters(tab.connId, tab.slave.id);
        } catch (e) {
            registerCache[cacheKey] = [];
        }
    }

    const regs = registerCache[cacheKey].filter(r => {
        const rt = getRegType(r.startAddr);
        return rt && rt.fc === tab.regType.fc;
    }).slice().sort((a, b) => a.startAddr - b.startAddr);

    const isBitType = tab.regType.fc === 1 || tab.regType.fc === 2;
    const colCount = isBitType ? 5 : 11;

    content.innerHTML = `
        <div class="table-wrap">
            <table class="register-table ${isBitType ? 'is-bit' : 'is-word'}">
                <thead>
                    <tr>
                        <th>地址</th>
                        <th>Hex地址</th>
                        <th>${isBitType ? '位' : 'Hex'}</th>
                        ${isBitType ? '' : '<th>Int16</th><th>UInt16</th><th><div class="th-title">ABCD</div><div class="th-sub">正序/大端</div></th><th><div class="th-title">BADC</div><div class="th-sub">单字反转</div></th><th><div class="th-title">CDAB</div><div class="th-sub">双字反转/PLC顺序</div></th><th><div class="th-title">DCBA</div><div class="th-sub">反转/小端</div></th>'}
                        <th>名称</th>
                        <th>操作</th>
                    </tr>
                </thead>
                <tbody>
                    ${regs.length ? renderRegisterRows(regs, tab, isBitType) : `<tr><td colspan="${colCount}" style="text-align:center">暂无寄存器</td></tr>`}
                </tbody>
            </table>
        </div>
    `;
}

function renderRegisterRows(regs, tab, isBitType) {
    let html = '';
    let prevAddr = null;
    let segmentIndex = 0;
    regs.forEach(reg => {
        const names = reg.names ? reg.names.split(',') : [];
        const coeffs = reg.coefficients ? reg.coefficients.split(',').map(parseFloat) : [];

        if (isBitType) {
            // Bit type: each byte = 8 bits
            const bytes = hexToBytes(reg.hexData);
            let bitIndex = 0;
            bytes.forEach((b, byteIdx) => {
                for (let i = 0; i < 8; i++) {
                    const addr = reg.startAddr + bitIndex;
                    if (prevAddr !== null && addr !== prevAddr + 1) {
                        segmentIndex++;
                    }
                    prevAddr = addr;
                    const segClass = segmentIndex % 2;
                    const bitVal = (b >> i) & 1;
                    const name = names[bitIndex] || '';
                    html += `
                        <tr class="addr-seg-${segClass}" data-reg-id="${reg.id}" data-bit-idx="${bitIndex}">
                            <td class="addr-cell">${addr}</td>
                            <td class="addr-hex-cell">${formatHexAddr(addr, tab.regType.fc)}</td>
                            <td>
                                <input type="checkbox" ${bitVal ? 'checked' : ''}
                                    onchange="updateBit('${tab.connId}', '${tab.slave.id}', '${reg.id}', ${bitIndex}, this.checked)">
                            </td>
                            <td>${name}</td>
                            <td>
                                <button class="btn-icon" onclick="showEditRegister('${tab.connId}', '${tab.slave.id}', '${reg.id}')">✏️</button>
                            </td>
                        </tr>
                    `;
                    bitIndex++;
                }
            });
        } else {
            // Register type: 4 hex = 1 register
            const regCount = reg.hexData.length / 4;
            for (let i = 0; i < regCount; i++) {
                const addr = reg.startAddr + i;
                if (prevAddr !== null && addr !== prevAddr + 1) {
                    segmentIndex++;
                }
                prevAddr = addr;
                const segClass = segmentIndex % 2;
                const hexVal = reg.hexData.substr(i * 4, 4);
                const int16Val = hexToInt16(hexVal);
                const uint16Val = hexToUint16(hexVal);
                const coeff = coeffs[i] || 1;
                const name = names[i] || '';

                // Float32 (sliding window)
                let float32 = 'NA';
                let float32Badc = 'NA';
                let float32Cdab = 'NA';
                let float32Dcba = 'NA';
                if (i < regCount - 1) {
                    const hex32 = reg.hexData.substr(i * 4, 8);
                    float32 = hexToFloat32(hex32);
                    float32Badc = hexToFloat32Order(hex32, 'BADC');
                    float32Cdab = hexToFloat32Order(hex32, 'CDAB');
                    float32Dcba = hexToFloat32Order(hex32, 'DCBA');
                }

                html += `
                    <tr class="addr-seg-${segClass}" data-reg-id="${reg.id}" data-reg-idx="${i}">
                        <td class="addr-cell">${addr}</td>
                        <td class="addr-hex-cell">${formatHexAddr(addr, tab.regType.fc)}</td>
                        <td>
                            <input type="text" value="${hexVal}" maxlength="4" pattern="[0-9A-Fa-f]{4}"
                                onchange="updateHex('${tab.connId}', '${tab.slave.id}', '${reg.id}', ${i}, this.value)">
                        </td>
                        <td>${formatNumber(int16Val * coeff)}</td>
                        <td>${formatNumber(uint16Val * coeff)}</td>
                        <td>${float32}</td>
                        <td>${float32Badc}</td>
                        <td>${float32Cdab}</td>
                        <td>${float32Dcba}</td>
                        <td>${name}</td>
                        <td>
                            ${i === 0 ? `<button class="btn-icon" onclick="showEditRegister('${tab.connId}', '${tab.slave.id}', '${reg.id}')">✏️</button>` : ''}
                            ${i === 0 ? `` : ''}
                        </td>
                    </tr>
                `;
            }
        }
    });
    return html;
}

// Hex conversion helpers
function hexToBytes(hex) {
    const bytes = [];
    for (let i = 0; i < hex.length; i += 2) {
        bytes.push(parseInt(hex.substr(i, 2), 16));
    }
    return bytes;
}

function bytesToHex(bytes) {
    return bytes.map(b => b.toString(16).padStart(2, '0').toUpperCase()).join('');
}

function hexToInt16(hex) {
    const val = parseInt(hex, 16);
    return val > 0x7FFF ? val - 0x10000 : val;
}

function hexToUint16(hex) {
    return parseInt(hex, 16);
}

// Modbus CRC16 (RTU)
function crc16(bytes) {
    let crc = 0xFFFF;
    for (const b of bytes) {
        crc ^= b;
        for (let i = 0; i < 8; i++) {
            if (crc & 0x0001) {
                crc = (crc >> 1) ^ 0xA001;
            } else {
                crc >>= 1;
            }
        }
    }
    return crc & 0xFFFF;
}

function validateCRC(bytes) {
    if (bytes.length < 3) return false;
    const data = bytes.slice(0, -2);
    const crc = crc16(data);
    const lo = bytes[bytes.length - 2];
    const hi = bytes[bytes.length - 1];
    return lo === (crc & 0xFF) && hi === ((crc >> 8) & 0xFF);
}

function hexToFloat32(hex) {
    return hexToFloat32Order(hex, 'ABCD');
}

function hexToFloat32Order(hex, order) {
    if (hex.length !== 8) return 'NA';
    try {
        const bytes = hexToBytes(hex);
        let idx;
        switch (order) {
            case 'BADC':
                idx = [1, 0, 3, 2];
                break;
            case 'CDAB':
                idx = [2, 3, 0, 1];
                break;
            case 'DCBA':
                idx = [3, 2, 1, 0];
                break;
            case 'ABCD':
            default:
                idx = [0, 1, 2, 3];
                break;
        }
        const buf = new ArrayBuffer(4);
        const view = new DataView(buf);
        for (let i = 0; i < 4; i++) {
            view.setUint8(i, bytes[idx[i]]);
        }
        return formatNumber(view.getFloat32(0, false));
    } catch {
        return 'NA';
    }
}

// Update functions
async function updateHex(connId, slaveId, regId, idx, value) {
    value = value.toUpperCase();
    if (!/^[0-9A-F]{4}$/.test(value)) {
        alert('无效的十六进制值。必须是 4 个十六进制字符。');
        refreshTab();
        return;
    }

    const regs = registerCache[slaveId] || [];
    const reg = regs.find(r => r.id === regId);
    if (!reg) return;

    const newHex = reg.hexData.substr(0, idx * 4) + value + reg.hexData.substr((idx + 1) * 4);
    reg.hexData = newHex;

    try {
        await api.updateRegister(connId, slaveId, regId, reg);
    } catch (e) {
        alert('保存失败: ' + (e.error || e.message));
        delete registerCache[slaveId];
    }
    refreshTab();
}

async function updateBit(connId, slaveId, regId, bitIdx, checked) {
    const regs = registerCache[slaveId] || [];
    const reg = regs.find(r => r.id === regId);
    if (!reg) return;

    const bytes = hexToBytes(reg.hexData);
    const byteIdx = Math.floor(bitIdx / 8);
    const bitPos = bitIdx % 8;

    if (checked) {
        bytes[byteIdx] |= (1 << bitPos);
    } else {
        bytes[byteIdx] &= ~(1 << bitPos);
    }

    reg.hexData = bytes.map(b => b.toString(16).padStart(2, '0').toUpperCase()).join('');

    try {
        await api.updateRegister(connId, slaveId, regId, reg);
    } catch (e) {
        alert('保存失败: ' + (e.error || e.message));
        delete registerCache[slaveId];
    }
    refreshTab();
}

function refreshTab() {
    if (activeTab) {
        const tab = openTabs.find(t => t.id === activeTab);
        if (tab) loadTabContent(tab);
    }
}

// Import frames
const IMPORT_NEW_CONN_VALUE = '__new__';

document.getElementById('importBtn').onclick = () => {
    populateImportConnections();
    document.getElementById('importFrames').value = '';
    const nameEl = document.getElementById('importConnName');
    if (nameEl) nameEl.value = '';
    const portEl = document.getElementById('importConnPort');
    if (portEl) portEl.value = '';
    setImportSummary('');
    showModal('importModal');
};

document.getElementById('importConnId').onchange = () => {
    toggleImportNewConnFields();
};

document.getElementById('importForm').onsubmit = async (e) => {
    e.preventDefault();
    if (importInProgress) return;

    let connId = document.getElementById('importConnId').value;
    const rawText = document.getElementById('importFrames').value;
    const submitBtn = e.submitter;

    if (!connId) {
        setImportSummary('<div class="summary-status warn">请选择连接。</div>');
        return;
    }
    const isCreatingNew = connId === IMPORT_NEW_CONN_VALUE;
    const nameInput = document.getElementById('importConnName').value.trim();
    if (isCreatingNew && !nameInput) {
        setImportSummary('<div class="summary-status warn">请输入名称。</div>');
        return;
    }

    let pendingConn = null;
    if (connId === IMPORT_NEW_CONN_VALUE) {
        const portInput = document.getElementById('importConnPort').value;
        const port = portInput ? parseInt(portInput) : getNextPort();
        if (!port || port < 1 || port > 65535) {
            setImportSummary('<div class="summary-status warn">请输入有效的端口号 (1-65535)。</div>');
            return;
        }
        const existing = deviceTree.find(n => n.connection.port === port);
        if (existing) {
            setImportSummary('<div class="summary-status warn">该端口已存在，请从下拉选择。</div>');
            return;
        }
        pendingConn = {name: nameInput, port};
    }

    importInProgress = true;
    if (submitBtn) {
        submitBtn.disabled = true;
        submitBtn.textContent = '保存中...';
    }

    try {
        if (pendingConn) {
            const conn = await api.createConnection(pendingConn);
            connId = conn.id;
        }
        const trimmed = rawText.trim();
        if (trimmed) {
            const report = await importFrames(connId, rawText);
            setImportSummary(renderImportSummary(report));
        } else {
            setImportSummary('<div class="summary-status warn">未导入报文</div>');
        }
        const createdDefault = await ensureDefaultSlave(connId, pendingConn ? nameInput : '');
        if (createdDefault && !trimmed) {
            setImportSummary('<div class="summary-status ok">已新增设备</div>');
        }
    } catch (err) {
        setImportSummary(`<div class="summary-status warn">${formatImportError(err)}</div>`);
    }

    if (submitBtn) {
        submitBtn.disabled = false;
        submitBtn.textContent = '保存';
    }
    importInProgress = false;
};

function populateImportConnections() {
    const select = document.getElementById('importConnId');
    select.innerHTML = '';
    const newOpt = document.createElement('option');
    newOpt.value = IMPORT_NEW_CONN_VALUE;
    newOpt.textContent = '----新增----';
    select.appendChild(newOpt);
    deviceTree.forEach(node => {
        const option = document.createElement('option');
        option.value = node.connection.id;
        let displayName = node.connection.name || `端口${node.connection.port}`;
        if (node.slaves && node.slaves.length) {
            displayName = node.slaves.length === 1 ? node.slaves[0].name : `${node.slaves[0].name} 等`;
        }
        option.textContent = `(${node.connection.port}) ${displayName}`;
        select.appendChild(option);
    });
    select.value = IMPORT_NEW_CONN_VALUE;
    toggleImportNewConnFields();
}

function toggleImportNewConnFields() {
    const isNew = document.getElementById('importConnId').value === IMPORT_NEW_CONN_VALUE;
    const nameGroup = document.getElementById('importNewConnNameGroup');
    const portGroup = document.getElementById('importNewConnPortGroup');
    const nameInput = document.getElementById('importConnName');
    const portInput = document.getElementById('importConnPort');
    if (nameGroup) nameGroup.style.display = isNew ? '' : 'none';
    if (portGroup) portGroup.style.display = isNew ? '' : 'none';
    if (nameInput) nameInput.required = isNew;
    if (portInput) portInput.required = false;
}

function setImportSummary(html) {
    const el = document.getElementById('importSummary');
    el.innerHTML = html;
}

function renderImportSummary(report) {
    const success = report.processed > 0;
    const reasons = [];
    if (!success) {
        if (report.errors && report.errors.length) {
            report.errors.forEach(err => reasons.push(err));
        }
        if (report.pairs === 0) reasons.push('未识别到有效报文');
        if (report.crcErrors) reasons.push(`CRC错误 ${report.crcErrors}`);
        if (report.parseErrors) reasons.push(`解析错误 ${report.parseErrors}`);
        if (report.unmatchedSends) reasons.push(`发送未配对 ${report.unmatchedSends}`);
        if (report.unmatchedReceives) reasons.push(`接收未配对 ${report.unmatchedReceives}`);
        if (report.skippedWrites) reasons.push(`写功能码已忽略 ${report.skippedWrites}`);
        if (report.warnings) reasons.push(`警告 ${report.warnings}`);
    }
    const uniqueReasons = [...new Set(reasons)];
    const statusText = success ? '导入成功' : (uniqueReasons.length ? `导入失败：${uniqueReasons.join('；')}` : '导入失败');
    const statusClass = success ? 'ok' : 'warn';
    return `<div class="summary-status ${statusClass}">${statusText}</div>`;
}

function formatImportError(err) {
    if (!err) return '导入失败';
    if (typeof err === 'string') return `导入失败：${err}`;
    if (err.error) return `导入失败：${err.error}`;
    if (err.message) return `导入失败：${err.message}`;
    return '导入失败';
}

async function importFrames(connId, rawText) {
    const report = {
        pairs: 0,
        processed: 0,
        createdSlaves: 0,
        updatedRegisters: 0,
        createdRegisters: 0,
        skippedWrites: 0,
        crcErrors: 0,
        parseErrors: 0,
        unmatchedSends: 0,
        unmatchedReceives: 0,
        skippedBits: 0,
        warnings: 0,
        errors: []
    };

    await loadTree();

    const selectedNode = deviceTree.find(n => n.connection.id === connId);
    let preferredName = '';
    if (selectedNode) {
        if (selectedNode.slaves.length === 1) {
            preferredName = selectedNode.slaves[0].name;
        } else if (selectedNode.slaves.length === 0) {
            preferredName = selectedNode.connection.name || '';
        }
    }

    const pairs = parseFramePairs(rawText, report);
    report.pairs = pairs.length;

    const affectedSlaves = new Set();

    for (const pair of pairs) {
        const req = parseRequest(pair.send.bytes, report);
        if (!req) continue;

        if (![1, 2, 3, 4].includes(req.functionCode)) {
            report.skippedWrites++;
            continue;
        }

        const resp = parseResponse(pair.receive.bytes, req.protocol, report);
        if (!resp) continue;

        if (resp.functionCode !== req.functionCode) {
            report.warnings++;
        }

        if (resp.unitId !== req.unitId) {
            report.warnings++;
        }

        const expectedBytes = req.functionCode === 1 || req.functionCode === 2
            ? Math.ceil(req.quantity / 8)
            : req.quantity * 2;

        let dataBytes = resp.data;
        if (dataBytes.length < expectedBytes) {
            report.warnings++;
        } else if (dataBytes.length > expectedBytes) {
            report.warnings++;
            dataBytes = dataBytes.slice(0, expectedBytes);
        }

        const effectiveCount = req.functionCode === 1 || req.functionCode === 2
            ? Math.min(req.quantity, dataBytes.length * 8)
            : Math.min(req.quantity, Math.floor(dataBytes.length / 2));

        if (effectiveCount <= 0) {
            report.warnings++;
            continue;
        }

        const logicalStart = getLogicalStart(req.functionCode, req.startAddress);

        const slave = await findOrCreateSlave(connId, req.unitId, preferredName, report);
        if (!slave) {
            continue;
        }

        const result = await applyRegisterUpdate(
            connId,
            slave.id,
            req.functionCode,
            logicalStart,
            effectiveCount,
            dataBytes,
            report
        );

        report.updatedRegisters += result.updatedRegisters;
        report.createdRegisters += result.createdRegisters;
        report.skippedBits += result.skippedBits;
        report.processed++;
        affectedSlaves.add(slave.id);
    }

    affectedSlaves.forEach(slaveId => {
        delete registerCache[slaveId];
    });
    refreshTab();
    await loadTree();

    return report;
}

async function ensureDefaultSlave(connId, nameHint) {
    await loadTree();
    const node = deviceTree.find(n => n.connection.id === connId);
    if (!node || (node.slaves && node.slaves.length)) return false;
    const baseName = (nameHint || node.connection.name || '从站 1').trim() || '从站 1';
    await api.createSlave(connId, {name: baseName, slaveAddr: 1});
    await loadTree();
    return true;
}

function parseFramePairs(text, report) {
    const lines = text.split(/\r?\n/);
    const entries = [];

    lines.forEach((line, idx) => {
        const dirMatch = line.match(/【(发送|接收)】/);
        if (!dirMatch) return;
        const direction = dirMatch[1];
        const deviceName = '';

        const hexMatch = line.match(/<([^>]+)>/);
        if (!hexMatch) {
            report.parseErrors++;
            return;
        }
        const parsed = parseHexPayload(hexMatch[1]);
        if (!parsed) {
            report.parseErrors++;
            return;
        }

        entries.push({
            direction,
            deviceName,
            bytes: parsed.bytes,
            lineNo: idx + 1
        });
    });

    const pending = new Map();
    const pairs = [];

    entries.forEach(entry => {
        const key = 'default';
        if (entry.direction === '发送') {
            if (!pending.has(key)) pending.set(key, []);
            pending.get(key).push(entry);
            return;
        }

        const queue = pending.get(key) || [];
        if (!queue.length) {
            report.unmatchedReceives++;
            return;
        }
        const send = queue.shift();
        pairs.push({send, receive: entry, deviceName: key});
    });

    for (const queue of pending.values()) {
        report.unmatchedSends += queue.length;
    }

    return pairs;
}

function parseHexPayload(raw) {
    const hex = raw.replace(/[^0-9A-Fa-f]/g, '');
    if (!hex || hex.length % 2 !== 0) {
        return null;
    }
    const bytes = [];
    for (let i = 0; i < hex.length; i += 2) {
        const val = parseInt(hex.substr(i, 2), 16);
        if (Number.isNaN(val)) {
            return null;
        }
        bytes.push(val);
    }
    return {hex: hex.toUpperCase(), bytes};
}

function parseRequest(bytes, report) {
    const tcp = parseTCPRequest(bytes);
    if (tcp) return tcp;

    const rtu = parseRTURequest(bytes);
    if (rtu && rtu.error === 'crc') {
        report.crcErrors++;
        return null;
    }
    if (rtu) return rtu;

    report.parseErrors++;
    return null;
}

function parseResponse(bytes, protocol, report) {
    if (protocol === 'tcp') {
        const resp = parseTCPResponse(bytes);
        if (!resp) {
            report.parseErrors++;
            return null;
        }
        return resp;
    }

    const resp = parseRTUResponse(bytes);
    if (resp && resp.error === 'crc') {
        report.crcErrors++;
        return null;
    }
    if (!resp) {
        report.parseErrors++;
        return null;
    }
    return resp;
}

function parseTCPRequest(bytes) {
    if (bytes.length < 12) return null;
    const protocolId = (bytes[2] << 8) | bytes[3];
    if (protocolId !== 0) return null;
    const length = (bytes[4] << 8) | bytes[5];
    if (length !== bytes.length - 6) return null;
    return {
        protocol: 'tcp',
        unitId: bytes[6],
        functionCode: bytes[7],
        startAddress: (bytes[8] << 8) | bytes[9],
        quantity: (bytes[10] << 8) | bytes[11]
    };
}

function parseTCPResponse(bytes) {
    if (bytes.length < 9) return null;
    const protocolId = (bytes[2] << 8) | bytes[3];
    if (protocolId !== 0) return null;
    const length = (bytes[4] << 8) | bytes[5];
    if (length !== bytes.length - 6) return null;
    const unitId = bytes[6];
    const functionCode = bytes[7];
    if (functionCode & 0x80) return null;
    const byteCount = bytes[8];
    if (bytes.length < 9 + byteCount) return null;
    const data = bytes.slice(9, 9 + byteCount);
    return {protocol: 'tcp', unitId, functionCode, data};
}

function parseRTURequest(bytes) {
    if (bytes.length < 8) return null;
    if (!validateCRC(bytes)) {
        return {error: 'crc'};
    }
    return {
        protocol: 'rtu',
        unitId: bytes[0],
        functionCode: bytes[1],
        startAddress: (bytes[2] << 8) | bytes[3],
        quantity: (bytes[4] << 8) | bytes[5]
    };
}

function parseRTUResponse(bytes) {
    if (bytes.length < 5) return null;
    if (!validateCRC(bytes)) {
        return {error: 'crc'};
    }
    const unitId = bytes[0];
    const functionCode = bytes[1];
    if (functionCode & 0x80) return null;
    const byteCount = bytes[2];
    if (bytes.length < 3 + byteCount + 2) return null;
    const data = bytes.slice(3, 3 + byteCount);
    return {protocol: 'rtu', unitId, functionCode, data};
}

async function findOrCreateSlave(connId, slaveAddr, deviceName, report) {
    let node = deviceTree.find(n => n.connection.id === connId);
    if (!node) {
        if (report && report.errors) report.errors.push('连接不存在');
        return null;
    }

    let slave = node.slaves.find(s => s.slaveAddr === slaveAddr);
    if (slave) return slave;

    const normalizedName = (deviceName || '').trim().toLowerCase();
    if (normalizedName) {
        const sameNameInConn = node.slaves.find(s => s.name.trim().toLowerCase() === normalizedName);
        if (sameNameInConn) return sameNameInConn;

        const sameNameNode = deviceTree.find(n => n.slaves.some(s => s.name.trim().toLowerCase() === normalizedName));
        if (sameNameNode && report && report.errors) {
            report.errors.push(`设备名称已存在（端口${sameNameNode.connection.port}），请选择对应设备`);
            return null;
        }
    }

    const name = deviceName || `从站 ${slaveAddr}`;
    try {
        await api.createSlave(connId, {name, slaveAddr});
        report.createdSlaves++;
        await loadTree();
        node = deviceTree.find(n => n.connection.id === connId);
        slave = node?.slaves.find(s => s.slaveAddr === slaveAddr);
        return slave || null;
    } catch (e) {
        if (report && report.errors) {
            const msg = e?.error || e?.message || '创建设备失败';
            report.errors.push(`创建设备失败：${msg}`);
        }
        return null;
    }
}

function getRegisterEndAddr(reg) {
    const rt = getRegType(reg.startAddr);
    if (!rt) return reg.startAddr;
    if (rt.fc === 1 || rt.fc === 2) {
        return reg.startAddr + (reg.hexData.length / 2) * 8 - 1;
    }
    return reg.startAddr + (reg.hexData.length / 4) - 1;
}

function isAddressCoveredByRegs(addr, regs) {
    return regs.some(reg => addr >= reg.startAddr && addr <= getRegisterEndAddr(reg));
}

async function ensureRegisterCache(connId, slaveId) {
    if (!registerCache[slaveId]) {
        try {
            registerCache[slaveId] = await api.getRegisters(connId, slaveId);
        } catch {
            registerCache[slaveId] = [];
        }
    }
}

async function applyRegisterUpdate(connId, slaveId, fc, logicalStart, quantity, dataBytes, report) {
    await ensureRegisterCache(connId, slaveId);

    const regs = registerCache[slaveId] || [];
    const regsOfType = regs.filter(r => getRegType(r.startAddr)?.fc === fc);
    const isBitType = fc === 1 || fc === 2;
    const logicalEnd = logicalStart + quantity - 1;

    let updatedRegisters = 0;
    let createdRegisters = 0;
    let skippedBits = 0;

    if (isBitType) {
        const bitValues = [];
        for (let i = 0; i < quantity; i++) {
            const byteIdx = Math.floor(i / 8);
            const bitIdx = i % 8;
            bitValues[i] = ((dataBytes[byteIdx] >> bitIdx) & 1) === 1;
        }

        for (const reg of regsOfType) {
            const regEnd = getRegisterEndAddr(reg);
            const overlapStart = Math.max(logicalStart, reg.startAddr);
            const overlapEnd = Math.min(logicalEnd, regEnd);
            if (overlapStart > overlapEnd) continue;

            const bytes = hexToBytes(reg.hexData.toUpperCase());
            for (let addr = overlapStart; addr <= overlapEnd; addr++) {
                const bitOffset = addr - reg.startAddr;
                const byteIdx = Math.floor(bitOffset / 8);
                const bitIdx = bitOffset % 8;
                const value = bitValues[addr - logicalStart];
                if (value) {
                    bytes[byteIdx] |= (1 << bitIdx);
                } else {
                    bytes[byteIdx] &= ~(1 << bitIdx);
                }
            }
            reg.hexData = bytesToHex(bytes);
            await api.updateRegister(connId, slaveId, reg.id, reg);
            updatedRegisters++;
        }

        const createdCoverage = new Set();
        for (let addr = logicalStart; addr <= logicalEnd; addr++) {
            if (isAddressCoveredByRegs(addr, regsOfType) || createdCoverage.has(addr)) {
                continue;
            }

            const blockStart = addr;
            const blockEnd = blockStart + 7;
            let canCreate = true;
            for (let a = blockStart; a <= blockEnd; a++) {
                if (isAddressCoveredByRegs(a, regsOfType) || createdCoverage.has(a)) {
                    canCreate = false;
                    break;
                }
            }
            if (!canCreate) {
                skippedBits++;
                continue;
            }

            let byteVal = 0;
            for (let bit = 0; bit < 8; bit++) {
                const addrPos = blockStart + bit;
                if (addrPos >= logicalStart && addrPos <= logicalEnd) {
                    const bitVal = bitValues[addrPos - logicalStart];
                    if (bitVal) {
                        byteVal |= (1 << bit);
                    }
                }
            }

            const hexData = bytesToHex([byteVal]);
            try {
                await api.createRegister(connId, slaveId, {
                    startAddr: blockStart,
                    hexData,
                    names: '',
                    coefficients: '',
                    jitterAmp: 0
                });
                createdRegisters++;
                for (let a = blockStart; a <= blockEnd; a++) {
                    createdCoverage.add(a);
                }
            } catch (e) {
                report.warnings++;
            }
        }
    } else {
        const regValues = [];
        for (let i = 0; i < quantity; i++) {
            const idx = i * 2;
            regValues[i] = [dataBytes[idx], dataBytes[idx + 1]];
        }

        const covered = new Set();
        for (const reg of regsOfType) {
            const regEnd = getRegisterEndAddr(reg);
            const overlapStart = Math.max(logicalStart, reg.startAddr);
            const overlapEnd = Math.min(logicalEnd, regEnd);
            if (overlapStart > overlapEnd) continue;

            let hexData = reg.hexData.toUpperCase();
            for (let addr = overlapStart; addr <= overlapEnd; addr++) {
                const offset = (addr - reg.startAddr) * 4;
                if (offset + 4 > hexData.length) continue;
                const value = regValues[addr - logicalStart];
                if (!value) continue;
                hexData = hexData.slice(0, offset) + bytesToHex(value) + hexData.slice(offset + 4);
                covered.add(addr);
            }
            reg.hexData = hexData;
            await api.updateRegister(connId, slaveId, reg.id, reg);
            updatedRegisters++;
        }

        let addr = logicalStart;
        while (addr <= logicalEnd) {
            if (covered.has(addr) || isAddressCoveredByRegs(addr, regsOfType)) {
                addr++;
                continue;
            }

            const segStart = addr;
            while (addr <= logicalEnd && !covered.has(addr) && !isAddressCoveredByRegs(addr, regsOfType)) {
                addr++;
            }
            const segEnd = addr - 1;

            let hexData = '';
            for (let a = segStart; a <= segEnd; a++) {
                const value = regValues[a - logicalStart] || [0x00, 0x00];
                hexData += bytesToHex(value);
            }

            try {
                await api.createRegister(connId, slaveId, {
                    startAddr: segStart,
                    hexData,
                    names: '',
                    coefficients: '',
                    jitterAmp: 0
                });
                createdRegisters++;
            } catch (e) {
                report.warnings++;
            }
        }
    }

    return {updatedRegisters, createdRegisters, skippedBits};
}

// Modal functions
function showModal(id) {
    document.getElementById(id).classList.add('show');
}

function closeModal(id) {
    document.getElementById(id).classList.remove('show');
}

// Device CRUD
const addConnectionBtn = document.getElementById('addConnectionBtn');
if (addConnectionBtn) {
    addConnectionBtn.onclick = () => {
        document.getElementById('connectionModalTitle').textContent = '添加设备';
        document.getElementById('connId').value = '';
        document.getElementById('slaveId').value = '';
        document.getElementById('connName').value = '';
        document.getElementById('connPort').value = '';
        document.getElementById('slaveAddr').value = '1';
        showModal('connectionModal');
    };
}

function editDevice(connId, slaveId) {
    const node = deviceTree.find(n => n.connection.id === connId);
    const slave = node?.slaves.find(s => s.id === slaveId);
    if (!node || !slave) return;

    document.getElementById('connectionModalTitle').textContent = '编辑设备';
    document.getElementById('connId').value = connId;
    document.getElementById('slaveId').value = slaveId;
    document.getElementById('connName').value = slave.name;
    document.getElementById('connPort').value = node.connection.port;
    document.getElementById('slaveAddr').value = slave.slaveAddr;
    showModal('connectionModal');
}

async function copyDevice(connId, slaveId) {
    const node = deviceTree.find(n => n.connection.id === connId);
    const slave = node?.slaves.find(s => s.id === slaveId);
    if (!node || !slave) {
        alert('复制失败：未找到设备');
        return;
    }

    const newName = buildCopyDeviceName(slave.name);
    const port = getNextPort();
    if (!port || port < 1 || port > 65535) {
        alert('复制失败：无法分配有效端口');
        return;
    }

    try {
        const newConn = await api.createConnection({name: newName, port});
        const newSlave = await api.createSlave(newConn.id, {
            name: newName,
            slaveAddr: slave.slaveAddr
        });

        let regs = [];
        try {
            regs = await api.getRegisters(connId, slaveId);
        } catch {
            regs = [];
        }

        for (const reg of regs) {
            await api.createRegister(newConn.id, newSlave.id, {
                startAddr: reg.startAddr,
                hexData: String(reg.hexData || '').toUpperCase(),
                names: reg.names || '',
                coefficients: reg.coefficients || '',
                jitterAmp: Number.isInteger(reg.jitterAmp) ? reg.jitterAmp : parseInt(reg.jitterAmp || 0, 10) || 0
            });
        }

        await loadTree();
    } catch (e) {
        alert('复制失败：' + (e.error || e.message));
    }
}

function getNextPort() {
    if (!deviceTree.length) return 1502;
    const maxPort = Math.max(...deviceTree.map(n => n.connection.port || 0));
    return maxPort + 1;
}

function normalizeDeviceName(name) {
    return name.trim().toLowerCase();
}

function isDuplicateDeviceName(name, excludeSlaveId) {
    const normalized = normalizeDeviceName(name);
    if (!normalized) return false;
    return deviceTree.some(node => (
        node.slaves.some(slave => slave.id !== excludeSlaveId && normalizeDeviceName(slave.name) === normalized)
    ));
}

function buildCopyDeviceName(baseName) {
    const base = (baseName || '').trim() || '设备';
    let candidate = `${base}_副本`;
    if (!isDuplicateDeviceName(candidate)) return candidate;
    let index = 2;
    while (isDuplicateDeviceName(`${base}_副本${index}`)) {
        index++;
    }
    return `${base}_副本${index}`;
}

document.getElementById('connectionForm').onsubmit = async (e) => {
    e.preventDefault();

    const name = document.getElementById('connName').value.trim();
    const portInput = document.getElementById('connPort').value;
    const slaveAddrInput = document.getElementById('slaveAddr').value;

    let port = portInput ? parseInt(portInput) : getNextPort();
    if (!port || port < 1 || port > 65535) {
        alert('请输入有效的端口号 (1-65535)');
        document.getElementById('connPort').focus();
        return;
    }

    let slaveAddr = slaveAddrInput ? parseInt(slaveAddrInput) : 1;
    if (!slaveAddr || slaveAddr < 1 || slaveAddr > 247) {
        alert('请输入有效的从机地址 (1-247)');
        document.getElementById('slaveAddr').focus();
        return;
    }

    const connId = document.getElementById('connId').value;
    const slaveId = document.getElementById('slaveId').value;
    const slaveName = name || `从站${slaveAddr}`;

    if (isDuplicateDeviceName(slaveName, slaveId)) {
        alert('设备名称不能重复');
        document.getElementById('connName').focus();
        return;
    }

    try {
        if (slaveId) {
            const node = deviceTree.find(n => n.connection.id === connId);
            if (node && node.connection.port !== port) {
                alert('暂不支持修改端口，请删除后重新添加。');
                return;
            }
            await api.updateSlave(connId, slaveId, {name: slaveName, slaveAddr});
        } else {
            let node = deviceTree.find(n => n.connection.port === port);
            let targetConnId = node?.connection.id;
            if (!targetConnId) {
                const conn = await api.createConnection({name: `端口${port}`, port});
                targetConnId = conn.id;
            }
            await api.createSlave(targetConnId, {name: slaveName, slaveAddr});
        }
        closeModal('connectionModal');
        await loadTree();
    } catch (e) {
        alert('保存失败: ' + (e.error || e.message));
    }
};

async function deleteDevice(connId, slaveId) {
    if (!confirm('确定要删除此设备及其所有寄存器吗?')) return;
    try {
        await api.deleteSlave(connId, slaveId);
        delete registerCache[slaveId];
        closeTabsBySlave(slaveId);

        const node = deviceTree.find(n => n.connection.id === connId);
        if (node && node.slaves.length <= 1) {
            await api.deleteConnection(connId);
            closeTabsByConnection(connId);
        }

        await loadTree();
    } catch (e) {
        alert('删除失败: ' + (e.error || e.message));
    }
}

// Register CRUD
function showAddRegister(connId, slaveId) {
    document.getElementById('registerModalTitle').textContent = '添加寄存器';
    document.getElementById('regConnId').value = connId;
    document.getElementById('regSlaveId').value = slaveId;
    document.getElementById('regId').value = '';
    document.getElementById('regStartAddr').value = '';
    document.getElementById('regHexData').value = '';
    document.getElementById('regNames').value = '';
    document.getElementById('regCoefficients').value = '';
    const randomEnabledEl = document.getElementById('regRandomEnabled');
    const jitterAmpEl = document.getElementById('regJitterAmp');
    if (randomEnabledEl) {
        randomEnabledEl.checked = false;
    }
    if (jitterAmpEl) {
        jitterAmpEl.value = '5';
    }
    toggleRegisterJitterInput(false);
    setRegisterHexError('');
    setRegisterJitterError('');
    showModal('registerModal');
}

async function showEditRegister(connId, slaveId, regId) {
    try {
        const reg = await api.getRegister(connId, slaveId, regId);
        document.getElementById('registerModalTitle').textContent = '编辑寄存器';
        document.getElementById('regConnId').value = connId;
        document.getElementById('regSlaveId').value = slaveId;
        document.getElementById('regId').value = regId;
        document.getElementById('regStartAddr').value = reg.startAddr;
        document.getElementById('regHexData').value = reg.hexData;
        // Handle names - could be array or comma-separated string
        const names = Array.isArray(reg.names) ? reg.names.join(',') : (reg.names || '');
        document.getElementById('regNames').value = names;
        // Handle coefficients - could be array or comma-separated string
        const coeffs = Array.isArray(reg.coefficients) ? reg.coefficients.join(',') : (reg.coefficients || '');
        document.getElementById('regCoefficients').value = coeffs;
        const randomEnabledEl = document.getElementById('regRandomEnabled');
        const jitterAmpEl = document.getElementById('regJitterAmp');
        const jitterAmp = Number.isInteger(reg.jitterAmp) ? reg.jitterAmp : parseInt(reg.jitterAmp || 0, 10);
        const enabled = !!jitterAmp;
        if (randomEnabledEl) {
            randomEnabledEl.checked = enabled;
        }
        if (jitterAmpEl) {
            jitterAmpEl.value = enabled ? String(jitterAmp) : '5';
        }
        toggleRegisterJitterInput(enabled);
        setRegisterHexError('');
        setRegisterJitterError('');
        showModal('registerModal');
    } catch (e) {
        alert('加载寄存器失败: ' + (e.error || e.message));
    }
}

async function deleteRegisterAddr(connId, slaveId, regId) {
    if (!confirm('确定要删除此寄存器块吗?')) return;
    try {
        await api.deleteRegister(connId, slaveId, regId);
        delete registerCache[slaveId];
        refreshTab();
        await loadTree();
    } catch (e) {
        alert('删除失败: ' + (e.error || e.message));
    }
}

function setRegisterHexError(message) {
    const el = document.getElementById('regHexError');
    if (!el) return;
    if (message) {
        el.textContent = message;
        el.style.display = 'block';
    } else {
        el.textContent = '';
        el.style.display = 'none';
    }
}

function setRegisterJitterError(message) {
    const el = document.getElementById('regJitterError');
    if (!el) return;
    if (message) {
        el.textContent = message;
        el.style.display = 'block';
    } else {
        el.textContent = '';
        el.style.display = 'none';
    }
}

function toggleRegisterJitterInput(enabled, withDefault = false) {
    const group = document.getElementById('regJitterGroup');
    const input = document.getElementById('regJitterAmp');
    if (!group || !input) return;

    group.style.display = enabled ? '' : 'none';
    if (enabled && withDefault) {
        input.value = '5';
    }
    if (!enabled) {
        setRegisterJitterError('');
    }
}

document.getElementById('registerForm').onsubmit = async (e) => {
    e.preventDefault();
    const connId = document.getElementById('regConnId').value;
    const slaveId = document.getElementById('regSlaveId').value;
    const id = document.getElementById('regId').value;
    const startAddrVal = document.getElementById('regStartAddr').value;
    const hexInputEl = document.getElementById('regHexData');
    const randomEnabledEl = document.getElementById('regRandomEnabled');
    const jitterAmpEl = document.getElementById('regJitterAmp');
    const rawHex = hexInputEl.value.trim().toUpperCase();
    if (!rawHex) {
        if (id) {
            setRegisterHexError('清空并保存会删除该寄存器块。');
            if (!confirm('清空并保存将删除该寄存器块，是否继续？')) {
                hexInputEl.focus();
                return;
            }
            try {
                await api.deleteRegister(connId, slaveId, id);
                closeModal('registerModal');
                delete registerCache[slaveId];
                refreshTab();
                await loadTree();
            } catch (e) {
                setRegisterHexError(`删除失败：${e.error || e.message}`);
            }
            return;
        }
        setRegisterHexError('十六进制数据不能为空。');
        hexInputEl.focus();
        return;
    }
    const startAddr = parseInt(startAddrVal);
    const regType = getRegType(startAddr);
    if (!/^[0-9A-F]+$/.test(rawHex)) {
        setRegisterHexError('只能输入 0-9、A-F。');
        hexInputEl.focus();
        return;
    }
    if (regType) {
        const isBitType = regType.fc === 1 || regType.fc === 2;
        if ((isBitType && rawHex.length % 2 !== 0) || (!isBitType && rawHex.length % 4 !== 0)) {
            const hint = isBitType ? '线圈/离散输入需为 2 的倍数字符' : '寄存器需为 4 的倍数字符';
            setRegisterHexError(`长度不合法：${hint}。`);
            hexInputEl.focus();
            return;
        }
    }
    setRegisterHexError('');

    let jitterAmp = 0;
    if (randomEnabledEl && randomEnabledEl.checked) {
        const rawJitter = jitterAmpEl ? jitterAmpEl.value.trim() : '';
        if (!/^\d+$/.test(rawJitter)) {
            setRegisterJitterError('请输入 0-65535 的整数。');
            jitterAmpEl?.focus();
            return;
        }
        jitterAmp = parseInt(rawJitter, 10);
        if (jitterAmp < 0 || jitterAmp > 65535) {
            setRegisterJitterError('抖动范围必须在 0-65535 之间。');
            jitterAmpEl?.focus();
            return;
        }
    }
    setRegisterJitterError('');

    const data = {
        startAddr: startAddr,
        hexData: rawHex,
        names: document.getElementById('regNames').value,
        coefficients: document.getElementById('regCoefficients').value,
        jitterAmp: jitterAmp
    };

    try {
        if (id) {
            await api.updateRegister(connId, slaveId, id, data);
        } else {
            await api.createRegister(connId, slaveId, data);
        }
        closeModal('registerModal');
        delete registerCache[slaveId];
        refreshTab();
        await loadTree();
    } catch (e) {
        alert('保存失败: ' + (e.error || e.message));
    }
};

document.getElementById('regHexData').addEventListener('input', () => {
    setRegisterHexError('');
});

document.getElementById('regRandomEnabled').addEventListener('change', (e) => {
    toggleRegisterJitterInput(e.target.checked, e.target.checked);
});

document.getElementById('regJitterAmp').addEventListener('input', () => {
    setRegisterJitterError('');
});

// Load data
async function loadTree() {
    try {
        deviceTree = await api.getTree();
        renderTree();
    } catch (e) {
        console.error('加载设备树失败:', e);
    }
}

// Init
loadTree();

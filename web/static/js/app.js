// State
let deviceTree = [];
let registerCache = {};
let openTabs = [];
let activeTab = null;
let dirtyRegisters = new Map();

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
    {name: 'Coils', fc: 1, min: 1, max: 9999},
    {name: 'Discrete Inputs', fc: 2, min: 10001, max: 19999},
    {name: 'Input Registers', fc: 4, min: 30001, max: 39999},
    {name: 'Holding Registers', fc: 3, min: 40001, max: 49999}
];

function getRegType(startAddr) {
    return regTypes.find(t => startAddr >= t.min && startAddr <= t.max);
}

// Tree rendering
function renderTree() {
    const container = document.getElementById('deviceTree');
    container.innerHTML = '';

    deviceTree.forEach(node => {
        const connEl = createTreeNode(node);
        container.appendChild(connEl);
    });
}

function createTreeNode(node) {
    const conn = node.connection;
    const el = document.createElement('div');
    el.className = 'tree-node';
    el.innerHTML = `
        <div class="tree-node-content" data-type="connection" data-id="${conn.id}">
            <span class="tree-expand">${node.slaves.length ? '▶' : ''}</span>
            <span class="tree-icon">📡</span>
            <span class="tree-label">${conn.name} (:${conn.port})</span>
            <div class="tree-actions">
                <button class="btn-icon" onclick="event.stopPropagation(); showAddSlave('${conn.id}')" title="Add Slave">➕</button>
                <button class="btn-icon" onclick="event.stopPropagation(); editConnection('${conn.id}')" title="Edit">✏️</button>
                <button class="btn-icon btn-danger" onclick="event.stopPropagation(); deleteConnection('${conn.id}')" title="Delete">🗑️</button>
            </div>
        </div>
        <div class="tree-children collapsed"></div>
    `;

    const childrenContainer = el.querySelector('.tree-children');
    node.slaves.forEach(slave => {
        const slaveEl = createSlaveNode(conn.id, slave);
        childrenContainer.appendChild(slaveEl);
    });

    el.querySelector('.tree-node-content').onclick = () => toggleNode(el);

    return el;
}

function createSlaveNode(connId, slave) {
    const el = document.createElement('div');
    el.className = 'tree-node';
    el.innerHTML = `
        <div class="tree-node-content" data-type="slave" data-conn-id="${connId}" data-id="${slave.id}">
            <span class="tree-expand">▶</span>
            <span class="tree-icon">🔌</span>
            <span class="tree-label">${slave.name} (Addr: ${slave.slaveAddr})</span>
            <div class="tree-actions">
                <button class="btn-icon" onclick="event.stopPropagation(); showAddRegister('${connId}', '${slave.id}')" title="Add Register">➕</button>
                <button class="btn-icon" onclick="event.stopPropagation(); editSlave('${connId}', '${slave.id}')" title="Edit">✏️</button>
                <button class="btn-icon btn-danger" onclick="event.stopPropagation(); deleteSlave('${connId}', '${slave.id}')" title="Delete">🗑️</button>
            </div>
        </div>
        <div class="tree-children collapsed"></div>
    `;

    const childrenContainer = el.querySelector('.tree-children');
    regTypes.forEach(rt => {
        const rtEl = document.createElement('div');
        rtEl.className = 'tree-node';
        rtEl.innerHTML = `
            <div class="tree-node-content" data-type="regtype" data-conn-id="${connId}" data-slave-id="${slave.id}" data-fc="${rt.fc}">
                <span class="tree-expand"></span>
                <span class="tree-icon">📋</span>
                <span class="tree-label">${rt.name}</span>
            </div>
        `;
        rtEl.querySelector('.tree-node-content').onclick = () => openRegisterTab(connId, slave, rt);
        childrenContainer.appendChild(rtEl);
    });

    el.querySelector('.tree-node-content').onclick = () => toggleNode(el);

    return el;
}

function toggleNode(el) {
    const children = el.querySelector('.tree-children');
    const expand = el.querySelector('.tree-expand');
    if (children) {
        children.classList.toggle('collapsed');
        expand.textContent = children.classList.contains('collapsed') ? '▶' : '▼';
    }
}

// Tab management
function openRegisterTab(connId, slave, regType) {
    const tabId = `${slave.id}-${regType.fc}`;
    let tab = openTabs.find(t => t.id === tabId);

    if (!tab) {
        tab = {
            id: tabId,
            connId,
            slave,
            regType,
            title: `${slave.name} - ${regType.name}`
        };
        openTabs.push(tab);
    }

    activeTab = tabId;
    renderTabs();
    loadTabContent(tab);
}

function renderTabs() {
    const container = document.getElementById('tabHeaders');
    container.innerHTML = openTabs.map(tab => `
        <div class="tab ${tab.id === activeTab ? 'active' : ''}" data-id="${tab.id}" onclick="selectTab('${tab.id}')">
            <span>${tab.title}</span>
            <span class="tab-close" onclick="event.stopPropagation(); closeTab('${tab.id}')">×</span>
        </div>
    `).join('');
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
        document.getElementById('tabContent').innerHTML = '<div class="empty-state">Select a register type from the device tree</div>';
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
    });

    const isBitType = tab.regType.fc === 1 || tab.regType.fc === 2;

    content.innerHTML = `
        <div class="table-actions">
            <button class="btn btn-primary" onclick="showAddRegister('${tab.connId}', '${tab.slave.id}')">Add Register</button>
        </div>
        <table class="register-table">
            <thead>
                <tr>
                    <th>Address</th>
                    <th>Name</th>
                    <th>${isBitType ? 'Bit' : 'Hex'}</th>
                    ${isBitType ? '' : '<th>Int16</th><th>UInt16</th><th>Float32</th>'}
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                ${regs.length ? renderRegisterRows(regs, tab, isBitType) : '<tr><td colspan="7" style="text-align:center">No registers</td></tr>'}
            </tbody>
        </table>
    `;
}

function renderRegisterRows(regs, tab, isBitType) {
    let html = '';
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
                    const bitVal = (b >> i) & 1;
                    const name = names[bitIndex] || '';
                    html += `
                        <tr data-reg-id="${reg.id}" data-bit-idx="${bitIndex}">
                            <td>${addr}</td>
                            <td>${name}</td>
                            <td>
                                <input type="checkbox" ${bitVal ? 'checked' : ''}
                                    onchange="updateBit('${tab.connId}', '${tab.slave.id}', '${reg.id}', ${bitIndex}, this.checked)">
                            </td>
                            <td>
                                <button class="btn-icon btn-danger" onclick="deleteRegisterAddr('${tab.connId}', '${tab.slave.id}', '${reg.id}')">🗑️</button>
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
                const hexVal = reg.hexData.substr(i * 4, 4);
                const int16Val = hexToInt16(hexVal);
                const uint16Val = hexToUint16(hexVal);
                const coeff = coeffs[i] || 1;
                const name = names[i] || '';

                // Float32 (sliding window)
                let float32 = 'NA';
                if (i < regCount - 1) {
                    const hex32 = reg.hexData.substr(i * 4, 8);
                    float32 = hexToFloat32(hex32);
                }

                html += `
                    <tr data-reg-id="${reg.id}" data-reg-idx="${i}">
                        <td>${addr}</td>
                        <td>${name}</td>
                        <td>
                            <input type="text" value="${hexVal}" maxlength="4" pattern="[0-9A-Fa-f]{4}"
                                onchange="updateHex('${tab.connId}', '${tab.slave.id}', '${reg.id}', ${i}, this.value)">
                        </td>
                        <td>${(int16Val * coeff).toFixed(3)}</td>
                        <td>${(uint16Val * coeff).toFixed(3)}</td>
                        <td>${float32}</td>
                        <td>
                            ${i === 0 ? `<button class="btn-icon btn-danger" onclick="deleteRegisterAddr('${tab.connId}', '${tab.slave.id}', '${reg.id}')">🗑️</button>` : ''}
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

function hexToInt16(hex) {
    const val = parseInt(hex, 16);
    return val > 0x7FFF ? val - 0x10000 : val;
}

function hexToUint16(hex) {
    return parseInt(hex, 16);
}

function hexToFloat32(hex) {
    if (hex.length !== 8) return 'NA';
    try {
        const buf = new ArrayBuffer(4);
        const view = new DataView(buf);
        view.setUint32(0, parseInt(hex, 16), false);
        return view.getFloat32(0, false).toFixed(3);
    } catch {
        return 'NA';
    }
}

// Update functions
async function updateHex(connId, slaveId, regId, idx, value) {
    value = value.toUpperCase();
    if (!/^[0-9A-F]{4}$/.test(value)) {
        alert('Invalid hex value. Must be 4 hex characters.');
        refreshTab();
        return;
    }

    const regs = registerCache[slaveId] || [];
    const reg = regs.find(r => r.id === regId);
    if (!reg) return;

    const newHex = reg.hexData.substr(0, idx * 4) + value + reg.hexData.substr((idx + 1) * 4);
    reg.hexData = newHex;

    markDirty(regId);
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

    markDirty(regId);
    refreshTab();
}

function markDirty(regId) {
    dirtyRegisters.set(regId, true);
    document.getElementById('saveBtn').disabled = false;
}

function refreshTab() {
    if (activeTab) {
        const tab = openTabs.find(t => t.id === activeTab);
        if (tab) loadTabContent(tab);
    }
}

// Save changes
document.getElementById('saveBtn').onclick = async () => {
    const btn = document.getElementById('saveBtn');
    btn.disabled = true;
    btn.textContent = 'Saving...';

    try {
        for (const [regId] of dirtyRegisters) {
            // Find the register
            for (const slaveId in registerCache) {
                const reg = registerCache[slaveId].find(r => r.id === regId);
                if (reg) {
                    // Find connId
                    for (const node of deviceTree) {
                        const slave = node.slaves.find(s => s.id === slaveId);
                        if (slave) {
                            await api.updateRegister(node.connection.id, slaveId, regId, reg);
                            break;
                        }
                    }
                    break;
                }
            }
        }

        dirtyRegisters.clear();
        btn.textContent = 'Saved!';
        setTimeout(() => {
            btn.textContent = 'Save Changes';
        }, 1500);
    } catch (e) {
        alert('Save failed: ' + (e.error || e.message));
        btn.disabled = false;
        btn.textContent = 'Save Changes';
    }
};

// Modal functions
function showModal(id) {
    document.getElementById(id).classList.add('show');
}

function closeModal(id) {
    document.getElementById(id).classList.remove('show');
}

// Connection CRUD
document.getElementById('addConnectionBtn').onclick = () => {
    document.getElementById('connectionModalTitle').textContent = 'Add Connection';
    document.getElementById('connId').value = '';
    document.getElementById('connName').value = '';
    document.getElementById('connPort').value = '';
    document.getElementById('connProtocol').value = '0';
    showModal('connectionModal');
};

function editConnection(id) {
    const node = deviceTree.find(n => n.connection.id === id);
    if (!node) return;

    document.getElementById('connectionModalTitle').textContent = 'Edit Connection';
    document.getElementById('connId').value = id;
    document.getElementById('connName').value = node.connection.name;
    document.getElementById('connPort').value = node.connection.port;
    document.getElementById('connProtocol').value = node.connection.protocolType;
    showModal('connectionModal');
}

async function deleteConnection(id) {
    if (!confirm('Delete this connection and all its slaves?')) return;
    try {
        await api.deleteConnection(id);
        await loadTree();
    } catch (e) {
        alert('Delete failed: ' + (e.error || e.message));
    }
}

document.getElementById('connectionForm').onsubmit = async (e) => {
    e.preventDefault();
    const id = document.getElementById('connId').value;
    const data = {
        name: document.getElementById('connName').value,
        port: parseInt(document.getElementById('connPort').value) || 0,
        protocolType: parseInt(document.getElementById('connProtocol').value)
    };

    try {
        if (id) {
            await api.updateConnection(id, data);
        } else {
            await api.createConnection(data);
        }
        closeModal('connectionModal');
        await loadTree();
    } catch (e) {
        alert('Save failed: ' + (e.error || e.message));
    }
};

// Slave CRUD
function showAddSlave(connId) {
    document.getElementById('slaveModalTitle').textContent = 'Add Slave';
    document.getElementById('slaveConnId').value = connId;
    document.getElementById('slaveId').value = '';
    document.getElementById('slaveName').value = '';
    document.getElementById('slaveAddr').value = '1';
    showModal('slaveModal');
}

function editSlave(connId, slaveId) {
    const node = deviceTree.find(n => n.connection.id === connId);
    const slave = node?.slaves.find(s => s.id === slaveId);
    if (!slave) return;

    document.getElementById('slaveModalTitle').textContent = 'Edit Slave';
    document.getElementById('slaveConnId').value = connId;
    document.getElementById('slaveId').value = slaveId;
    document.getElementById('slaveName').value = slave.name;
    document.getElementById('slaveAddr').value = slave.slaveAddr;
    showModal('slaveModal');
}

async function deleteSlave(connId, slaveId) {
    if (!confirm('Delete this slave and all its registers?')) return;
    try {
        await api.deleteSlave(connId, slaveId);
        delete registerCache[slaveId];
        await loadTree();
    } catch (e) {
        alert('Delete failed: ' + (e.error || e.message));
    }
}

document.getElementById('slaveForm').onsubmit = async (e) => {
    e.preventDefault();
    const connId = document.getElementById('slaveConnId').value;
    const id = document.getElementById('slaveId').value;
    const data = {
        name: document.getElementById('slaveName').value,
        slaveAddr: parseInt(document.getElementById('slaveAddr').value)
    };

    try {
        if (id) {
            await api.updateSlave(connId, id, data);
        } else {
            await api.createSlave(connId, data);
        }
        closeModal('slaveModal');
        await loadTree();
    } catch (e) {
        alert('Save failed: ' + (e.error || e.message));
    }
};

// Register CRUD
function showAddRegister(connId, slaveId) {
    document.getElementById('registerModalTitle').textContent = 'Add Register';
    document.getElementById('regConnId').value = connId;
    document.getElementById('regSlaveId').value = slaveId;
    document.getElementById('regId').value = '';
    document.getElementById('regStartAddr').value = '';
    document.getElementById('regHexData').value = '';
    document.getElementById('regNames').value = '';
    document.getElementById('regCoefficients').value = '';
    showModal('registerModal');
}

async function deleteRegisterAddr(connId, slaveId, regId) {
    if (!confirm('Delete this register block?')) return;
    try {
        await api.deleteRegister(connId, slaveId, regId);
        delete registerCache[slaveId];
        refreshTab();
        await loadTree();
    } catch (e) {
        alert('Delete failed: ' + (e.error || e.message));
    }
}

document.getElementById('registerForm').onsubmit = async (e) => {
    e.preventDefault();
    const connId = document.getElementById('regConnId').value;
    const slaveId = document.getElementById('regSlaveId').value;
    const id = document.getElementById('regId').value;
    const data = {
        startAddr: parseInt(document.getElementById('regStartAddr').value),
        hexData: document.getElementById('regHexData').value.toUpperCase(),
        names: document.getElementById('regNames').value,
        coefficients: document.getElementById('regCoefficients').value
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
        alert('Save failed: ' + (e.error || e.message));
    }
};

// Load data
async function loadTree() {
    try {
        deviceTree = await api.getTree();
        renderTree();
    } catch (e) {
        console.error('Failed to load tree:', e);
    }
}

// Init
loadTree();

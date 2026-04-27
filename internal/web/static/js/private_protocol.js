(() => {
    const MODE_MODBUS = 'modbus';
    const MODE_PRIVATE = 'private';
    const MODE_STORAGE_KEY = 'app_view_mode';
    const PRIVATE_MATCH_EXACT = 0;
    const PRIVATE_MATCH_CONTAINS = 1;
    const PRIVATE_TOKEN_RE = /^@[A-Z0-9_]+/;
    const privateConnectionCollator = new Intl.Collator('zh-Hans-CN', {
        numeric: true,
        sensitivity: 'base',
        ignorePunctuation: true
    });

    const state = {
        mode: MODE_MODBUS,
        privateTree: [],
        configs: new Map(),
        activeConnId: '',
        selectedRowIndex: -1,
        dirtyConnections: new Set(),
        expandedCategories: new Set(),
        knownCategories: new Set(),
        toastTimer: null
    };

    const api = {
        async request(url, options = {}) {
            const res = await fetch(url, {
                headers: {'Content-Type': 'application/json'},
                ...options
            });
            if (!res.ok) {
                let err;
                try { err = await res.json(); } catch { err = {error: res.statusText}; }
                throw err;
            }
            if (res.status === 204) return null;
            return res.json();
        },
        tree() { return this.request('/api/connections/tree'); },
        createConnection(data) { return this.request('/api/connections', {method: 'POST', body: JSON.stringify(data)}); },
        updateConnection(id, data) { return this.request(`/api/connections/${id}`, {method: 'PUT', body: JSON.stringify(data)}); },
        deleteConnection(id) { return this.request(`/api/connections/${id}`, {method: 'DELETE'}); },
        getConfig(id) { return this.request(`/api/connections/${id}/private-protocol`); },
        saveConfig(id, data) { return this.request(`/api/connections/${id}/private-protocol`, {method: 'PUT', body: JSON.stringify(data)}); }
    };

    document.addEventListener('DOMContentLoaded', init);

    function getHostLabel() {
        return window.location.hostname || 'localhost';
    }

    function init() {
        ensureHeaderModeSwitch();
        ensurePrivateActionBar();
        ensurePrivateTreeFooter();
        ensureConnectionSettingsModal();
        bindEvents();
        const savedMode = localStorage.getItem(MODE_STORAGE_KEY) === MODE_PRIVATE ? MODE_PRIVATE : MODE_MODBUS;
        setMode(savedMode);
    }

    function bindEvents() {
        document.getElementById('deviceTree')?.addEventListener('click', onTreeClick);
        document.getElementById('tabContent')?.addEventListener('click', onContentClick);
        document.getElementById('tabContent')?.addEventListener('input', onContentInput);
        document.getElementById('tabContent')?.addEventListener('change', onContentChange);
        document.getElementById('tabContent')?.addEventListener('focusin', onContentFocusIn);
        document.getElementById('tabContent')?.addEventListener('focusout', onContentFocusOut);

        document.getElementById('privateTreeFooter')?.addEventListener('click', async (event) => {
            const btn = event.target.closest('[data-private-tree-action]');
            if (!btn) return;
            switch (btn.dataset.privateTreeAction) {
                case 'add-connection':
                    openConnectionSettings();
                    break;
                case 'settings':
                    if (state.activeConnId) openConnectionSettings(state.activeConnId);
                    break;
                case 'delete':
                    if (state.activeConnId) await deleteConnection(state.activeConnId);
                    break;
            }
        });
    }

    function ensureHeaderModeSwitch() {
        let host = document.getElementById('headerActions');
        if (!host) {
            host = document.createElement('div');
            host.id = 'headerActions';
            host.className = 'header-actions';
            document.querySelector('header')?.appendChild(host);
        }
        if (host.querySelector('.mode-switch')) return;
        host.innerHTML = `
            <div class="mode-switch" role="tablist" aria-label="工作区切换">
                <button type="button" class="mode-switch-btn" data-app-mode="modbus">Modbus</button>
                <button type="button" class="mode-switch-btn" data-app-mode="private">私有协议</button>
            </div>
        `;
        host.querySelectorAll('[data-app-mode]').forEach((btn) => {
            btn.addEventListener('click', () => setMode(btn.dataset.appMode));
        });
    }

    function ensurePrivateActionBar() {
        const host = document.getElementById('privateTreeActions');
        if (!host || host.children.length) return;
        host.innerHTML = '';
    }

    function ensurePrivateTreeFooter() {
        const panel = document.querySelector('.tree-panel');
        if (!panel || document.getElementById('privateTreeFooter')) return;
        const footer = document.createElement('div');
        footer.id = 'privateTreeFooter';
        footer.className = 'tree-footer private-tree-footer';
        footer.innerHTML = `
            <div class="tree-footer-actions private-tree-footer-actions">
                <button type="button" class="btn btn-sm" data-private-tree-action="add-connection">新增</button>
                <button type="button" class="btn btn-sm" data-private-tree-action="settings">编辑</button>
                <button type="button" class="btn btn-sm btn-danger" data-private-tree-action="delete">删除</button>
            </div>
        `;
        panel.appendChild(footer);
    }

    function ensureConnectionSettingsModal() {
        if (document.getElementById('privateConnectionModal')) return;
        const modal = document.createElement('div');
        modal.id = 'privateConnectionModal';
        modal.className = 'modal';
        modal.innerHTML = `
            <div class="modal-content">
                <h3 id="privateConnectionTitle">私有协议连接</h3>
                <form id="privateConnectionForm">
                    <input type="hidden" id="privateConnectionId">
                    <div class="form-group">
                        <label>名称</label>
                        <input type="text" id="privateConnectionName" required>
                    </div>
                    <div class="form-group" id="privateConnectionPortGroup">
                        <label id="privateConnectionPortLabel">端口</label>
                        <input type="number" id="privateConnectionPort" min="1" max="65535" placeholder="不填则自动分配">
                    </div>
                    <div class="modal-actions">
                        <button type="button" class="btn" data-private-modal-close="privateConnectionModal">取消</button>
                        <button type="submit" class="btn btn-primary">保存</button>
                    </div>
                </form>
            </div>
        `;
        document.body.appendChild(modal);
        modal.addEventListener('click', (event) => {
            if (event.target === modal || event.target.closest('[data-private-modal-close]')) {
                closeModal(modal.id);
            }
        });
        modal.querySelector('#privateConnectionForm').addEventListener('submit', submitConnectionSettings);
    }

    async function setMode(mode) {
        const nextMode = mode === MODE_PRIVATE ? MODE_PRIVATE : MODE_MODBUS;
        try {
            if (state.mode === MODE_PRIVATE && nextMode !== MODE_PRIVATE) {
                const discard = await confirmDiscardActivePrivateDraft('当前连接存在未保存修改，点击“确定”放弃修改并切换到 Modbus，点击“取消”停留当前页面。');
                if (!discard) return;
            }
            state.mode = nextMode;
            localStorage.setItem(MODE_STORAGE_KEY, state.mode);
            updateModeButtons();
            toggleModeChrome();
            if (state.mode === MODE_PRIVATE) {
                await enterPrivateMode();
            } else {
                enterModbusMode();
            }
        } catch (e) {
            showToast(`切换失败：${e.error || e.message || e}`, true);
        }
    }

    function updateModeButtons() {
        document.querySelectorAll('[data-app-mode]').forEach((btn) => {
            btn.classList.toggle('active', btn.dataset.appMode === state.mode);
        });
    }

    function toggleModeChrome() {
        const importBtn = document.getElementById('importBtn');
        const privateActions = document.getElementById('privateTreeActions');
        const privateTreeFooter = document.getElementById('privateTreeFooter');
        const modbusTreeFooter = document.getElementById('modbusTreeFooter');
        if (importBtn) {
            importBtn.style.display = state.mode === MODE_PRIVATE ? 'none' : '';
            importBtn.textContent = '编辑';
        }
        if (privateActions) privateActions.style.display = 'none';
        if (privateTreeFooter) privateTreeFooter.style.display = state.mode === MODE_PRIVATE ? 'block' : 'none';
        if (modbusTreeFooter) modbusTreeFooter.style.display = state.mode === MODE_PRIVATE ? 'none' : 'block';
    }

    function enterModbusMode() {
        if (typeof window.loadTree === 'function') window.loadTree();
        if (typeof window.refreshTab === 'function') window.refreshTab();
        const content = document.getElementById('tabContent');
        if (content && !content.querySelector('.register-table')) {
            content.innerHTML = '<div class="empty-state">请从设备树中选择寄存器类型</div>';
        }
    }

    async function enterPrivateMode() {
        try {
            await refreshPrivateData();
        } catch (e) {
            renderPrivateError(e.error || e.message || String(e));
        }
    }

    async function refreshPrivateData(preferredConnId = '') {
        const tree = await api.tree();
        state.privateTree = tree
            .filter((node) => Number(node.connection?.serviceType || 0) === 1)
            .sort(comparePrivateConnectionNodes);
        const validConnIds = new Set(state.privateTree.map((node) => node.connection.id));
        state.dirtyConnections = new Set([...state.dirtyConnections].filter((id) => validConnIds.has(id)));

        const configEntries = await Promise.all(state.privateTree.map(async (node) => {
            try {
                const cfg = await api.getConfig(node.connection.id);
                return [node.connection.id, toUiConfig(node, cfg)];
            } catch {
                return [node.connection.id, createEmptyConfig(node)];
            }
        }));

        state.configs = new Map(configEntries);
        const candidateIds = [preferredConnId, state.activeConnId, state.privateTree[0]?.connection?.id].filter(Boolean);
        state.activeConnId = candidateIds.find((id) => state.configs.has(id)) || '';

        const config = getActiveConfig();
        if (!config || !config.rules.length) {
            state.selectedRowIndex = -1;
        } else if (state.selectedRowIndex < 0 || state.selectedRowIndex >= config.rules.length) {
            state.selectedRowIndex = 0;
        }

        renderPrivateTree();
        renderPrivateTable();
        updatePrivateActionState();
    }

    function renderPrivateError(message) {
        document.getElementById('deviceTree').innerHTML = `<div class="tree-empty">${escapeHtml(message)}</div>`;
        document.getElementById('tabContent').innerHTML = `<div class="empty-state">${escapeHtml(message)}</div>`;
    }

    function renderPrivateTree() {
        if (state.mode !== MODE_PRIVATE) return;
        const container = document.getElementById('deviceTree');
        if (!container) return;

        if (!state.privateTree.length) {
            container.innerHTML = `
                <div class="tree-empty">
                    暂无私有协议连接
                </div>`;
            return;
        }

        container.innerHTML = groupPrivateConnections(state.privateTree).map((group) => {
            const expanded = state.expandedCategories.has(group.category);
            return `
                <div class="tree-node tree-node-group private-tree-group">
                    <div class="tree-node-content" data-private-category="${escapeAttr(group.category)}">
                        <span class="tree-expand${expanded ? ' is-expanded' : ''}">›</span>
                        <span class="tree-label tree-label-group">
                            <span class="tree-label-group-name">${escapeHtml(group.category)}</span>
                            <span class="tree-group-count">${group.items.length}</span>
                        </span>
                    </div>
                    <div class="tree-children ${expanded ? '' : 'collapsed'}">
                        ${group.items.map(renderPrivateConnectionNode).join('')}
                    </div>
                </div>`;
        }).join('');
    }

    function renderPrivateConnectionNode(node) {
        const conn = node.connection;
        const config = state.configs.get(conn.id) || createEmptyConfig(node);
        const statusClass = state.dirtyConnections.has(conn.id) ? 'dirty' : (config.rules.length ? '' : 'empty');
        const statusText = state.dirtyConnections.has(conn.id) ? '未保存' : (config.rules.length ? '' : '未配置');
        const statusMarkup = statusText ? `<span class="private-status-tag ${statusClass}">${statusText}</span>` : '';
        return `
            <div class="tree-node private-tree-node">
                <div class="tree-node-content private-conn-node ${conn.id === state.activeConnId ? 'selected' : ''}" data-private-conn-id="${escapeAttr(conn.id)}">
                    <span class="tree-expand"></span>
                    <span class="tree-label tree-label-endpoint">
                        <span class="tree-label-main">${escapeHtml(conn.name || '未命名连接')}${state.dirtyConnections.has(conn.id) ? ' *' : ''}</span>
                        <span class="tree-label-sub">
                            <span>${escapeHtml(getHostLabel())}:${conn.port}</span>
                            ${statusMarkup}
                        </span>
                    </span>
                </div>
            </div>`;
    }

    function getPrivateCategoryName(name) {
        const raw = String(name || '').trim();
        if (!raw) return '未分类';
        const match = raw.match(/^[^_\-\s—]+/);
        return match ? match[0] : raw;
    }

    function comparePrivateConnectionNodes(a, b) {
        const aConn = a.connection || {};
        const bConn = b.connection || {};
        const categoryCompare = privateConnectionCollator.compare(
            getPrivateCategoryName(aConn.name),
            getPrivateCategoryName(bConn.name)
        );
        if (categoryCompare !== 0) return categoryCompare;
        const nameCompare = privateConnectionCollator.compare(aConn.name || '', bConn.name || '');
        if (nameCompare !== 0) return nameCompare;
        const portCompare = (aConn.port || 0) - (bConn.port || 0);
        if (portCompare !== 0) return portCompare;
        return String(aConn.id || '').localeCompare(String(bConn.id || ''));
    }

    function groupPrivateConnections(nodes) {
        const groups = [];
        nodes.forEach((node) => {
            const category = getPrivateCategoryName(node.connection?.name);
            let group = groups.find((entry) => entry.category === category);
            if (!group) {
                group = {category, items: []};
                groups.push(group);
            }
            group.items.push(node);
        });
        groups.forEach((group) => {
            if (!state.knownCategories.has(group.category)) {
                state.knownCategories.add(group.category);
                state.expandedCategories.add(group.category);
            }
        });
        return groups;
    }

    function renderPrivateTable() {
        if (state.mode !== MODE_PRIVATE) return;
        const content = document.getElementById('tabContent');
        if (!content) return;
        const config = getActiveConfig();
        if (!config) {
            content.innerHTML = '<div class="empty-state">请先在左侧新建或选择一个私有协议连接</div>';
            return;
        }
        if (!config.rules.length) {
            content.innerHTML = `
                <div class="table-wrap">
                    <table class="private-rules-table">
                        <thead>
                            <tr>
                                <th class="private-col-name">名称</th>
                                <th class="private-col-match">匹配模式</th>
                                <th class="private-col-send">发送</th>
                                <th class="private-col-return">接收</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td colspan="4" style="text-align:center; color: var(--text-subtle);">暂无规则，点击底部的“新增行”开始配置</td>
                            </tr>
                        </tbody>
                    </table>
                </div>
                ${renderPrivateTableFooter()}`;
            updatePrivateActionState();
            return;
        }

        content.innerHTML = `
            <div class="table-wrap">
                <table class="private-rules-table">
                    <thead>
                        <tr>
                            <th class="private-col-name">名称</th>
                            <th class="private-col-match">匹配模式</th>
                            <th class="private-col-send">发送</th>
                            <th class="private-col-return">接收</th>
                        </tr>
                    </thead>
                    <tbody>${getActiveConfig().rules.map((row, index) => renderRuleRow(row, index)).join('')}</tbody>
                </table>
            </div>
            ${renderPrivateTableFooter()}`;
        applySelectedRowState();
        refreshPrivateEditors(content);
    }

    function renderPrivateTableFooter() {
        return `
            <div class="content-footer private-table-footer">
                <div class="content-footer-actions private-table-footer-actions">
                    <button type="button" class="btn btn-sm btn-primary" data-private-table-action="save">保存</button>
                    <button type="button" class="btn btn-sm" data-private-table-action="import">导入</button>
                    <button type="button" class="btn btn-sm" data-private-table-action="add-row">新增行</button>
                    <button type="button" class="btn btn-sm" data-private-table-action="delete-row">删除行</button>
                </div>
            </div>`;
    }

    function renderRuleRow(row, index) {
        const errors = row.errors || {};
        return `
            <tr data-row-index="${index}" class="${index === state.selectedRowIndex ? 'is-selected' : ''}">
                <td><input type="text" data-field="name" data-row-index="${index}" value="${escapeAttr(row.name)}" class="${buildInputClass(row, errors.name)}" title="${escapeAttr(errors.name || '')}" spellcheck="false" autocorrect="off" autocapitalize="off" autocomplete="off"></td>
                <td class="private-cell-match">
                    <div class="private-match-switch" role="group" aria-label="匹配模式">
                        <button type="button" class="private-match-btn ${row.matchMode === PRIVATE_MATCH_EXACT ? 'active' : ''}" data-match-mode="${PRIVATE_MATCH_EXACT}" data-row-index="${index}">完全</button>
                        <button type="button" class="private-match-btn ${row.matchMode === PRIVATE_MATCH_CONTAINS ? 'active' : ''}" data-match-mode="${PRIVATE_MATCH_CONTAINS}" data-row-index="${index}">包含</button>
                    </div>
                </td>
                <td class="private-cell-io private-cell-send">
                    <div class="private-io-stack">
                        <label class="private-io-editor">
                            <span class="private-io-edit-hint">ASCII</span>
                            <textarea rows="1" data-field="sendAscii" data-row-index="${index}" aria-label="发送 ASCII" class="private-io-input ${buildInputClass(row, errors.sendAscii)}" title="${escapeAttr(errors.sendAscii || row.sendAscii || '')}" spellcheck="false" autocorrect="off" autocapitalize="off" autocomplete="off">${escapeHtml(row.sendAscii)}</textarea>
                        </label>
                        <label class="private-io-editor">
                            <span class="private-io-edit-hint">Hex</span>
                            <textarea rows="1" data-field="sendHex" data-row-index="${index}" aria-label="发送 Hex" class="private-io-input ${buildInputClass(row, errors.sendHex)}" title="${escapeAttr(errors.sendHex || formatHexTemplateForDisplay(row.sendHex, {allowTokens: false}) || '')}" spellcheck="false" autocorrect="off" autocapitalize="off" autocomplete="off">${escapeHtml(formatHexTemplateForDisplay(row.sendHex, {allowTokens: false}))}</textarea>
                        </label>
                    </div>
                </td>
                <td class="private-cell-io private-cell-return">
                    <div class="private-io-stack">
                        <label class="private-io-editor">
                            <span class="private-io-edit-hint">ASCII</span>
                            <textarea rows="1" data-field="returnAscii" data-row-index="${index}" aria-label="接收 ASCII" class="private-io-input ${buildInputClass(row, errors.returnAscii)}" title="${escapeAttr(errors.returnAscii || row.returnAscii || '')}" spellcheck="false" autocorrect="off" autocapitalize="off" autocomplete="off">${escapeHtml(row.returnAscii)}</textarea>
                        </label>
                        <label class="private-io-editor">
                            <span class="private-io-edit-hint">Hex</span>
                            <textarea rows="1" data-field="returnHex" data-row-index="${index}" aria-label="接收 Hex" class="private-io-input ${buildInputClass(row, errors.returnHex)}" title="${escapeAttr(errors.returnHex || formatHexTemplateForDisplay(row.returnHex, {allowTokens: true}) || '')}" spellcheck="false" autocorrect="off" autocapitalize="off" autocomplete="off">${escapeHtml(formatHexTemplateForDisplay(row.returnHex, {allowTokens: true}))}</textarea>
                        </label>
                    </div>
                </td>
            </tr>`;
    }

    function buildInputClass(row, error) {
        const classes = [];
        if (row.dirty) classes.push('is-dirty');
        if (error) classes.push('has-error');
        return classes.join(' ');
    }

    function applySelectedRowState() {
        document.querySelectorAll('tr[data-row-index]').forEach((row) => {
            row.classList.toggle('is-selected', Number(row.dataset.rowIndex) === state.selectedRowIndex);
        });
        updatePrivateActionState();
    }

    function updatePrivateActionState() {
        const config = getActiveConfig();
        const hasConn = !!config;
        const hasRow = !!config && state.selectedRowIndex >= 0 && state.selectedRowIndex < config.rules.length;

        document.querySelectorAll('[data-private-global-action], [data-private-table-action], [data-private-tree-action]').forEach((btn) => {
            const action = btn.dataset.privateGlobalAction || btn.dataset.privateTableAction || btn.dataset.privateTreeAction;
            let disabled = false;
            if (['save', 'import', 'add-row'].includes(action) && !hasConn) disabled = true;
            if (action === 'delete-row' && !hasRow) disabled = true;
            if (['settings', 'delete'].includes(action) && !hasConn) disabled = true;
            btn.disabled = disabled;
        });
    }

    function onTreeClick(event) {
        if (state.mode !== MODE_PRIVATE) return;
        const categoryNode = event.target.closest('[data-private-category]');
        if (categoryNode) {
            const category = categoryNode.dataset.privateCategory;
            if (state.expandedCategories.has(category)) {
                state.expandedCategories.delete(category);
            } else {
                state.expandedCategories.add(category);
            }
            renderPrivateTree();
            return;
        }
        const node = event.target.closest('[data-private-conn-id]');
        if (!node) return;
        attemptSwitchConnection(node.dataset.privateConnId);
    }

    async function onContentClick(event) {
        if (state.mode !== MODE_PRIVATE) return;
        const matchBtn = event.target.closest('[data-match-mode][data-row-index]');
        if (matchBtn) {
            const rowIndex = Number(matchBtn.dataset.rowIndex);
            const row = getRuleByIndex(rowIndex);
            if (!row) return;
            row.matchMode = normalizeMatchMode(matchBtn.dataset.matchMode);
            row.dirty = true;
            state.selectedRowIndex = rowIndex;
            markDirty();
            updateRowDom(rowIndex);
            return;
        }
        const tableBtn = event.target.closest('[data-private-table-action]');
        if (tableBtn) {
            switch (tableBtn.dataset.privateTableAction) {
                case 'save':
                    saveActiveConnection();
                    break;
                case 'import':
                    if (state.activeConnId && state.dirtyConnections.has(state.activeConnId)) {
                        const discard = await confirmDiscardActivePrivateDraft('当前连接存在未保存修改，点击“确定”放弃修改并继续导入，点击“取消”返回。');
                        if (!discard) return;
                    }
                    if (typeof window.openImportModalForPrivate === 'function') {
                        window.openImportModalForPrivate();
                    }
                    break;
                case 'add-row':
                    addRow();
                    break;
                case 'delete-row':
                    deleteSelectedRow();
                    break;
            }
            return;
        }
        const row = event.target.closest('tr[data-row-index]');
        if (!row) return;
        state.selectedRowIndex = Number(row.dataset.rowIndex);
        applySelectedRowState();
    }

    function onContentInput(event) {
        if (state.mode !== MODE_PRIVATE) return;
        const input = event.target.closest('[data-field]');
        if (!input) return;
        const row = getRuleByIndex(input.dataset.rowIndex);
        if (!row) return;
        row[input.dataset.field] = input.value;
        row.dirty = true;
        markDirty();
        if (input.matches('textarea.private-io-input')) {
            syncPrivateEditorState(input, true);
        }
    }

    function onContentChange(event) {
        if (state.mode !== MODE_PRIVATE) return;
        const input = event.target.closest('[data-field]');
        if (!input) return;
        const rowIndex = Number(input.dataset.rowIndex);
        const row = getRuleByIndex(rowIndex);
        if (!row) return;
        syncRuleField(row, input.dataset.field, input.value);
        row.dirty = true;
        markDirty();
        updateRowDom(rowIndex);
    }

    function onContentFocusIn(event) {
        if (state.mode !== MODE_PRIVATE) return;
        const input = event.target.closest('textarea.private-io-input[data-field]');
        if (!input) return;
        syncPrivateEditorState(input, true);
    }

    function onContentFocusOut(event) {
        if (state.mode !== MODE_PRIVATE) return;
        const input = event.target.closest('textarea.private-io-input[data-field]');
        if (!input) return;
        requestAnimationFrame(() => {
            if (document.activeElement === input) return;
            syncPrivateEditorState(input, false);
        });
    }

    function updateRowDom(rowIndex) {
        const row = getRuleByIndex(rowIndex);
        const tr = document.querySelector(`tr[data-row-index="${rowIndex}"]`);
        if (!row || !tr) return;
        const errors = row.errors || {};
        tr.querySelectorAll('[data-field]').forEach((input) => {
            const field = input.dataset.field;
            if (field === 'sendHex') {
                input.value = formatHexTemplateForDisplay(row.sendHex, {allowTokens: false});
            } else if (field === 'returnHex') {
                input.value = formatHexTemplateForDisplay(row.returnHex, {allowTokens: true});
            } else {
                input.value = row[field] || '';
            }
            input.className = field === 'name'
                ? buildInputClass(row, errors[field])
                : `private-io-input ${buildInputClass(row, errors[field])}`.trim();
            input.title = errors[field] || input.value || '';
        });
        tr.querySelectorAll('[data-match-mode]').forEach((btn) => {
            btn.classList.toggle('active', Number(btn.dataset.matchMode) === row.matchMode);
        });
        refreshPrivateEditors(tr);
        applySelectedRowState();
    }

    function refreshPrivateEditors(scope) {
        scope?.querySelectorAll?.('textarea.private-io-input').forEach((input) => {
            syncPrivateEditorState(input, input === document.activeElement);
        });
    }

    function syncPrivateEditorState(input, expanded) {
        if (!input || !input.matches('textarea.private-io-input')) return;
        input.classList.toggle('is-expanded', !!expanded);
        if (!expanded) {
            input.style.height = '';
            input.style.overflowY = 'hidden';
            return;
        }
        input.style.height = 'auto';
        const maxHeight = 180;
        const nextHeight = Math.min(Math.max(input.scrollHeight, 30), maxHeight);
        input.style.height = `${nextHeight}px`;
        input.style.overflowY = input.scrollHeight > maxHeight ? 'auto' : 'hidden';
    }

    function syncRuleField(row, field, rawValue) {
        row.errors = row.errors || {};
        if (field === 'name') {
            row.name = rawValue.trimStart();
            row.errors.name = row.name ? '' : '名称不能为空';
            return;
        }

        if (field === 'sendAscii') {
            row.sendAscii = rawValue;
            const result = asciiToHexTemplate(rawValue, {allowTokens: false});
            if (!result.ok) {
                row.errors.sendAscii = result.error;
                return;
            }
            row.errors.sendAscii = '';
            row.errors.sendHex = '';
            row.sendHex = result.value;
            return;
        }

        if (field === 'sendHex') {
            const normalized = normalizeHexTemplate(rawValue, {allowTokens: false, allowEmpty: true});
            row.sendHex = normalized.value;
            if (!normalized.ok) {
                row.errors.sendHex = normalized.error;
                return;
            }
            row.errors.sendHex = '';
            const converted = hexToAsciiTemplate(row.sendHex, {allowTokens: false});
            if (!converted.ok) {
                row.errors.sendHex = converted.error;
                return;
            }
            row.errors.sendAscii = '';
            row.sendAscii = converted.value;
            return;
        }

        if (field === 'returnAscii') {
            row.returnAscii = rawValue;
            const result = asciiToHexTemplate(rawValue, {allowTokens: true});
            if (!result.ok) {
                row.errors.returnAscii = result.error;
                return;
            }
            row.errors.returnAscii = '';
            row.errors.returnHex = '';
            row.returnHex = result.value;
            return;
        }

        if (field === 'returnHex') {
            const normalized = normalizeHexTemplate(rawValue, {allowTokens: true, allowEmpty: true});
            row.returnHex = normalized.value;
            if (!normalized.ok) {
                row.errors.returnHex = normalized.error;
                return;
            }
            row.errors.returnHex = '';
            const converted = hexToAsciiTemplate(row.returnHex, {allowTokens: true});
            if (!converted.ok) {
                row.errors.returnHex = converted.error;
                return;
            }
            row.errors.returnAscii = '';
            row.returnAscii = converted.value;
        }
    }

    async function attemptSwitchConnection(nextConnId) {
        if (!nextConnId || nextConnId === state.activeConnId) return;
        if (state.activeConnId && state.dirtyConnections.has(state.activeConnId)) {
            const discard = await confirmDiscardActivePrivateDraft('当前连接存在未保存修改，点击“确定”放弃修改并切换，点击“取消”停留当前页面。');
            if (!discard) return;
        }
        state.activeConnId = nextConnId;
        const config = getActiveConfig();
        state.selectedRowIndex = config && config.rules.length ? 0 : -1;
        renderPrivateTree();
        renderPrivateTable();
    }

    function addRow() {
        const config = getActiveConfig();
        if (!config) return;
        config.rules.push(createEmptyRule(nextRuleName(config)));
        state.selectedRowIndex = config.rules.length - 1;
        markDirty();
        renderPrivateTable();
    }

    function deleteSelectedRow() {
        const config = getActiveConfig();
        if (!config) return;
        if (state.selectedRowIndex < 0 || state.selectedRowIndex >= config.rules.length) return;
        config.rules.splice(state.selectedRowIndex, 1);
        if (!config.rules.length) state.selectedRowIndex = -1;
        else if (state.selectedRowIndex >= config.rules.length) state.selectedRowIndex = config.rules.length - 1;
        markDirty();
        renderPrivateTable();
    }

    async function saveActiveConnection() {
        if (!state.activeConnId) return;
        try {
            const payload = buildSavePayload(state.activeConnId);
            await api.saveConfig(state.activeConnId, payload);
            state.dirtyConnections.delete(state.activeConnId);
            showToast('私有协议已保存');
            await refreshPrivateData(state.activeConnId);
        } catch (e) {
            showToast(`保存失败：${e.error || e.message || e}`, true);
        }
    }

    function buildSavePayload(connId) {
        const node = getNodeById(connId);
        const config = state.configs.get(connId);
        if (!node || !config) throw new Error('未找到私有协议连接');

        return {
            name: node.connection.name,
            rules: config.rules.map((row, index) => {
                const request = normalizeHexTemplate(row.sendHex, {allowTokens: false, allowEmpty: false});
                if (!request.ok) throw new Error(`第 ${index + 1} 行发送Hex无效：${request.error}`);
                const response = normalizeHexTemplate(row.returnHex, {allowTokens: true, allowEmpty: false});
                if (!response.ok) throw new Error(`第 ${index + 1} 行返回Hex无效：${response.error}`);
                if (!(row.name || '').trim()) throw new Error(`第 ${index + 1} 行名称不能为空`);
                return {
                    name: row.name.trim(),
                    matchMode: normalizeMatchMode(row.matchMode),
                    requestHex: request.value,
                    responseHex: response.value,
                    randomConfig: cloneRandomConfig(row.randomConfig || [])
                };
            })
        };
    }

    function openConnectionSettings(connId = '') {
        const node = connId ? getNodeById(connId) : null;
        const isEdit = !!connId;
        document.getElementById('privateConnectionTitle').textContent = connId ? '连接设置' : '新建私有协议连接';
        document.getElementById('privateConnectionId').value = connId || '';
        document.getElementById('privateConnectionName').value = node?.connection?.name || '';
        const portLabel = document.getElementById('privateConnectionPortLabel');
        const portInput = document.getElementById('privateConnectionPort');
        if (portLabel) portLabel.textContent = isEdit ? '端口' : '端口 (不填则自动分配)';
        if (portInput) {
            portInput.value = isEdit ? String(node?.connection?.port || '') : '';
            portInput.placeholder = isEdit ? '' : '不填则自动分配';
            portInput.required = false;
        }
        showModal('privateConnectionModal');
    }

    function isDuplicatePrivateConnectionName(name, excludeConnId = '') {
        const normalized = String(name || '').trim().toLowerCase();
        if (!normalized) return false;
        return state.privateTree.some((node) => (
            node.connection.id !== excludeConnId &&
            String(node.connection.name || '').trim().toLowerCase() === normalized
        ));
    }

    async function submitConnectionSettings(event) {
        event.preventDefault();
        const connId = document.getElementById('privateConnectionId').value.trim();
        const name = document.getElementById('privateConnectionName').value.trim();
        const portInput = document.getElementById('privateConnectionPort');
        const portRaw = String(portInput?.value || '').trim();
        const port = portRaw ? Number(portRaw) : 0;

        if (!name) {
            showToast('名称不能为空', true);
            return;
        }
        if (isDuplicatePrivateConnectionName(name, connId)) {
            showToast('连接名称不能重复', true);
            document.getElementById('privateConnectionName').focus();
            return;
        }
        if (portRaw && (!port || port < 1 || port > 65535)) {
            showToast('端口必须是 1-65535', true);
            return;
        }
        if (connId && !portRaw) {
            showToast('端口不能为空', true);
            return;
        }

        try {
            if (connId) {
                await api.updateConnection(connId, {name, port, serviceType: 1});
                const payload = buildSavePayload(connId);
                payload.name = name;
                await api.saveConfig(connId, payload);
                state.dirtyConnections.delete(connId);
                showToast('连接设置已更新');
                closeModal('privateConnectionModal');
                await refreshPrivateData(connId);
            } else {
                const connPayload = {name, serviceType: 1};
                if (portRaw) connPayload.port = port;
                const conn = await api.createConnection(connPayload);
                await api.saveConfig(conn.id, {name, rules: []});
                showToast('私有协议连接已创建');
                closeModal('privateConnectionModal');
                await refreshPrivateData(conn.id);
            }
        } catch (e) {
            showToast(`保存失败：${e.error || e.message || e}`, true);
        }
    }

    async function deleteConnection(connId) {
        const node = getNodeById(connId);
        if (!node) return;
        if (!confirm(`确定要删除连接“${node.connection.name || '未命名连接'}”吗？`)) return;
        try {
            await api.deleteConnection(connId);
            state.configs.delete(connId);
            state.dirtyConnections.delete(connId);
            if (state.activeConnId === connId) {
                state.activeConnId = '';
                state.selectedRowIndex = -1;
            }
            showToast('连接已删除');
            await refreshPrivateData();
        } catch (e) {
            showToast(`删除失败：${e.error || e.message || e}`, true);
        }
    }

    function toUiConfig(node, cfg) {
        return {
            connId: node.connection.id,
            name: cfg?.name || node.connection.name || '',
            rules: Array.isArray(cfg?.rules) ? cfg.rules.map((rule, index) => toUiRule(rule, index)) : []
        };
    }

    function toUiRule(rule, index) {
        const requestHex = normalizeHexTemplate(rule.requestHex || '', {allowTokens: false, allowEmpty: true}).value;
        const responseHex = normalizeHexTemplate(rule.responseHex || '', {allowTokens: true, allowEmpty: true}).value;
        return {
            id: uid(),
            name: (rule.name || `新规则${index + 1}`).trim(),
            matchMode: normalizeMatchMode(rule.matchMode),
            sendAscii: requestHex ? hexToAsciiTemplate(requestHex, {allowTokens: false}).value : '',
            sendHex: requestHex,
            returnAscii: responseHex ? hexToAsciiTemplate(responseHex, {allowTokens: true}).value : '',
            returnHex: responseHex,
            randomConfig: cloneRandomConfig(rule.randomConfig || []),
            errors: {},
            dirty: false
        };
    }

    function createEmptyConfig(node) {
        return {
            connId: node.connection.id,
            name: node.connection.name || '',
            rules: []
        };
    }

    function createEmptyRule(name = '') {
        return {
            id: uid(),
            name,
            matchMode: PRIVATE_MATCH_EXACT,
            sendAscii: '',
            sendHex: '',
            returnAscii: '',
            returnHex: '',
            randomConfig: [],
            errors: {},
            dirty: true
        };
    }

    function nextRuleName(config) {
        let index = config.rules.length + 1;
        const names = new Set(config.rules.map((rule) => rule.name));
        while (names.has(`新规则${index}`)) index++;
        return `新规则${index}`;
    }

    function normalizeMatchMode(value) {
        return Number(value) === PRIVATE_MATCH_CONTAINS ? PRIVATE_MATCH_CONTAINS : PRIVATE_MATCH_EXACT;
    }

    function getNextPrivatePort() {
        const ports = state.privateTree.map((node) => Number(node.connection.port || 0)).filter(Boolean);
        return ports.length ? Math.max(...ports) + 1 : 2502;
    }

    function getNodeById(connId) {
        return state.privateTree.find((node) => node.connection.id === connId) || null;
    }

    function getActiveConfig() {
        return state.configs.get(state.activeConnId) || null;
    }

    function getRuleByIndex(index) {
        const config = getActiveConfig();
        if (!config) return null;
        const idx = Number(index);
        return Number.isInteger(idx) && idx >= 0 && idx < config.rules.length ? config.rules[idx] : null;
    }

    async function confirmDiscardActivePrivateDraft(message) {
        if (!state.activeConnId || !state.dirtyConnections.has(state.activeConnId)) return true;
        if (!confirm(message)) return false;
        await reloadPrivateConfig(state.activeConnId);
        state.dirtyConnections.delete(state.activeConnId);
        return true;
    }

    async function reloadPrivateConfig(connId) {
        const node = getNodeById(connId);
        if (!node) {
            state.configs.delete(connId);
            if (state.activeConnId === connId) state.selectedRowIndex = -1;
            return;
        }
        try {
            const cfg = await api.getConfig(connId);
            state.configs.set(connId, toUiConfig(node, cfg));
        } catch {
            state.configs.set(connId, createEmptyConfig(node));
        }
        if (state.activeConnId !== connId) return;
        const config = state.configs.get(connId);
        if (!config || !config.rules.length) {
            state.selectedRowIndex = -1;
        } else if (state.selectedRowIndex < 0 || state.selectedRowIndex >= config.rules.length) {
            state.selectedRowIndex = 0;
        }
    }

    function markDirty() {
        if (!state.activeConnId) return;
        state.dirtyConnections.add(state.activeConnId);
        renderPrivateTree();
        updatePrivateActionState();
    }

    function formatHexTemplateForDisplay(rawValue, options = {}) {
        const allowTokens = !!options.allowTokens;
        const compact = String(rawValue ?? '').replace(/\s+/g, '').toUpperCase();
        if (!compact) return '';
        if (!allowTokens) {
            return compact.match(/.{1,2}/g)?.join(' ') || compact;
        }
        const parts = [];
        for (let i = 0; i < compact.length;) {
            if (compact[i] === '@') {
                const matched = compact.slice(i).match(PRIVATE_TOKEN_RE);
                if (matched) {
                    parts.push(matched[0]);
                    i += matched[0].length;
                    continue;
                }
            }
            const pair = compact.slice(i, i + 2);
            if (!pair) break;
            parts.push(pair);
            i += pair.length;
        }
        return parts.join(' ');
    }

    function normalizeHexTemplate(rawValue, options = {}) {
        const allowTokens = !!options.allowTokens;
        const allowEmpty = !!options.allowEmpty;
        const compact = String(rawValue ?? '').replace(/\s+/g, '').toUpperCase();
        if (!compact) return allowEmpty ? {ok: true, value: ''} : {ok: false, value: '', error: '不能为空'};
        if (!allowTokens) {
            if (!/^[0-9A-F]+$/.test(compact)) return {ok: false, value: compact, error: '只能包含 HEX 字符'};
            if (compact.length % 2 !== 0) return {ok: false, value: compact, error: 'HEX 长度必须为偶数'};
            return {ok: true, value: compact};
        }
        let out = '';
        for (let i = 0; i < compact.length;) {
            if (compact[i] === '@') {
                const matched = compact.slice(i).match(PRIVATE_TOKEN_RE);
                if (!matched) return {ok: false, value: out, error: 'TOKEN 格式无效'};
                out += matched[0];
                i += matched[0].length;
                continue;
            }
            const pair = compact.slice(i, i + 2);
            if (pair.length < 2) return {ok: false, value: out, error: 'HEX 长度必须为偶数'};
            if (!/^[0-9A-F]{2}$/.test(pair)) return {ok: false, value: out, error: '只能包含 HEX 字符或 TOKEN'};
            out += pair;
            i += 2;
        }
        return {ok: true, value: out};
    }

    function asciiToHexTemplate(rawValue, options = {}) {
        const allowTokens = !!options.allowTokens;
        const raw = String(rawValue ?? '');
        if (!raw) return {ok: true, value: ''};
        let out = '';
        for (let i = 0; i < raw.length;) {
            const control = raw.slice(i).match(/^\{(\d{1,3})\}/);
            if (control) {
                const value = Number(control[1]);
                if (!Number.isInteger(value) || value < 0 || value > 255) {
                    return {ok: false, error: '控制字节必须写成 {0}-{255}'};
                }
                out += value.toString(16).toUpperCase().padStart(2, '0');
                i += control[0].length;
                continue;
            }
            if (allowTokens && raw[i] === '@') {
                const matched = raw.slice(i).toUpperCase().match(PRIVATE_TOKEN_RE);
                if (matched) {
                    out += matched[0];
                    i += matched[0].length;
                    continue;
                }
            }
            const code = raw.codePointAt(i);
            const char = String.fromCodePoint(code);
            if (code < 32 || code > 126) {
                return {ok: false, error: '仅支持 ASCII 可打印字符和 {13}{10} 形式的控制字节'};
            }
            out += code.toString(16).toUpperCase().padStart(2, '0');
            i += char.length;
        }
        return {ok: true, value: out};
    }

    function hexToAsciiTemplate(rawValue, options = {}) {
        const normalized = normalizeHexTemplate(rawValue, {allowTokens: !!options.allowTokens, allowEmpty: true});
        if (!normalized.ok) return {ok: false, value: '', error: normalized.error};
        let out = '';
        for (let i = 0; i < normalized.value.length;) {
            if (options.allowTokens && normalized.value[i] === '@') {
                const matched = normalized.value.slice(i).match(PRIVATE_TOKEN_RE);
                if (matched) {
                    out += matched[0];
                    i += matched[0].length;
                    continue;
                }
            }
            const value = parseInt(normalized.value.slice(i, i + 2), 16);
            out += value >= 32 && value <= 126 ? String.fromCharCode(value) : `{${value}}`;
            i += 2;
        }
        return {ok: true, value: out};
    }

    function cloneRandomConfig(config) {
        return JSON.parse(JSON.stringify(Array.isArray(config) ? config : []));
    }

    function uid() {
        return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 9)}`;
    }

    function escapeHtml(value) {
        return String(value ?? '').replace(/[&<>"']/g, (ch) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[ch]));
    }

    function escapeAttr(value) {
        return escapeHtml(value).replace(/`/g, '&#96;');
    }

    function showToast(message, isError = false) {
        document.querySelector('.app-toast')?.remove();
        const toast = document.createElement('div');
        toast.className = 'app-toast';
        if (isError) toast.style.background = 'rgba(127, 29, 29, 0.94)';
        toast.textContent = message;
        document.body.appendChild(toast);
        clearTimeout(state.toastTimer);
        state.toastTimer = setTimeout(() => toast.remove(), 2600);
    }

    function showModal(id) {
        document.getElementById(id)?.classList.add('show');
    }

    function closeModal(id) {
        document.getElementById(id)?.classList.remove('show');
    }

    window.privateProtocolUI = {
        isPrivateMode: () => state.mode === MODE_PRIVATE,
        refreshData: (preferredConnId = '') => refreshPrivateData(preferredConnId),
        getActiveConnection: () => {
            const node = getNodeById(state.activeConnId);
            if (!node) return null;
            return {
                id: node.connection.id,
                name: node.connection.name || '',
                port: node.connection.port || 0
            };
        }
    };
})();


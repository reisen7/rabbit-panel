/**
 * 容器管理模块
 */

let containerActionInProgress = false;
let allContainersData = [];
let containerSortKey = 'name';
let containerSortDir = 'asc';

// 容器分页器
const containerPaginator = new Paginator({
    pageSize: 10,
    containerId: 'containers-pagination',
    onRender: renderContainersTable
});
window.paginators['containers-pagination'] = containerPaginator;

// 加载容器列表
async function loadContainers(force = false) {
    if (containerActionInProgress && !force) return;
    
    try {
        const response = await authFetch('/api/containers');
        if (!response.ok) throw new Error(await response.text() || '获取容器列表失败');
        
        const data = await response.json();
        if (!Array.isArray(data)) {
            console.error('返回的数据不是数组:', data);
            return;
        }
        
        allContainersData = data;
        containerPaginator.setData(data);
        applyContainerSort();
        filterContainers();
    } catch (error) {
        console.error('加载容器列表失败:', error);
        showToast(error.message, 'error', { title: '加载容器列表失败' });
    }
}

// 筛选容器（带防抖）
const filterContainers = debounce(function() {
    const searchText = DOM.get('container-search').value.toLowerCase();
    const statusFilter = DOM.get('container-status-filter').value;
    
    containerPaginator.filter(container => {
        const matchSearch = !searchText || 
            container.name.toLowerCase().includes(searchText) || 
            container.image.toLowerCase().includes(searchText) ||
            container.id.toLowerCase().includes(searchText);
        const matchStatus = !statusFilter || container.state === statusFilter;
        return matchSearch && matchStatus;
    });
}, 300);

// 排序容器
function sortContainers(key) {
    if (containerSortKey === key) {
        containerSortDir = containerSortDir === 'asc' ? 'desc' : 'asc';
    } else {
        containerSortKey = key;
        containerSortDir = 'asc';
    }
    applyContainerSort();
    updateSortIcons('container');
}

function applyContainerSort() {
    containerPaginator.sort(containerSortKey, containerSortDir);
}

function updateSortIcons(type) {
    const key = type === 'container' ? containerSortKey : imageSortKey;
    const dir = type === 'container' ? containerSortDir : imageSortDir;
    
    document.querySelectorAll(`#${type}s-tab .sort-icon`).forEach(icon => {
        icon.classList.remove('active');
        icon.textContent = '↕';
    });
    
    const activeIcon = document.querySelector(`#${type}s-tab [data-sort="${key}"] .sort-icon`);
    if (activeIcon) {
        activeIcon.classList.add('active');
        activeIcon.textContent = dir === 'asc' ? '↑' : '↓';
    }
}

// 容器资源统计缓存
const containerStatsCache = {};

// 渲染容器表格
function renderContainersTable(data) {
    const tbody = DOM.get('containers-tbody');
    
    if (!data || data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="8" class="px-4 py-8 text-center text-gray-500 dark:text-dark-muted">${t('container.empty')}</td></tr>`;
        return;
    }

    tbody.innerHTML = data.map(container => {
        const statusClass = container.state === 'running' ? 'status-running' : 'status-exited';
        const statusText = container.state === 'running' ? t('container.status.running') : t('container.status.stopped');
        const escapedName = container.name.replace(/'/g, "\\'");
        const isRunning = container.state === 'running';
        
        const startBtn = !isRunning ? 
            `<button onclick="containerAction('${container.id}', 'start', '${escapedName}')" class="action-btn bg-green-500 text-white rounded text-xs hover:bg-green-600">${t('container.start')}</button>` : '';
        const stopBtn = isRunning ? 
            `<button onclick="containerAction('${container.id}', 'stop', '${escapedName}')" class="action-btn bg-yellow-500 text-white rounded text-xs hover:bg-yellow-600">${t('container.stop')}</button>` : '';
        const restartBtn = isRunning ? 
            `<button onclick="containerAction('${container.id}', 'restart', '${escapedName}')" class="action-btn bg-blue-500 text-white rounded text-xs hover:bg-blue-600">${t('container.restart')}</button>` : '';
        
        // 终端和文件管理只在运行中的容器显示
        const terminalBtn = isRunning ?
            `<button onclick="openTerminalModal('${container.id}', '${escapedName}')" class="action-btn bg-gray-700 text-white rounded text-xs hover:bg-gray-800">${t('container.terminal')}</button>` : '';
        const filesBtn = isRunning ?
            `<button onclick="openFilesModal('${container.id}', '${escapedName}')" class="action-btn bg-indigo-500 text-white rounded text-xs hover:bg-indigo-600">${t('container.files')}</button>` : '';
        
        // 资源列：优先使用缓存数据，避免闪烁
        let resourcesCell = '-';
        if (isRunning) {
            const cached = containerStatsCache[container.id];
            if (cached) {
                resourcesCell = `<span id="stats-${container.id}">${cached}</span>`;
            } else {
                resourcesCell = `<span id="stats-${container.id}" class="text-xs text-gray-400">-</span>`;
            }
        }
        
        return `
            <tr class="hover:bg-gray-50 dark:hover:bg-dark-border transition-colors">
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text" title="${container.id}">${container.id}</td>
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text" title="${container.name}">${container.name}</td>
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text wrap-cell" title="${container.image}">${container.image}</td>
                <td class="px-4 py-3 text-sm ${statusClass}">${statusText}</td>
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text wrap-cell" title="${container.ports}">${container.ports || '-'}</td>
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text">${resourcesCell}</td>
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text whitespace-nowrap">${container.created}</td>
                <td class="px-4 py-3 text-sm">
                    <div class="flex gap-1 flex-wrap">
                        ${startBtn}${stopBtn}${restartBtn}
                        <button onclick="viewLogs('${container.id}', '${escapedName}')" class="action-btn bg-purple-500 text-white rounded text-xs hover:bg-purple-600">${t('container.logs')}</button>
                        ${terminalBtn}${filesBtn}
                        <button onclick="openContainerConfigModal('${container.id}')" class="action-btn bg-teal-500 text-white rounded text-xs hover:bg-teal-600">${t('container.config')}</button>
                        <button onclick="containerAction('${container.id}', 'remove', '${escapedName}')" class="action-btn bg-red-500 text-white rounded text-xs hover:bg-red-600">${t('container.remove')}</button>
                    </div>
                </td>
            </tr>
        `;
    }).join('');
    
    // 异步加载运行中容器的资源统计
    data.filter(c => c.state === 'running').forEach(container => {
        loadContainerStats(container.id);
    });
}

// 容器操作
async function containerAction(id, action, containerName) {
    const actionMap = { 'start': '启动', 'stop': '停止', 'restart': '重启', 'remove': '删除' };
    
    if (action === 'remove') {
        const confirmed = await showConfirm({
            title: '删除容器',
            message: `确定要删除容器 <strong>${containerName || id}</strong> 吗？<br><span style="color:#ef4444;font-size:12px;">此操作不可恢复！</span>`,
            type: 'danger',
            confirmText: '确认删除'
        });
        if (!confirmed) return;
    }
    
    if (action === 'stop') {
        const confirmed = await showConfirm({
            title: '停止容器',
            message: `确定要停止容器 <strong>${containerName || id}</strong> 吗？`,
            type: 'warning',
            confirmText: '确认停止'
        });
        if (!confirmed) return;
    }

    containerActionInProgress = true;
    const buttons = document.querySelectorAll(`button[onclick*="'${id}', '${action}',"]`);
    buttons.forEach(btn => {
        btn.disabled = true;
        btn.innerHTML = `<svg class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>`;
        btn.classList.add('opacity-75', 'cursor-not-allowed');
    });

    try {
        const response = await authFetch('/api/containers/action', {
            method: 'POST',
            body: JSON.stringify({ id, action })
        });

        if (!response.ok) throw new Error(await response.text());
        showToast(`容器 ${containerName || id} ${actionMap[action]}成功`, 'success', { title: actionMap[action] + '成功' });
    } catch (error) {
        showToast(error.message, 'error', { title: actionMap[action] + '失败' });
    } finally {
        containerActionInProgress = false;
        // 强制刷新，并稍微延迟确保后端状态已更新
        setTimeout(() => loadContainers(true), 300);
    }
}

// 刷新容器
async function refreshContainers() {
    const icon = DOM.get('refresh-containers-icon');
    icon.classList.add('refresh-spinning');
    await loadContainers(true);
    setTimeout(() => icon.classList.remove('refresh-spinning'), 300);
}

// ===== 创建容器功能 =====

function openCreateContainerModal() {
    // 重置表单
    DOM.get('create-container-form').reset();
    // 重置端口、环境变量、卷的额外行
    resetDynamicFields();
    // 更新预览
    updateCommandPreview();
    // 显示模态框
    DOM.get('create-container-modal').classList.add('active');
}

function closeCreateContainerModal() {
    DOM.get('create-container-modal').classList.remove('active');
    // 隐藏日志区域
    const logContainer = DOM.get('cc-log-container');
    if (logContainer) logContainer.classList.add('hidden');
}

// 解析 docker run 命令并填充表单
function parseDockerCommand() {
    const cmdInput = DOM.get('cc-docker-cmd');
    let cmd = cmdInput.value.trim();
    
    if (!cmd) {
        showToast('请输入 docker run 命令', 'warning');
        return;
    }
    
    // 处理多行命令（合并反斜杠续行）
    cmd = cmd.replace(/\\\s*\n\s*/g, ' ').replace(/\s+/g, ' ').trim();
    
    // 检查是否是 docker run 命令
    if (!cmd.match(/^docker\s+run\b/i)) {
        showToast('请输入有效的 docker run 命令', 'warning');
        return;
    }
    
    // 移除 docker run 前缀
    cmd = cmd.replace(/^docker\s+run\s*/i, '');
    
    // 解析结果
    const result = {
        image: '',
        name: '',
        restart: '',
        network: '',
        ports: [],
        envs: [],
        volumes: []
    };
    
    // 使用状态机解析参数（处理引号内的空格）
    const tokens = [];
    let current = '';
    let inQuote = false;
    let quoteChar = '';
    
    for (let i = 0; i < cmd.length; i++) {
        const char = cmd[i];
        
        if ((char === '"' || char === "'") && (i === 0 || cmd[i-1] !== '\\')) {
            if (!inQuote) {
                inQuote = true;
                quoteChar = char;
            } else if (char === quoteChar) {
                inQuote = false;
                quoteChar = '';
            } else {
                current += char;
            }
        } else if (char === ' ' && !inQuote) {
            if (current) {
                tokens.push(current);
                current = '';
            }
        } else {
            current += char;
        }
    }
    if (current) tokens.push(current);
    
    // 解析 tokens
    let i = 0;
    while (i < tokens.length) {
        const token = tokens[i];
        
        // --name
        if (token === '--name' || token === '-n') {
            if (i + 1 < tokens.length) {
                result.name = tokens[++i];
            }
        }
        // --name=xxx
        else if (token.startsWith('--name=')) {
            result.name = token.substring(7);
        }
        // -p / --publish
        else if (token === '-p' || token === '--publish') {
            if (i + 1 < tokens.length) {
                const port = tokens[++i];
                const match = port.match(/^(?:(\d+\.\d+\.\d+\.\d+):)?(\d+):(\d+)(?:\/\w+)?$/);
                if (match) {
                    result.ports.push({ host: match[2], container: match[3] });
                }
            }
        }
        // -p8080:80 格式
        else if (token.match(/^-p\d+:\d+/)) {
            const port = token.substring(2);
            const match = port.match(/^(\d+):(\d+)/);
            if (match) {
                result.ports.push({ host: match[1], container: match[2] });
            }
        }
        // -e / --env
        else if (token === '-e' || token === '--env') {
            if (i + 1 < tokens.length) {
                const env = tokens[++i];
                const eqIdx = env.indexOf('=');
                if (eqIdx > 0) {
                    result.envs.push({ key: env.substring(0, eqIdx), value: env.substring(eqIdx + 1) });
                } else {
                    result.envs.push({ key: env, value: '' });
                }
            }
        }
        // -e KEY=VALUE 格式
        else if (token.startsWith('-e') && token.length > 2) {
            const env = token.substring(2);
            const eqIdx = env.indexOf('=');
            if (eqIdx > 0) {
                result.envs.push({ key: env.substring(0, eqIdx), value: env.substring(eqIdx + 1) });
            }
        }
        // --env=KEY=VALUE
        else if (token.startsWith('--env=')) {
            const env = token.substring(6);
            const eqIdx = env.indexOf('=');
            if (eqIdx > 0) {
                result.envs.push({ key: env.substring(0, eqIdx), value: env.substring(eqIdx + 1) });
            }
        }
        // -v / --volume
        else if (token === '-v' || token === '--volume') {
            if (i + 1 < tokens.length) {
                const vol = tokens[++i];
                const parts = vol.split(':');
                if (parts.length >= 2) {
                    result.volumes.push({ host: parts[0], container: parts[1] });
                }
            }
        }
        // --volume=xxx:yyy
        else if (token.startsWith('--volume=')) {
            const vol = token.substring(9);
            const parts = vol.split(':');
            if (parts.length >= 2) {
                result.volumes.push({ host: parts[0], container: parts[1] });
            }
        }
        // --restart
        else if (token === '--restart') {
            if (i + 1 < tokens.length) {
                result.restart = tokens[++i];
            }
        }
        else if (token.startsWith('--restart=')) {
            result.restart = token.substring(10);
        }
        // --network / --net
        else if (token === '--network' || token === '--net') {
            if (i + 1 < tokens.length) {
                result.network = tokens[++i];
            }
        }
        else if (token.startsWith('--network=')) {
            result.network = token.substring(10);
        }
        else if (token.startsWith('--net=')) {
            result.network = token.substring(6);
        }
        // -d (detach) - 忽略
        else if (token === '-d' || token === '--detach') {
            // 忽略
        }
        // -it / -i / -t - 忽略
        else if (token === '-it' || token === '-i' || token === '-t' || token === '--interactive' || token === '--tty') {
            // 忽略
        }
        // --rm - 忽略
        else if (token === '--rm') {
            // 忽略
        }
        // 其他未知参数跳过，最后一个非参数 token 作为镜像名
        else if (!token.startsWith('-')) {
            result.image = token;
        }
        
        i++;
    }
    
    if (!result.image) {
        showToast('未能解析出镜像名称', 'warning');
        return;
    }
    
    // 填充表单
    DOM.get('cc-image').value = result.image;
    DOM.get('cc-name').value = result.name;
    
    // 重启策略
    const restartSelect = DOM.get('cc-restart');
    if (result.restart) {
        const restartMap = { 'always': 'always', 'unless-stopped': 'unless-stopped', 'on-failure': 'on-failure', 'no': '' };
        restartSelect.value = restartMap[result.restart] || '';
    } else {
        restartSelect.value = '';
    }
    
    // 网络模式
    const networkSelect = DOM.get('cc-network');
    if (result.network === 'host' || result.network === 'none') {
        networkSelect.value = result.network;
    } else {
        networkSelect.value = '';
    }
    
    // 端口映射
    resetDynamicFields();
    if (result.ports.length > 0) {
        const portsContainer = DOM.get('cc-ports');
        portsContainer.innerHTML = '';
        result.ports.forEach((p, idx) => {
            const div = document.createElement('div');
            div.className = 'flex gap-2 items-center';
            div.innerHTML = `
                <input type="text" placeholder="主机端口" class="cc-port-host w-24 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" value="${p.host}" oninput="updateCommandPreview()">
                <span class="text-gray-500">:</span>
                <input type="text" placeholder="容器端口" class="cc-port-container w-24 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" value="${p.container}" oninput="updateCommandPreview()">
                ${idx === 0 ? `<button type="button" onclick="addPortMapping()" class="text-green-500 hover:text-green-700 p-1">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg>
                </button>` : `<button type="button" onclick="this.parentElement.remove(); updateCommandPreview();" class="text-red-500 hover:text-red-700 p-1">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                </button>`}
            `;
            portsContainer.appendChild(div);
        });
    }
    
    // 环境变量
    if (result.envs.length > 0) {
        const envsContainer = DOM.get('cc-envs');
        envsContainer.innerHTML = '';
        result.envs.forEach((e, idx) => {
            const div = document.createElement('div');
            div.className = 'flex gap-2 items-center';
            div.innerHTML = `
                <input type="text" placeholder="变量名" class="cc-env-key flex-1 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" value="${e.key}" oninput="updateCommandPreview()">
                <span class="text-gray-500">=</span>
                <input type="text" placeholder="值" class="cc-env-value flex-1 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" value="${e.value}" oninput="updateCommandPreview()">
                ${idx === 0 ? `<button type="button" onclick="addEnvVar()" class="text-green-500 hover:text-green-700 p-1">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg>
                </button>` : `<button type="button" onclick="this.parentElement.remove(); updateCommandPreview();" class="text-red-500 hover:text-red-700 p-1">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                </button>`}
            `;
            envsContainer.appendChild(div);
        });
    }
    
    // 数据卷
    if (result.volumes.length > 0) {
        const volsContainer = DOM.get('cc-volumes');
        volsContainer.innerHTML = '';
        result.volumes.forEach((v, idx) => {
            const div = document.createElement('div');
            div.className = 'flex gap-2 items-center';
            div.innerHTML = `
                <input type="text" placeholder="主机路径" class="cc-vol-host flex-1 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" value="${v.host}" oninput="updateCommandPreview()">
                <span class="text-gray-500">:</span>
                <input type="text" placeholder="容器路径" class="cc-vol-container flex-1 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" value="${v.container}" oninput="updateCommandPreview()">
                ${idx === 0 ? `<button type="button" onclick="addVolumeMapping()" class="text-green-500 hover:text-green-700 p-1">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg>
                </button>` : `<button type="button" onclick="this.parentElement.remove(); updateCommandPreview();" class="text-red-500 hover:text-red-700 p-1">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                </button>`}
            `;
            volsContainer.appendChild(div);
        });
    }
    
    // 更新命令预览
    updateCommandPreview();
    
    // 清空输入框
    cmdInput.value = '';
    
    showToast('命令解析成功', 'success');
}

function resetDynamicFields() {
    // 重置端口映射
    DOM.get('cc-ports').innerHTML = `
        <div class="flex gap-2 items-center">
            <input type="text" placeholder="主机端口" class="cc-port-host w-24 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" oninput="updateCommandPreview()">
            <span class="text-gray-500">:</span>
            <input type="text" placeholder="容器端口" class="cc-port-container w-24 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" oninput="updateCommandPreview()">
            <button type="button" onclick="addPortMapping()" class="text-green-500 hover:text-green-700 p-1">
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg>
            </button>
        </div>
    `;
    // 重置环境变量
    DOM.get('cc-envs').innerHTML = `
        <div class="flex gap-2 items-center">
            <input type="text" placeholder="变量名" class="cc-env-key flex-1 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" oninput="updateCommandPreview()">
            <span class="text-gray-500">=</span>
            <input type="text" placeholder="值" class="cc-env-value flex-1 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" oninput="updateCommandPreview()">
            <button type="button" onclick="addEnvVar()" class="text-green-500 hover:text-green-700 p-1">
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg>
            </button>
        </div>
    `;
    // 重置数据卷
    DOM.get('cc-volumes').innerHTML = `
        <div class="flex gap-2 items-center">
            <input type="text" placeholder="主机路径" class="cc-vol-host flex-1 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" oninput="updateCommandPreview()">
            <span class="text-gray-500">:</span>
            <input type="text" placeholder="容器路径" class="cc-vol-container flex-1 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" oninput="updateCommandPreview()">
            <button type="button" onclick="addVolumeMapping()" class="text-green-500 hover:text-green-700 p-1">
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg>
            </button>
        </div>
    `;
}

function addPortMapping() {
    const container = DOM.get('cc-ports');
    const div = document.createElement('div');
    div.className = 'flex gap-2 items-center';
    div.innerHTML = `
        <input type="text" placeholder="主机端口" class="cc-port-host w-24 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" oninput="updateCommandPreview()">
        <span class="text-gray-500">:</span>
        <input type="text" placeholder="容器端口" class="cc-port-container w-24 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" oninput="updateCommandPreview()">
        <button type="button" onclick="this.parentElement.remove(); updateCommandPreview();" class="text-red-500 hover:text-red-700 p-1">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>
        </button>
    `;
    container.appendChild(div);
}

function addEnvVar() {
    const container = DOM.get('cc-envs');
    const div = document.createElement('div');
    div.className = 'flex gap-2 items-center';
    div.innerHTML = `
        <input type="text" placeholder="变量名" class="cc-env-key flex-1 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" oninput="updateCommandPreview()">
        <span class="text-gray-500">=</span>
        <input type="text" placeholder="值" class="cc-env-value flex-1 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" oninput="updateCommandPreview()">
        <button type="button" onclick="this.parentElement.remove(); updateCommandPreview();" class="text-red-500 hover:text-red-700 p-1">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>
        </button>
    `;
    container.appendChild(div);
}

function addVolumeMapping() {
    const container = DOM.get('cc-volumes');
    const div = document.createElement('div');
    div.className = 'flex gap-2 items-center';
    div.innerHTML = `
        <input type="text" placeholder="主机路径" class="cc-vol-host flex-1 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" oninput="updateCommandPreview()">
        <span class="text-gray-500">:</span>
        <input type="text" placeholder="容器路径" class="cc-vol-container flex-1 px-2 py-1.5 border border-gray-300 dark:border-dark-border rounded text-sm" oninput="updateCommandPreview()">
        <button type="button" onclick="this.parentElement.remove(); updateCommandPreview();" class="text-red-500 hover:text-red-700 p-1">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>
        </button>
    `;
    container.appendChild(div);
}

// 更新命令预览
function updateCommandPreview() {
    const image = DOM.get('cc-image')?.value || '';
    const name = DOM.get('cc-name')?.value || '';
    const restart = DOM.get('cc-restart')?.value || '';
    const network = DOM.get('cc-network')?.value || '';
    
    let cmd = 'docker run -d';
    
    // 容器名
    if (name) cmd += ` --name ${name}`;
    
    // 重启策略
    if (restart) cmd += ` --restart ${restart}`;
    
    // 网络模式
    if (network) cmd += ` --network ${network}`;
    
    // 端口映射
    document.querySelectorAll('#cc-ports > div').forEach(row => {
        const host = row.querySelector('.cc-port-host')?.value;
        const container = row.querySelector('.cc-port-container')?.value;
        if (host && container) cmd += ` -p ${host}:${container}`;
    });
    
    // 环境变量
    document.querySelectorAll('#cc-envs > div').forEach(row => {
        const key = row.querySelector('.cc-env-key')?.value;
        const value = row.querySelector('.cc-env-value')?.value;
        if (key) cmd += ` -e ${key}=${value || ''}`;
    });
    
    // 数据卷
    document.querySelectorAll('#cc-volumes > div').forEach(row => {
        const host = row.querySelector('.cc-vol-host')?.value;
        const container = row.querySelector('.cc-vol-container')?.value;
        if (host && container) cmd += ` -v ${host}:${container}`;
    });
    
    // 镜像
    cmd += ` ${image || '<镜像名>'}`;
    
    DOM.get('cc-preview').textContent = cmd;
}

// 收集表单数据
function collectContainerConfig() {
    const config = {
        image: DOM.get('cc-image').value.trim(),
        name: DOM.get('cc-name').value.trim(),
        restart: DOM.get('cc-restart').value,
        network: DOM.get('cc-network').value,
        ports: [],
        envs: [],
        volumes: []
    };
    
    // 端口
    document.querySelectorAll('#cc-ports > div').forEach(row => {
        const host = row.querySelector('.cc-port-host')?.value?.trim();
        const container = row.querySelector('.cc-port-container')?.value?.trim();
        if (host && container) config.ports.push({ host, container });
    });
    
    // 环境变量
    document.querySelectorAll('#cc-envs > div').forEach(row => {
        const key = row.querySelector('.cc-env-key')?.value?.trim();
        const value = row.querySelector('.cc-env-value')?.value || '';
        if (key) config.envs.push({ key, value });
    });
    
    // 数据卷
    document.querySelectorAll('#cc-volumes > div').forEach(row => {
        const host = row.querySelector('.cc-vol-host')?.value?.trim();
        const container = row.querySelector('.cc-vol-container')?.value?.trim();
        if (host && container) config.volumes.push({ host, container });
    });
    
    return config;
}

// 提交创建容器（使用流式 API）
async function submitCreateContainer(e) {
    e.preventDefault();
    
    const config = collectContainerConfig();
    
    if (!config.image) {
        showToast('请输入镜像名称', 'warning');
        return;
    }
    
    const submitBtn = DOM.get('cc-submit-btn');
    const logContainer = DOM.get('cc-log-container');
    const logEl = DOM.get('cc-log');
    
    // 显示日志区域
    logContainer.classList.remove('hidden');
    logEl.textContent = '';
    submitBtn.disabled = true;
    submitBtn.textContent = '创建中...';
    
    // 添加日志的辅助函数
    const appendLog = (msg, type = 'info') => {
        const color = type === 'error' ? 'text-red-400' : type === 'success' ? 'text-green-400' : 'text-gray-300';
        const line = document.createElement('div');
        line.className = color;
        line.textContent = msg;
        logEl.appendChild(line);
        logEl.scrollTop = logEl.scrollHeight;
    };
    
    try {
        // 使用 credentials: 'include' 发送 Cookie，和 authFetch 保持一致
        const response = await fetch('/api/containers/run/stream', {
            method: 'POST',
            credentials: 'include',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(config)
        });
        
        if (!response.ok) {
            let errMsg = await response.text();
            // 尝试解析 JSON 错误
            try {
                const errJson = JSON.parse(errMsg);
                errMsg = errJson.error || errJson.message || errMsg;
            } catch (e) {}
            appendLog(errMsg, 'error');
            showToast(errMsg, 'error', { title: '创建失败' });
            return;
        }
        
        // 读取 SSE 流
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            
            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split('\n');
            buffer = lines.pop() || '';
            
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    try {
                        const data = JSON.parse(line.slice(6));
                        if (data.type === 'log') {
                            appendLog(data.message);
                        } else if (data.type === 'error') {
                            appendLog(data.message, 'error');
                            showToast(data.message, 'error', { title: '创建失败' });
                        } else if (data.type === 'success') {
                            appendLog(`容器创建成功！ID: ${data.id}`, 'success');
                            showToast(`容器创建成功: ${data.id}`, 'success');
                            // 延迟关闭，让用户看到成功消息
                            setTimeout(() => {
                                closeCreateContainerModal();
                                loadContainers(true);
                            }, 1500);
                        }
                    } catch (e) {
                        // 忽略解析错误
                    }
                }
            }
        }
    } catch (err) {
        appendLog(err.message, 'error');
        showToast(err.message, 'error');
    } finally {
        submitBtn.disabled = false;
        submitBtn.textContent = '创建并运行';
    }
}

// 初始化创建容器表单
document.addEventListener('DOMContentLoaded', () => {
    const form = DOM.get('create-container-form');
    if (form) {
        form.addEventListener('submit', submitCreateContainer);
        
        // 监听输入更新预览
        ['cc-image', 'cc-name', 'cc-restart', 'cc-network'].forEach(id => {
            const el = DOM.get(id);
            if (el) el.addEventListener('input', updateCommandPreview);
            if (el) el.addEventListener('change', updateCommandPreview);
        });
    }
    
    // 点击模态框外部关闭
    const modal = DOM.get('create-container-modal');
    if (modal) {
        modal.addEventListener('click', (e) => {
            if (e.target === modal) closeCreateContainerModal();
        });
    }
});


// ===== 容器资源统计 =====

// 格式化字节大小
function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

// 加载单个容器的资源统计
async function loadContainerStats(containerId) {
    const el = document.getElementById(`stats-${containerId}`);
    if (!el) return;
    
    try {
        const response = await authFetch(`/api/containers/stats?id=${containerId}`);
        if (!response.ok) {
            el.textContent = '-';
            delete containerStatsCache[containerId];
            return;
        }
        
        const stats = await response.json();
        const cpuPercent = stats.cpu_percent.toFixed(1);
        const cpuCores = stats.cpu_cores || '-';
        const memUsage = formatBytes(stats.memory_usage);
        const memLimit = formatBytes(stats.memory_limit);
        
        const html = `
            <div class="text-xs leading-tight whitespace-nowrap">
                <div title="CPU 使用率 / 核心数">CPU: ${cpuPercent}% / ${cpuCores}</div>
                <div title="内存使用 / 限制">Mem: ${memUsage} / ${memLimit}</div>
            </div>
        `;
        el.innerHTML = html;
        // 缓存结果，避免刷新时闪烁
        containerStatsCache[containerId] = html;
    } catch (error) {
        el.textContent = '-';
        delete containerStatsCache[containerId];
    }
}

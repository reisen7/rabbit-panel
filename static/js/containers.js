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
        
        return `
            <tr class="hover:bg-gray-50 dark:hover:bg-dark-border transition-colors">
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text" title="${container.id}">${container.id}</td>
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text" title="${container.name}">${container.name}</td>
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text wrap-cell" title="${container.image}">${container.image}</td>
                <td class="px-4 py-3 text-sm ${statusClass}">${statusText}</td>
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text wrap-cell" title="${container.ports}">${container.ports || '-'}</td>
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text" title="容器可写层文件系统大小">${container.memory || '-'}</td>
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

// 提交创建容器
async function submitCreateContainer(e) {
    e.preventDefault();
    
    const config = collectContainerConfig();
    
    if (!config.image) {
        showToast('请输入镜像名称', 'warning');
        return;
    }
    
    const submitBtn = e.target.querySelector('button[type="submit"]');
    submitBtn.disabled = true;
    submitBtn.textContent = '创建中...';
    
    try {
        const res = await authFetch('/api/containers/run', {
            method: 'POST',
            body: JSON.stringify(config)
        });
        
        if (res.ok) {
            const data = await res.json();
            showToast(`容器创建成功: ${data.id?.substring(0, 12) || ''}`, 'success');
            closeCreateContainerModal();
            loadContainers(true);
        } else {
            const text = await res.text();
            showToast(text, 'error', { title: '创建失败' });
        }
    } catch (err) {
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

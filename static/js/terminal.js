/**
 * 容器终端和文件管理模块
 */

// 当前操作的容器
let currentTerminalContainer = null;
let currentFileContainer = null;
let currentFilePath = '/';

// xterm.js 相关
let term = null;
let termSocket = null;
let fitAddon = null;

// ========== 终端功能 (xterm.js + WebSocket) ==========

// 打开终端模态框
function openTerminalModal(containerId, containerName) {
    currentTerminalContainer = containerId;
    
    const modal = document.getElementById('terminal-modal');
    document.getElementById('terminal-container-name').textContent = containerName;
    modal.classList.add('active');
    
    // 延迟初始化，等待 modal 显示
    setTimeout(() => initXterm(containerId), 100);
}

// 初始化 xterm.js
function initXterm(containerId) {
    const container = document.getElementById('xterm-container');
    container.innerHTML = ''; // 清空
    
    // 创建终端实例
    term = new Terminal({
        cursorBlink: true,
        fontSize: 14,
        fontFamily: 'Menlo, Monaco, "Courier New", monospace',
        theme: {
            background: '#1e1e1e',
            foreground: '#d4d4d4',
            cursor: '#d4d4d4',
            cursorAccent: '#1e1e1e',
            selection: 'rgba(255, 255, 255, 0.3)',
            black: '#000000',
            red: '#cd3131',
            green: '#0dbc79',
            yellow: '#e5e510',
            blue: '#2472c8',
            magenta: '#bc3fbc',
            cyan: '#11a8cd',
            white: '#e5e5e5',
            brightBlack: '#666666',
            brightRed: '#f14c4c',
            brightGreen: '#23d18b',
            brightYellow: '#f5f543',
            brightBlue: '#3b8eea',
            brightMagenta: '#d670d6',
            brightCyan: '#29b8db',
            brightWhite: '#ffffff'
        }
    });
    
    // 创建 fit 插件
    fitAddon = new FitAddon.FitAddon();
    term.loadAddon(fitAddon);
    
    // 打开终端
    term.open(container);
    fitAddon.fit();
    
    // 连接 WebSocket
    connectTerminalWS(containerId);
    
    // 监听窗口大小变化
    window.addEventListener('resize', handleTerminalResize);
}

// 连接 WebSocket
function connectTerminalWS(containerId) {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/containers/terminal/ws?id=${containerId}`;
    
    term.writeln('\x1b[33mConnecting to container...\x1b[0m');
    
    termSocket = new WebSocket(wsUrl);
    
    termSocket.onopen = () => {
        term.writeln('\x1b[32mConnected!\x1b[0m\r\n');
        // 发送初始终端大小
        sendTerminalSize();
    };
    
    termSocket.onmessage = (event) => {
        if (event.data instanceof Blob) {
            event.data.text().then(text => term.write(text));
        } else {
            term.write(event.data);
        }
    };
    
    termSocket.onclose = () => {
        term.writeln('\r\n\x1b[31mConnection closed.\x1b[0m');
    };
    
    termSocket.onerror = (error) => {
        term.writeln('\r\n\x1b[31mConnection error.\x1b[0m');
        console.error('WebSocket error:', error);
    };
    
    // 终端输入发送到 WebSocket
    term.onData(data => {
        if (termSocket && termSocket.readyState === WebSocket.OPEN) {
            termSocket.send(data);
        }
    });
}

// 发送终端大小
function sendTerminalSize() {
    if (termSocket && termSocket.readyState === WebSocket.OPEN && term) {
        termSocket.send(JSON.stringify({
            type: 'resize',
            cols: term.cols,
            rows: term.rows
        }));
    }
}

// 处理终端大小变化
function handleTerminalResize() {
    if (fitAddon && term) {
        fitAddon.fit();
        sendTerminalSize();
    }
}

// 关闭终端模态框
function closeTerminalModal() {
    // 关闭 WebSocket
    if (termSocket) {
        termSocket.close();
        termSocket = null;
    }
    
    // 销毁终端
    if (term) {
        term.dispose();
        term = null;
    }
    
    // 移除事件监听
    window.removeEventListener('resize', handleTerminalResize);
    
    document.getElementById('terminal-modal').classList.remove('active');
    currentTerminalContainer = null;
}

// ========== 文件管理功能 ==========

// 打开文件管理模态框
function openFilesModal(containerId, containerName) {
    currentFileContainer = containerId;
    currentFilePath = '/';
    
    const modal = document.getElementById('files-modal');
    document.getElementById('files-container-name').textContent = containerName;
    modal.classList.add('active');
    
    loadFilesList();
}

// 关闭文件管理模态框
function closeFilesModal() {
    document.getElementById('files-modal').classList.remove('active');
    currentFileContainer = null;
}

// 加载文件列表
async function loadFilesList(path) {
    if (path !== undefined) {
        currentFilePath = path;
    }
    
    document.getElementById('current-path').textContent = currentFilePath;
    const tbody = document.getElementById('files-tbody');
    tbody.innerHTML = '<tr><td colspan="5" class="px-4 py-8 text-center text-gray-500">' + t('common.loading') + '</td></tr>';
    
    try {
        const response = await authFetch('/api/containers/files?id=' + currentFileContainer + '&path=' + encodeURIComponent(currentFilePath));
        
        if (!response.ok) {
            throw new Error(await response.text());
        }
        
        const files = await response.json();
        renderFilesList(files);
    } catch (error) {
        tbody.innerHTML = '<tr><td colspan="5" class="px-4 py-8 text-center text-red-500">' + error.message + '</td></tr>';
    }
}

// 渲染文件列表
function renderFilesList(files) {
    const tbody = document.getElementById('files-tbody');
    
    if (!files || files.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="px-4 py-8 text-center text-gray-500">' + t('files.empty') + '</td></tr>';
        return;
    }
    
    // 排序：目录在前
    files.sort((a, b) => {
        if (a.is_dir && !b.is_dir) return -1;
        if (!a.is_dir && b.is_dir) return 1;
        return a.name.localeCompare(b.name);
    });
    
    let html = '';
    for (const file of files) {
        const icon = file.is_dir ? '📁' : '📄';
        const size = file.is_dir ? '-' : formatFileSize(file.size);
        
        html += '<tr class="hover:bg-gray-50 dark:hover:bg-dark-border">';
        html += '<td class="px-4 py-2">';
        if (file.is_dir) {
            html += '<a href="#" onclick="loadFilesList(\'' + file.path + '\')" class="text-blue-500 hover:underline">' + icon + ' ' + escapeHtml(file.name) + '</a>';
        } else {
            html += '<span>' + icon + ' ' + escapeHtml(file.name) + '</span>';
        }
        html += '</td>';
        html += '<td class="px-4 py-2 text-sm text-gray-500">' + size + '</td>';
        html += '<td class="px-4 py-2 text-sm text-gray-500">' + file.mode + '</td>';
        html += '<td class="px-4 py-2 text-sm text-gray-500">' + file.mod_time + '</td>';
        html += '<td class="px-4 py-2"><div class="flex gap-1">';
        if (!file.is_dir) {
            html += '<button onclick="downloadFile(\'' + file.path + '\')" class="text-blue-500 hover:text-blue-700 text-sm" title="' + t('files.download') + '">⬇️</button>';
            html += '<button onclick="editFile(\'' + file.path + '\')" class="text-green-500 hover:text-green-700 text-sm" title="' + t('files.edit') + '">✏️</button>';
        }
        html += '<button onclick="deleteFile(\'' + file.path + '\', ' + file.is_dir + ')" class="text-red-500 hover:text-red-700 text-sm" title="' + t('common.delete') + '">🗑️</button>';
        html += '</div></td></tr>';
    }
    
    tbody.innerHTML = html;
}

// 返回上级目录
function goParentDir() {
    if (currentFilePath === '/') return;
    const parts = currentFilePath.split('/').filter(p => p);
    parts.pop();
    loadFilesList('/' + parts.join('/'));
}

// 创建目录
async function createDirectory() {
    const name = prompt(t('files.enterDirName'));
    if (!name) return;
    
    try {
        const response = await authFetch('/api/containers/files/mkdir', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                container_id: currentFileContainer,
                path: currentFilePath + '/' + name
            })
        });
        
        if (!response.ok) {
            throw new Error(await response.text());
        }
        
        showToast(t('files.createSuccess'), 'success');
        loadFilesList();
    } catch (error) {
        showToast(t('files.createFailed') + ': ' + error.message, 'error');
    }
}

// 上传文件
function triggerUpload() {
    document.getElementById('file-upload-input').click();
}

async function handleFileUpload(input) {
    const file = input.files[0];
    if (!file) return;
    
    // 限制文件大小 10MB
    if (file.size > 10 * 1024 * 1024) {
        showToast(t('files.sizeLimit'), 'error');
        return;
    }
    
    try {
        const content = await readFileAsBase64(file);
        
        const response = await authFetch('/api/containers/files/upload', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                container_id: currentFileContainer,
                path: currentFilePath,
                filename: file.name,
                content: content
            })
        });
        
        if (!response.ok) {
            throw new Error(await response.text());
        }
        
        showToast(t('files.uploadSuccess'), 'success');
        loadFilesList();
    } catch (error) {
        showToast(t('files.uploadFailed') + ': ' + error.message, 'error');
    }
    
    input.value = '';
}

// 读取文件为 Base64
function readFileAsBase64(file) {
    return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => {
            const base64 = reader.result.split(',')[1];
            resolve(base64);
        };
        reader.onerror = reject;
        reader.readAsDataURL(file);
    });
}

// 下载文件
function downloadFile(path) {
    const url = '/api/containers/files/download?id=' + currentFileContainer + '&path=' + encodeURIComponent(path);
    
    // 创建带认证的下载链接
    authFetch(url).then(response => {
        if (!response.ok) throw new Error(t('files.downloadFailed'));
        return response.blob();
    }).then(blob => {
        const a = document.createElement('a');
        a.href = URL.createObjectURL(blob);
        a.download = path.split('/').pop();
        a.click();
        URL.revokeObjectURL(a.href);
    }).catch(error => {
        showToast(error.message, 'error');
    });
}

// 编辑文件
async function editFile(path) {
    try {
        const response = await authFetch('/api/containers/files/read?id=' + currentFileContainer + '&path=' + encodeURIComponent(path));
        
        if (!response.ok) {
            throw new Error(await response.text());
        }
        
        const data = await response.json();
        
        document.getElementById('edit-file-path').value = path;
        document.getElementById('edit-file-content').value = data.content;
        document.getElementById('file-edit-modal').classList.add('active');
    } catch (error) {
        showToast(t('files.readFailed') + ': ' + error.message, 'error');
    }
}

// 保存文件
async function saveEditedFile() {
    const path = document.getElementById('edit-file-path').value;
    const content = document.getElementById('edit-file-content').value;
    
    try {
        const response = await authFetch('/api/containers/files/write', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                container_id: currentFileContainer,
                path: path,
                content: content
            })
        });
        
        if (!response.ok) {
            throw new Error(await response.text());
        }
        
        showToast(t('common.saveSuccess'), 'success');
        document.getElementById('file-edit-modal').classList.remove('active');
        loadFilesList();
    } catch (error) {
        showToast(t('common.saveFailed') + ': ' + error.message, 'error');
    }
}

// 删除文件
async function deleteFile(path, isDir) {
    const type = isDir ? t('files.directory') : t('files.file');
    if (!confirm(t('files.confirmDelete') + ' ' + type + '?\n' + path)) return;
    
    try {
        const response = await authFetch('/api/containers/files/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                container_id: currentFileContainer,
                path: path
            })
        });
        
        if (!response.ok) {
            throw new Error(await response.text());
        }
        
        showToast(t('common.deleteSuccess'), 'success');
        loadFilesList();
    } catch (error) {
        showToast(t('common.deleteFailed') + ': ' + error.message, 'error');
    }
}

// 格式化文件大小
function formatFileSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// ========== 容器配置修改 ==========

let currentContainerConfig = null;

// 初始化配置标签页切换
function initConfigTabs() {
    document.querySelectorAll('.config-tab-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            const tab = this.dataset.configTab;
            
            // 更新按钮样式
            document.querySelectorAll('.config-tab-btn').forEach(b => {
                b.classList.remove('border-blue-500', 'text-blue-600');
                b.classList.add('border-transparent', 'text-gray-500');
            });
            this.classList.remove('border-transparent', 'text-gray-500');
            this.classList.add('border-blue-500', 'text-blue-600');
            
            // 切换内容
            document.querySelectorAll('.config-tab-content').forEach(content => {
                content.classList.add('hidden');
            });
            document.getElementById('config-tab-' + tab).classList.remove('hidden');
        });
    });
}


// 打开容器配置模态框
async function openContainerConfigModal(containerId) {
    try {
        const response = await authFetch('/api/containers/inspect?id=' + containerId);
        if (!response.ok) throw new Error(await response.text());
        
        const config = await response.json();
        currentContainerConfig = config;
        
        // 基本信息
        document.getElementById('config-container-id').value = containerId;
        document.getElementById('config-full-id').textContent = config.fullId || config.id;
        document.getElementById('config-container-name').value = config.name;
        document.getElementById('config-image').textContent = config.image;
        document.getElementById('config-state').innerHTML = config.running 
            ? '<span class="text-green-500">● ' + t('containers.running') + '</span>' 
            : '<span class="text-red-500">● ' + t('containers.stopped') + '</span>';
        document.getElementById('config-created').textContent = formatDateTime(config.created);
        document.getElementById('config-started').textContent = config.started ? formatDateTime(config.started) : '-';
        document.getElementById('config-pid').textContent = config.pid || '-';
        document.getElementById('config-restart').value = config.restart || 'no';
        document.getElementById('config-cmd').value = config.cmd ? config.cmd.join(' ') : '-';
        document.getElementById('config-entrypoint').value = config.entrypoint ? config.entrypoint.join(' ') : '-';
        document.getElementById('config-user').value = config.user || 'root';
        document.getElementById('config-workdir').value = config.workingDir || '/';
        
        // 网络配置
        document.getElementById('config-network-mode').textContent = config.networkMode || 'bridge';
        document.getElementById('config-ip').textContent = config.ipAddress || '-';
        document.getElementById('config-gateway').textContent = config.gateway || '-';
        document.getElementById('config-mac').textContent = config.macAddress || '-';
        document.getElementById('config-hostname').textContent = config.hostname || '-';
        document.getElementById('config-domain').textContent = config.domainname || '-';
        document.getElementById('config-dns').textContent = config.dns && config.dns.length ? config.dns.join(', ') : '-';
        document.getElementById('config-extra-hosts').textContent = config.extraHosts && config.extraHosts.length ? config.extraHosts.join('\n') : '-';
        
        // 端口映射
        const portsList = document.getElementById('config-ports-list');
        if (config.ports && config.ports.length > 0) {
            portsList.innerHTML = config.ports.map(p => 
                '<div class="flex items-center gap-2 p-2 bg-gray-50 dark:bg-dark-border rounded text-sm">' +
                '<span class="font-mono">' + (p.hostIP || '0.0.0.0') + ':' + p.host + '</span>' +
                '<span class="text-gray-400">→</span>' +
                '<span class="font-mono">' + p.container + '</span>' +
                '</div>'
            ).join('');
        } else {
            portsList.innerHTML = '<div class="text-sm text-gray-500 p-2">' + t('config.noPorts') + '</div>';
        }
        
        // 存储配置
        const volumesList = document.getElementById('config-volumes-list');
        const volumesEmpty = document.getElementById('config-volumes-empty');
        if (config.volumes && config.volumes.length > 0) {
            volumesList.innerHTML = config.volumes.map(v => 
                '<div class="flex items-center gap-2 p-2 bg-gray-50 dark:bg-dark-border rounded text-sm">' +
                '<span class="font-mono flex-1 truncate" title="' + v.host + '">' + v.host + '</span>' +
                '<span class="text-gray-400">→</span>' +
                '<span class="font-mono flex-1 truncate" title="' + v.container + '">' + v.container + '</span>' +
                '<span class="text-xs text-gray-400">' + v.mode + '</span>' +
                '</div>'
            ).join('');
            volumesList.classList.remove('hidden');
            volumesEmpty.classList.add('hidden');
        } else {
            volumesList.classList.add('hidden');
            volumesEmpty.classList.remove('hidden');
        }
        document.getElementById('config-readonly').textContent = config.readOnly ? '是' : '否';
        
        // 环境变量
        const envList = document.getElementById('config-env-list');
        const envEmpty = document.getElementById('config-env-empty');
        if (config.env && config.env.length > 0) {
            envList.innerHTML = config.env.map(e => 
                '<div class="flex gap-2 p-2 bg-gray-50 dark:bg-dark-border rounded text-sm font-mono">' +
                '<span class="text-blue-600 dark:text-blue-400">' + escapeHtml(e.key) + '</span>' +
                '<span class="text-gray-400">=</span>' +
                '<span class="flex-1 truncate dark:text-dark-text" title="' + escapeHtml(e.value) + '">' + escapeHtml(e.value) + '</span>' +
                '</div>'
            ).join('');
            envList.classList.remove('hidden');
            envEmpty.classList.add('hidden');
        } else {
            envList.classList.add('hidden');
            envEmpty.classList.remove('hidden');
        }
        
        // 资源限制
        const memoryMB = config.memory ? Math.round(config.memory / 1024 / 1024) : 0;
        const cpuCores = config.cpus ? (config.cpus / 1e9) : 0;
        document.getElementById('config-memory').value = memoryMB || '';
        document.getElementById('config-memory-current').textContent = memoryMB ? memoryMB + ' MB' : t('config.unlimited');
        document.getElementById('config-memory-swap').value = config.memorySwap ? Math.round(config.memorySwap / 1024 / 1024) : '';
        document.getElementById('config-cpus').value = cpuCores ? cpuCores.toFixed(2) : '';
        document.getElementById('config-cpus-current').textContent = cpuCores ? cpuCores.toFixed(2) + ' ' + t('config.cores') : t('config.unlimited');
        document.getElementById('config-cpu-shares').value = config.cpuShares || '';
        document.getElementById('config-cpuset').value = config.cpusetCpus || '';
        document.getElementById('config-pids').value = config.pidsLimit || '';
        
        // 高级配置
        document.getElementById('config-privileged').textContent = config.privileged ? '是 ⚠️' : '否';
        document.getElementById('config-tty').textContent = config.tty ? '是' : '否';
        document.getElementById('config-oom').textContent = config.oomKillDisable ? t('config.disabled') : t('config.enabled');
        document.getElementById('config-log-driver').textContent = config.logDriver || 'json-file';
        document.getElementById('config-cap-add').textContent = config.capAdd && config.capAdd.length ? config.capAdd.join(', ') : '-';
        document.getElementById('config-cap-drop').textContent = config.capDrop && config.capDrop.length ? config.capDrop.join(', ') : '-';
        
        // 标签
        const labelsList = document.getElementById('config-labels-list');
        if (config.labels && Object.keys(config.labels).length > 0) {
            labelsList.innerHTML = Object.entries(config.labels).map(([k, v]) => 
                '<div class="flex gap-2 p-2 bg-gray-50 dark:bg-dark-border rounded text-xs font-mono">' +
                '<span class="text-purple-600 dark:text-purple-400">' + escapeHtml(k) + '</span>' +
                '<span class="text-gray-400">=</span>' +
                '<span class="flex-1 truncate dark:text-dark-text" title="' + escapeHtml(v) + '">' + escapeHtml(v) + '</span>' +
                '</div>'
            ).join('');
        } else {
            labelsList.innerHTML = '<div class="text-sm text-gray-500">' + t('config.noLabels') + '</div>';
        }
        
        // 初始化标签页
        initConfigTabs();
        // 重置到第一个标签页
        document.querySelector('.config-tab-btn').click();
        
        document.getElementById('container-config-modal').classList.add('active');
    } catch (error) {
        showToast(t('config.getFailed') + ': ' + error.message, 'error');
    }
}

// 格式化日期时间
function formatDateTime(dateStr) {
    if (!dateStr) return '-';
    try {
        const date = new Date(dateStr);
        return date.toLocaleString();
    } catch (e) {
        return dateStr;
    }
}

// 关闭容器配置模态框
function closeContainerConfigModal() {
    document.getElementById('container-config-modal').classList.remove('active');
    currentContainerConfig = null;
}

// 保存容器配置
async function saveContainerConfig() {
    const containerId = document.getElementById('config-container-id').value;
    const restart = document.getElementById('config-restart').value;
    const memoryMB = document.getElementById('config-memory').value;
    const cpus = document.getElementById('config-cpus').value;
    
    try {
        const updateResponse = await authFetch('/api/containers/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                container_id: containerId,
                restart: restart,
                memory: memoryMB ? parseInt(memoryMB) * 1024 * 1024 : 0,
                cpus: cpus ? parseFloat(cpus) * 1e9 : 0
            })
        });
        
        if (!updateResponse.ok) {
            throw new Error(await updateResponse.text());
        }
        
        showToast(t('config.updateSuccess'), 'success');
        closeContainerConfigModal();
        loadContainers();
    } catch (error) {
        showToast(t('config.updateFailed') + ': ' + error.message, 'error');
    }
}

// 重命名容器
async function renameContainer() {
    const containerId = document.getElementById('config-container-id').value;
    const newName = document.getElementById('config-container-name').value;
    
    if (!newName) {
        showToast(t('config.nameRequired'), 'error');
        return;
    }
    
    try {
        const response = await authFetch('/api/containers/rename', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                container_id: containerId,
                new_name: newName
            })
        });
        
        if (!response.ok) {
            throw new Error(await response.text());
        }
        
        showToast(t('config.renameSuccess'), 'success');
        closeContainerConfigModal();
        loadContainers();
    } catch (error) {
        showToast(t('config.renameFailed') + ': ' + error.message, 'error');
    }
}

// HTML 转义
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}


// ========== 重建容器功能 ==========

// 打开重建容器模态框
function openRecreateContainerModal() {
    if (!currentContainerConfig) {
        showToast(t('config.noConfig'), 'error');
        return;
    }
    
    const config = currentContainerConfig;
    
    // 填充基本信息
    document.getElementById('recreate-container-id').value = config.fullId || config.id;
    document.getElementById('recreate-name').value = config.name;
    
    // 分离镜像地址和标签
    const imageParts = (config.image || '').split(':');
    const imageAddr = imageParts[0] || '';
    const imageTag = imageParts.slice(1).join(':') || 'latest';
    document.getElementById('recreate-image').value = imageAddr;
    document.getElementById('recreate-image-tag').value = imageTag;
    
    document.getElementById('recreate-restart').value = config.restart || 'no';
    document.getElementById('recreate-network').value = config.networkMode || 'bridge';
    document.getElementById('recreate-privileged').checked = config.privileged || false;
    document.getElementById('recreate-tty').checked = config.tty !== false;
    
    // 资源限制
    const memoryMB = config.memory ? Math.round(config.memory / 1024 / 1024) : '';
    const cpuCores = config.cpus ? (config.cpus / 1e9).toFixed(2) : '';
    document.getElementById('recreate-memory').value = memoryMB;
    document.getElementById('recreate-cpus').value = cpuCores;
    
    // 端口映射
    const portsList = document.getElementById('recreate-ports-list');
    portsList.innerHTML = '';
    if (config.ports && config.ports.length > 0) {
        config.ports.forEach(p => {
            addRecreatePort(p.host, p.container.replace('/tcp', '').replace('/udp', ''));
        });
    }
    
    // 数据卷
    const volumesList = document.getElementById('recreate-volumes-list');
    volumesList.innerHTML = '';
    if (config.volumes && config.volumes.length > 0) {
        config.volumes.forEach(v => {
            addRecreateVolume(v.host, v.container);
        });
    }
    
    // 环境变量
    const envList = document.getElementById('recreate-env-list');
    envList.innerHTML = '';
    if (config.env && config.env.length > 0) {
        config.env.forEach(e => {
            addRecreateEnv(e.key, e.value);
        });
    }
    
    document.getElementById('recreate-container-modal').classList.add('active');
}

// 关闭重建容器模态框
function closeRecreateContainerModal() {
    document.getElementById('recreate-container-modal').classList.remove('active');
}

// 添加端口映射行
function addRecreatePort(hostPort, containerPort) {
    const list = document.getElementById('recreate-ports-list');
    const div = document.createElement('div');
    div.className = 'flex gap-2 items-center';
    div.innerHTML = 
        '<input type="text" placeholder="' + t('create.port.host') + '" value="' + (hostPort || '') + '" class="flex-1 px-2 py-1 border border-gray-300 dark:border-dark-border rounded text-sm recreate-port-host">' +
        '<span class="text-gray-400">:</span>' +
        '<input type="text" placeholder="' + t('create.port.container') + '" value="' + (containerPort || '') + '" class="flex-1 px-2 py-1 border border-gray-300 dark:border-dark-border rounded text-sm recreate-port-container">' +
        '<button onclick="this.parentElement.remove()" class="text-red-500 hover:text-red-700 p-1">✕</button>';
    list.appendChild(div);
}

// 添加数据卷行
function addRecreateVolume(hostPath, containerPath) {
    const list = document.getElementById('recreate-volumes-list');
    const div = document.createElement('div');
    div.className = 'flex gap-2 items-center';
    div.innerHTML = 
        '<input type="text" placeholder="' + t('create.vol.host') + '" value="' + (hostPath || '') + '" class="flex-1 px-2 py-1 border border-gray-300 dark:border-dark-border rounded text-sm recreate-vol-host">' +
        '<span class="text-gray-400">:</span>' +
        '<input type="text" placeholder="' + t('create.vol.container') + '" value="' + (containerPath || '') + '" class="flex-1 px-2 py-1 border border-gray-300 dark:border-dark-border rounded text-sm recreate-vol-container">' +
        '<button onclick="this.parentElement.remove()" class="text-red-500 hover:text-red-700 p-1">✕</button>';
    list.appendChild(div);
}

// 添加环境变量行
function addRecreateEnv(key, value) {
    const list = document.getElementById('recreate-env-list');
    const div = document.createElement('div');
    div.className = 'flex gap-2 items-center';
    div.innerHTML = 
        '<input type="text" placeholder="' + t('create.env.key') + '" value="' + escapeHtml(key || '') + '" class="flex-1 px-2 py-1 border border-gray-300 dark:border-dark-border rounded text-sm recreate-env-key">' +
        '<span class="text-gray-400">=</span>' +
        '<input type="text" placeholder="' + t('create.env.value') + '" value="' + escapeHtml(value || '') + '" class="flex-1 px-2 py-1 border border-gray-300 dark:border-dark-border rounded text-sm recreate-env-value">' +
        '<button onclick="this.parentElement.remove()" class="text-red-500 hover:text-red-700 p-1">✕</button>';
    list.appendChild(div);
}

// 执行重建容器
async function executeRecreateContainer() {
    const containerId = document.getElementById('recreate-container-id').value;
    const name = document.getElementById('recreate-name').value;
    const imageAddr = document.getElementById('recreate-image').value.trim();
    const imageTag = document.getElementById('recreate-image-tag').value.trim() || 'latest';
    const image = imageAddr + ':' + imageTag;
    
    if (!imageAddr) {
        showToast(t('config.imageRequired'), 'error');
        return;
    }
    
    // 收集端口映射
    const ports = [];
    document.querySelectorAll('#recreate-ports-list > div').forEach(div => {
        const host = div.querySelector('.recreate-port-host').value.trim();
        const container = div.querySelector('.recreate-port-container').value.trim();
        if (host && container) {
            ports.push({ host, container });
        }
    });
    
    // 收集数据卷
    const volumes = [];
    document.querySelectorAll('#recreate-volumes-list > div').forEach(div => {
        const host = div.querySelector('.recreate-vol-host').value.trim();
        const container = div.querySelector('.recreate-vol-container').value.trim();
        if (host && container) {
            volumes.push({ host, container });
        }
    });
    
    // 收集环境变量
    const env = [];
    document.querySelectorAll('#recreate-env-list > div').forEach(div => {
        const key = div.querySelector('.recreate-env-key').value.trim();
        const value = div.querySelector('.recreate-env-value').value;
        if (key) {
            env.push({ key, value });
        }
    });
    
    const memoryMB = document.getElementById('recreate-memory').value;
    const cpus = document.getElementById('recreate-cpus').value;
    
    // 使用自定义确认弹窗
    const confirmed = await showConfirm({
        title: t('config.recreate'),
        message: t('config.recreateConfirm') + '<br><br><span class="text-yellow-600">⚠️ ' + t('config.recreateWarning') + '</span>',
        type: 'danger',
        confirmText: t('config.confirmRecreate'),
        cancelText: t('common.cancel')
    });
    
    if (!confirmed) return;
    
    // 获取按钮并显示加载状态
    const btn = document.getElementById('recreate-confirm-btn');
    const originalText = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '<span class="inline-flex items-center"><svg class="animate-spin -ml-1 mr-2 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>' + t('config.recreating') + '</span>';
    
    // 禁用关闭按钮
    const closeBtn = document.querySelector('#recreate-container-modal .modal-close');
    if (closeBtn) closeBtn.style.pointerEvents = 'none';
    
    try {
        const response = await authFetch('/api/containers/recreate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                container_id: containerId,
                name: name,
                image: image,
                ports: ports,
                volumes: volumes,
                env: env,
                restart: document.getElementById('recreate-restart').value,
                network: document.getElementById('recreate-network').value,
                memory: memoryMB ? parseInt(memoryMB) : 0,
                cpus: cpus ? parseFloat(cpus) : 0,
                privileged: document.getElementById('recreate-privileged').checked,
                tty: document.getElementById('recreate-tty').checked
            })
        });
        
        if (!response.ok) {
            throw new Error(await response.text());
        }
        
        showToast(t('config.recreateSuccess'), 'success');
        closeRecreateContainerModal();
        closeContainerConfigModal();
        loadContainers();
    } catch (error) {
        showToast(t('config.recreateFailed') + ': ' + error.message, 'error');
    } finally {
        // 恢复按钮状态
        btn.disabled = false;
        btn.innerHTML = originalText;
        if (closeBtn) closeBtn.style.pointerEvents = '';
    }
}

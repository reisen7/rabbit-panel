/**
 * Compose 管理模块 - 重构版
 */

let currentComposeProject = '';
let composeProjects = [];

// 加载项目列表
function loadComposeProjects() {
    fetch('/api/compose/list?t=' + Date.now(), { credentials: 'include' })
        .then(res => {
            if (res.status === 401) { handleLogout(); return; }
            return res.json();
        })
        .then(data => {
            if (!data) return;
            composeProjects = data;
            renderComposeList();
            
            // 如果当前有选中的项目，刷新它的状态
            if (currentComposeProject) {
                loadComposeStatus(currentComposeProject);
            }
        })
        .catch(err => showToast(err.message, 'error', { title: '加载失败' }));
}

// 渲染项目列表（桌面端和移动端）
function renderComposeList() {
    const list = DOM.get('compose-list');
    const listMobile = DOM.get('compose-list-mobile');
    
    if (composeProjects.length === 0) {
        const emptyHtml = '<div class="text-center text-gray-400 dark:text-dark-muted text-sm py-4">暂无项目</div>';
        if (list) list.innerHTML = emptyHtml;
        if (listMobile) listMobile.innerHTML = emptyHtml;
        return;
    }
    
    // 桌面端列表
    if (list) {
        list.innerHTML = composeProjects.map(p => `
            <div class="compose-item p-2 rounded cursor-pointer transition-colors ${currentComposeProject === p.name ? 'bg-blue-100 dark:bg-blue-900' : 'hover:bg-gray-100 dark:hover:bg-dark-card'}" 
                 onclick="selectComposeProject('${p.name}')" data-project="${p.name}">
                <div class="flex items-center justify-between">
                    <span class="font-medium text-sm dark:text-dark-text truncate">${p.name}</span>
                    <span id="compose-list-status-${p.name}" class="w-2 h-2 rounded-full bg-gray-300"></span>
                </div>
            </div>
        `).join('');
    }
    
    // 移动端列表（卡片样式）
    if (listMobile) {
        listMobile.innerHTML = composeProjects.map(p => `
            <div class="bg-white dark:bg-dark-card rounded-lg p-4 shadow-sm border border-gray-100 dark:border-dark-border" onclick="selectComposeProject('${p.name}')">
                <div class="flex items-center justify-between mb-2">
                    <span class="font-semibold dark:text-dark-text">${p.name}</span>
                    <span id="compose-mobile-list-status-${p.name}" class="px-2 py-0.5 text-xs rounded bg-gray-100 dark:bg-dark-border text-gray-500">加载中</span>
                </div>
                <div id="compose-mobile-containers-${p.name}" class="text-xs text-gray-500 dark:text-dark-muted">获取状态...</div>
            </div>
        `).join('');
    }
    
    // 加载每个项目的状态
    composeProjects.forEach(p => loadComposeListStatus(p.name));
}

// 加载列表中项目的状态指示器
function loadComposeListStatus(name) {
    fetch(`/api/compose/status?project=${name}`, { credentials: 'include' })
        .then(res => res.json())
        .then(data => {
            // 桌面端状态点
            const dot = document.getElementById(`compose-list-status-${name}`);
            if (dot) {
                const colors = {
                    running: 'bg-green-500',
                    partial: 'bg-yellow-500',
                    stopped: 'bg-gray-400',
                    unknown: 'bg-gray-300'
                };
                dot.className = `w-2 h-2 rounded-full ${colors[data.status] || colors.unknown}`;
            }
            
            // 移动端状态标签
            const mobileStatus = document.getElementById(`compose-mobile-list-status-${name}`);
            if (mobileStatus) {
                const statusConfig = {
                    running: { text: '运行中', class: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300' },
                    partial: { text: '部分运行', class: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300' },
                    stopped: { text: '已停止', class: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300' },
                    unknown: { text: '未知', class: 'bg-gray-100 text-gray-500' }
                };
                const config = statusConfig[data.status] || statusConfig.unknown;
                mobileStatus.className = `px-2 py-0.5 text-xs rounded ${config.class}`;
                mobileStatus.textContent = config.text;
            }
            
            // 移动端容器简要信息
            const mobileContainers = document.getElementById(`compose-mobile-containers-${name}`);
            if (mobileContainers) {
                if (!data.containers || data.containers.length === 0) {
                    mobileContainers.textContent = '无容器';
                } else {
                    const running = data.containers.filter(c => c.state === 'running').length;
                    mobileContainers.textContent = `${running}/${data.containers.length} 个容器运行中`;
                }
            }
        })
        .catch(() => {});
}

// 选择项目
function selectComposeProject(name) {
    currentComposeProject = name;
    
    const isMobile = window.innerWidth < 768;
    
    if (isMobile) {
        // 移动端：切换到详情视图
        DOM.get('compose-mobile-list').classList.add('hidden');
        DOM.get('compose-mobile-detail').classList.remove('hidden');
        DOM.get('compose-mobile-name').textContent = name;
        
        // 加载状态和文件到移动端元素
        loadComposeStatus(name);
        loadComposeFile(name);
    } else {
        // 桌面端：更新列表选中状态
        document.querySelectorAll('.compose-item').forEach(el => {
            el.classList.remove('bg-blue-100', 'dark:bg-blue-900');
            el.classList.add('hover:bg-gray-100', 'dark:hover:bg-dark-card');
        });
        const selected = document.querySelector(`.compose-item[data-project="${name}"]`);
        if (selected) {
            selected.classList.add('bg-blue-100', 'dark:bg-blue-900');
            selected.classList.remove('hover:bg-gray-100', 'dark:hover:bg-dark-card');
        }
        
        // 显示详情面板
        DOM.get('compose-detail-empty').classList.add('hidden');
        DOM.get('compose-detail').classList.remove('hidden');
        DOM.get('compose-detail-name').textContent = name;
        
        // 加载状态和文件
        loadComposeStatus(name);
        loadComposeFile(name);
    }
}

// 返回项目列表（移动端）
function backToComposeList() {
    currentComposeProject = '';
    DOM.get('compose-mobile-detail').classList.add('hidden');
    DOM.get('compose-mobile-list').classList.remove('hidden');
}

// 加载项目状态
function loadComposeStatus(name) {
    const isMobile = window.innerWidth < 768;
    const containersList = DOM.get(isMobile ? 'compose-containers-list-mobile' : 'compose-containers-list');
    const statusBadge = DOM.get(isMobile ? 'compose-mobile-status' : 'compose-detail-status');
    
    if (containersList) containersList.innerHTML = '<div class="text-gray-400 text-xs">加载中...</div>';
    
    fetch(`/api/compose/status?project=${name}`, { credentials: 'include' })
        .then(res => res.json())
        .then(data => {
            // 更新状态徽章
            const statusConfig = {
                running: { text: '运行中', class: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300' },
                partial: { text: '部分运行', class: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300' },
                stopped: { text: '已停止', class: 'bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-300' },
                unknown: { text: '未知', class: 'bg-gray-200 text-gray-500' }
            };
            const config = statusConfig[data.status] || statusConfig.unknown;
            if (statusBadge) {
                statusBadge.className = `px-2 py-0.5 text-xs rounded ${config.class}`;
                statusBadge.textContent = config.text;
            }
            
            // 更新容器列表
            if (!containersList) return;
            
            if (!data.containers || data.containers.length === 0) {
                containersList.innerHTML = '<div class="text-gray-400 dark:text-dark-muted text-xs py-2">无容器</div>';
                return;
            }
            
            containersList.innerHTML = data.containers.map(c => {
                const isRunning = c.state === 'running';
                return `
                    <div class="flex items-center justify-between py-2 px-2 rounded ${isRunning ? 'bg-green-50 dark:bg-green-900/20' : 'bg-gray-100 dark:bg-dark-card'}">
                        <div class="flex items-center gap-2 min-w-0">
                            <span class="${isRunning ? 'text-green-500' : 'text-gray-400'}">${isRunning ? '●' : '○'}</span>
                            <span class="text-sm dark:text-dark-text truncate">${c.service || c.name}</span>
                        </div>
                        <div class="flex items-center gap-2 flex-shrink-0">
                            <span class="text-xs ${isRunning ? 'text-green-600 dark:text-green-400' : 'text-gray-500'} hidden sm:inline">${c.status}</span>
                            <button onclick="viewLogs('${c.name}', '${c.service || c.name}')" class="text-xs text-purple-500 hover:text-purple-700 px-1">日志</button>
                        </div>
                    </div>
                `;
            }).join('');
        })
        .catch(() => {
            if (containersList) containersList.innerHTML = '<div class="text-red-400 text-xs">获取失败</div>';
        });
}

// 加载 compose 文件
function loadComposeFile(name) {
    fetch(`/api/compose/file?project=${name}`, { credentials: 'include' })
        .then(res => res.text())
        .then(text => {
            const isMobile = window.innerWidth < 768;
            const editor = DOM.get(isMobile ? 'compose-mobile-editor' : 'compose-detail-editor');
            if (editor) editor.value = text;
        });
}

// 刷新当前项目状态
function refreshCurrentComposeStatus() {
    if (currentComposeProject) {
        loadComposeStatus(currentComposeProject);
        loadComposeListStatus(currentComposeProject);
    }
}

// 上传 docker-compose.yml 文件
function handleComposeFileUpload(input, type) {
    const file = input.files[0];
    if (!file) return;

    // 限制文件大小 1MB
    if (file.size > 1024 * 1024) {
        showToast(t('compose.fileTooLarge'), 'error');
        input.value = '';
        return;
    }

    const reader = new FileReader();
    reader.onload = function(e) {
        const content = e.target.result;
        const editorId = type === 'mobile' ? 'compose-mobile-editor' : 'compose-detail-editor';
        const editor = document.getElementById(editorId);
        if (editor) {
            editor.value = content;
            showToast(t('compose.fileLoaded'), 'success');
        }
    };
    reader.onerror = function() {
        showToast(t('compose.fileReadError'), 'error');
    };
    reader.readAsText(file);
    
    // 清空 input，允许重复上传同一文件
    input.value = '';
}

// 保存当前项目文件
function saveCurrentComposeFile() {
    if (!currentComposeProject) return;
    const isMobile = window.innerWidth < 768;
    const editor = DOM.get(isMobile ? 'compose-mobile-editor' : 'compose-detail-editor');
    if (!editor) return;
    
    fetch('/api/compose/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ project: currentComposeProject, content: editor.value })
    })
    .then(async res => {
        if (res.ok) {
            showToast('文件已保存', 'success');
        } else {
            showToast(await res.text(), 'error', { title: '保存失败' });
        }
    });
}

// 执行 Compose 操作
async function composeAction(action) {
    if (!currentComposeProject) return;
    
    const actionMap = { up: '启动', down: '停止', restart: '重启', pull: '拉取镜像' };
    
    // 停止操作需要确认
    if (action === 'down') {
        const confirmed = await showConfirm({
            title: '停止项目',
            message: `确定要停止 <strong>${currentComposeProject}</strong> 的所有容器吗？`,
            type: 'warning',
            confirmText: '确认停止'
        });
        if (!confirmed) return;
    }
    
    const isMobile = window.innerWidth < 768;
    const outputDiv = DOM.get(isMobile ? 'compose-mobile-output' : 'compose-detail-output');
    if (outputDiv) {
        outputDiv.classList.remove('hidden');
        outputDiv.textContent = `正在执行 ${actionMap[action]}...\n`;
    }
    
    try {
        const res = await fetch('/api/compose/action', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ project: currentComposeProject, action })
        });
        
        const text = await res.text();
        if (outputDiv) {
            outputDiv.textContent += text;
            outputDiv.scrollTop = outputDiv.scrollHeight;
        }
        
        if (res.ok) {
            showToast(`${actionMap[action]}成功`, 'success');
            // 延迟刷新状态
            setTimeout(() => {
                refreshCurrentComposeStatus();
            }, 1500);
        } else {
            showToast(`${actionMap[action]}失败`, 'error');
        }
    } catch (err) {
        if (outputDiv) outputDiv.textContent += `\n错误: ${err.message}`;
        showToast(err.message, 'error');
    }
}

// 删除项目
async function deleteComposeProject() {
    if (!currentComposeProject) return;
    
    const confirmed = await showConfirm({
        title: '删除项目',
        message: `确定要删除项目 <strong>${currentComposeProject}</strong> 吗？<br><span class="text-red-500 text-xs">这将删除项目目录和所有配置文件！</span>`,
        type: 'danger',
        confirmText: '确认删除'
    });
    if (!confirmed) return;
    
    try {
        const res = await fetch('/api/compose/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ project: currentComposeProject })
        });
        
        if (res.ok) {
            showToast('项目已删除', 'success');
            currentComposeProject = '';
            DOM.get('compose-detail').classList.add('hidden');
            DOM.get('compose-detail-empty').classList.remove('hidden');
            loadComposeProjects();
        } else {
            showToast(await res.text(), 'error', { title: '删除失败' });
        }
    } catch (err) {
        showToast(err.message, 'error');
    }
}

// 新建项目
function openCreateComposeModal() {
    DOM.get('new-compose-name').value = '';
    DOM.get('create-compose-modal').classList.add('active');
}

function createComposeProject() {
    const name = DOM.get('new-compose-name').value.trim();
    if (!name) {
        showToast('请输入项目名称', 'warning');
        return;
    }
    
    // 验证名称格式
    if (!/^[a-zA-Z][a-zA-Z0-9_-]*$/.test(name)) {
        showToast('项目名称只能包含字母、数字、下划线和横线，且必须以字母开头', 'warning');
        return;
    }

    fetch('/api/compose/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ name })
    })
    .then(async res => {
        if (res.ok) {
            showToast(`项目 ${name} 创建成功`, 'success');
            DOM.get('create-compose-modal').classList.remove('active');
            loadComposeProjects();
            // 自动选中新项目
            setTimeout(() => selectComposeProject(name), 300);
        } else {
            showToast(await res.text(), 'error', { title: '创建失败' });
        }
    });
}

// 刷新
async function refreshCompose() {
    const icon = DOM.get('refresh-compose-icon');
    const iconMobile = DOM.get('refresh-compose-icon-mobile');
    if (icon) icon.classList.add('refresh-spinning');
    if (iconMobile) iconMobile.classList.add('refresh-spinning');
    await loadComposeProjects();
    setTimeout(() => {
        if (icon) icon.classList.remove('refresh-spinning');
        if (iconMobile) iconMobile.classList.remove('refresh-spinning');
    }, 300);
}

// 编辑器快捷键
document.addEventListener('DOMContentLoaded', () => {
    const editor = DOM.get('compose-detail-editor');
    if (editor) {
        editor.addEventListener('keydown', function(e) {
            // Tab 插入空格
            if (e.key === 'Tab') {
                e.preventDefault();
                const start = this.selectionStart;
                const end = this.selectionEnd;
                this.value = this.value.substring(0, start) + '  ' + this.value.substring(end);
                this.selectionStart = this.selectionEnd = start + 2;
            }
            // Ctrl+S 保存
            if ((e.ctrlKey || e.metaKey) && e.key === 's') {
                e.preventDefault();
                saveCurrentComposeFile();
            }
        });
    }
});
